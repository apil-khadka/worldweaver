package network

import (
	"log"
	"sync"
	"time"

	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// Hub manages all connected clients and the bridge between the network
// layer and the simulation engine.
//
// Design invariants:
//   - Hub never blocks the simulation loop
//   - Slow or disconnecting clients are isolated via per-client queues
//   - Player actions are enqueued on the engine, not applied inline
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}

	world       *world.World
	engine      *simulation.Engine
	metrics     *metrics.Metrics
	connLimiter *ConnRateLimiter
	Scoreboard  *game.Scoreboard
	WorldName   string
	auth        *game.AuthManager
	worldMgr    *game.WorldManager

	// Social: per-player chat rate limiting (max 5 msgs per 10s)
	chatLimits   map[uint32]*chatRateEntry
	chatLimitsMu sync.Mutex

	// Power combo detection
	recentPowers   []powerEvent
	recentPowersMu sync.Mutex
}

// chatRateEntry tracks chat messages timestamps per player for rate limiting.
type chatRateEntry struct {
	timestamps []time.Time
}

// powerEvent records a recent power application for combo detection.
type powerEvent struct {
	playerID uint32
	power    uint8
	x, y     int
	at       time.Time
}

// NewHub creates a Hub wired to the given world and engine.
func NewHub(w *world.World, eng *simulation.Engine, m *metrics.Metrics, sb *game.Scoreboard, worldName string, auth *game.AuthManager, worldMgr *game.WorldManager) *Hub {
	return &Hub{
		clients:      make(map[*Client]struct{}),
		world:        w,
		engine:       eng,
		metrics:      m,
		connLimiter:  NewConnRateLimiter(),
		Scoreboard:   sb,
		WorldName:    worldName,
		auth:         auth,
		worldMgr:     worldMgr,
		chatLimits:   make(map[uint32]*chatRateEntry),
		recentPowers: make([]powerEvent, 0, 64),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.metrics.PlayerCount.Add(1)
	h.Scoreboard.PlayerConnected(h.WorldName, c.Player.ID)
	log.Printf("hub: player %d registered (total %d)", c.Player.ID, h.metrics.PlayerCount.Load())
	h.BroadcastPlayerJoin(c.Player.ID)
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	h.metrics.PlayerCount.Add(-1)
	h.Scoreboard.PlayerDisconnected(h.WorldName, c.Player.ID)
	log.Printf("hub: player %d unregistered (total %d)", c.Player.ID, h.metrics.PlayerCount.Load())
	h.BroadcastPlayerLeave(c.Player.ID)
}

// handlePowerInput validates an incoming power request and enqueues it on
// the simulation engine.  Validation happens on the network goroutine to keep
// the simulation loop clean.
func (h *Hub) handlePowerInput(c *Client, msg *PowerInputMsg) {
	req := &game.PowerRequest{
		PlayerID:  c.Player.ID,
		Tool:      msg.Tool,
		Power:     msg.Power,
		Material:  msg.Material,
		X:         msg.X,
		Y:         msg.Y,
		Radius:    msg.Radius,
		Intensity: msg.Intensity,
	}

	if err := req.Validate(c.Player, h.world.Width, h.world.Height); err != nil {
		c.sendError(err.Error())
		return
	}

	h.engine.EnqueueAction(simulation.PlayerAction{
		PlayerID:  req.PlayerID,
		Tool:      req.Tool,
		Power:     simulation.PowerType(req.Power),
		Material:  req.Material,
		X:         req.X,
		Y:         req.Y,
		Radius:    req.Radius,
		Intensity: req.Intensity,
	})

	// Track scoring: estimate cells affected from radius
	cellsAffected := estimateCellsInRadius(req.Radius)
	cost := game.InfluenceCost[req.Power]
	if req.Tool != game.ToolForce {
		cost = game.ToolCostPerCell[req.Tool] * float32(cellsAffected)
	}
	h.Scoreboard.RecordPowerAction(h.WorldName, c.Player.ID, req.Power, cellsAffected, cost)

	// Update player score/level from scoreboard
	if ps := h.Scoreboard.GetPlayerScore(h.WorldName, c.Player.ID); ps != nil {
		c.Player.UpdateScore(ps.Score)
	}

	// Check for power combos with other recent applications
	h.RecordPowerForCombo(c.Player.ID, req.Power, req.X, req.Y)

	// Immediately send updated influence state so the client UI stays in sync.
	c.sendJSON(PlayerStateMsg{
		Type:           MsgPlayerState,
		PlayerID:       c.Player.ID,
		Influence:      c.Player.Influence(),
		MaxInfluence:   c.Player.MaxInfluenceCap(),
		Level:          c.Player.Level(),
		Score:          c.Player.Score(),
		NextLevelScore: c.Player.NextLevelScore(),
	})
}

// estimateCellsInRadius approximates the number of cells in a circular radius.
func estimateCellsInRadius(radius int) int {
	// Rough approximation: count cells in the circle
	count := 0
	r2 := radius * radius
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= r2 {
				count++
			}
		}
	}
	return count
}

// PlayerCount returns the current number of connected clients.
func (h *Hub) PlayerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// handleCursorMove broadcasts a player's cursor position to all other clients.
func (h *Hub) handleCursorMove(sender *Client, msg *CursorMsg) {
	// The nickname travels with the cursor so other clients can label it without
	// having to maintain a separate roster keyed by player ID.
	update := mustMarshal(CursorUpdateMsg{
		Type:     MsgCursorUpdate,
		PlayerID: sender.Player.ID,
		Nickname: sender.Player.Nickname,
		X:        msg.X,
		Y:        msg.Y,
		Power:    msg.Power,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c != sender {
			c.sendRaw(update)
		}
	}
}

// BroadcastPlayerJoin notifies all connected clients that a new player joined.
func (h *Hub) BroadcastPlayerJoin(playerID uint32) {
	msg := mustMarshal(PlayerJoinMsg{
		Type:     MsgPlayerJoin,
		PlayerID: playerID,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.sendRaw(msg)
	}
}

// BroadcastPlayerLeave notifies all connected clients that a player left.
func (h *Hub) BroadcastPlayerLeave(playerID uint32) {
	msg := mustMarshal(PlayerLeaveMsg{
		Type:     MsgPlayerLeave,
		PlayerID: playerID,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.sendRaw(msg)
	}
}

// RegenerateAllInfluence ticks influence regen for all connected players
// and sends updated state to each client.
func (h *Hub) RegenerateAllInfluence() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.Player.RegenerateInfluence()
		c.sendJSON(PlayerStateMsg{
			Type:           MsgPlayerState,
			PlayerID:       c.Player.ID,
			Influence:      c.Player.Influence(),
			MaxInfluence:   c.Player.MaxInfluenceCap(),
			Level:          c.Player.Level(),
			Score:          c.Player.Score(),
			NextLevelScore: c.Player.NextLevelScore(),
		})
	}
}

// ---- Social feature handlers ----

// handleChat broadcasts a chat message to all clients in the same world.
// Rate limited to 5 messages per 10 seconds per player.
func (h *Hub) handleChat(c *Client, msg *ChatInboundMsg) {
	// Validate text length
	text := msg.Text
	if len(text) == 0 || len(text) > 50 {
		c.sendError("chat message must be 1-50 characters")
		return
	}

	// Rate limit: max 5 messages per 10 seconds
	h.chatLimitsMu.Lock()
	entry, ok := h.chatLimits[c.Player.ID]
	if !ok {
		entry = &chatRateEntry{}
		h.chatLimits[c.Player.ID] = entry
	}
	now := time.Now()
	cutoff := now.Add(-10 * time.Second)
	valid := entry.timestamps[:0]
	for _, t := range entry.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid
	if len(entry.timestamps) >= 5 {
		h.chatLimitsMu.Unlock()
		c.sendError("chat rate limited: max 5 messages per 10 seconds")
		return
	}
	entry.timestamps = append(entry.timestamps, now)
	h.chatLimitsMu.Unlock()

	// Get player cursor position from their camera center
	cx, cy := c.Player.CursorPos()

	broadcast := mustMarshal(ChatBroadcastMsg{
		Type:     MsgChatBroadcast,
		PlayerID: c.Player.ID,
		Nickname: c.Player.Nickname,
		Text:     text,
		X:        cx,
		Y:        cy,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		client.sendRaw(broadcast)
	}
}

// handlePingLocation broadcasts a location ping to all clients.
func (h *Hub) handlePingLocation(c *Client, msg *PingLocationInboundMsg) {
	// Validate coordinates are within world bounds
	if msg.X < 0 || msg.X >= h.world.Width || msg.Y < 0 || msg.Y >= h.world.Height {
		c.sendError("ping location out of bounds")
		return
	}

	broadcast := mustMarshal(PingLocationBroadcastMsg{
		Type:     MsgPingBroadcast,
		PlayerID: c.Player.ID,
		X:        msg.X,
		Y:        msg.Y,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		client.sendRaw(broadcast)
	}
}

// handleEmote broadcasts an emote from a player.
func (h *Hub) handleEmote(c *Client, msg *EmoteInboundMsg) {
	// Validate emote is one of the allowed set
	allowed := map[string]bool{"👍": true, "🔥": true, "💧": true, "🌱": true, "⚠️": true, "❤️": true}
	if !allowed[msg.Emote] {
		c.sendError("invalid emote")
		return
	}

	cx, cy := c.Player.CursorPos()

	broadcast := mustMarshal(EmoteBroadcastMsg{
		Type:     MsgEmoteBroadcast,
		PlayerID: c.Player.ID,
		Emote:    msg.Emote,
		X:        cx,
		Y:        cy,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		client.sendRaw(broadcast)
	}
}

// RecordPowerForCombo records a power application and checks for combos.
// A combo occurs when 2+ players apply DIFFERENT powers within 32 cells
// of each other within 500ms.
func (h *Hub) RecordPowerForCombo(playerID uint32, power uint8, x, y int) {
	now := time.Now()
	h.recentPowersMu.Lock()
	defer h.recentPowersMu.Unlock()

	// Prune events older than 500ms
	cutoff := now.Add(-500 * time.Millisecond)
	valid := h.recentPowers[:0]
	for _, ev := range h.recentPowers {
		if ev.at.After(cutoff) {
			valid = append(valid, ev)
		}
	}
	h.recentPowers = valid

	// Add this event
	newEvent := powerEvent{playerID: playerID, power: power, x: x, y: y, at: now}
	h.recentPowers = append(h.recentPowers, newEvent)

	// Check for combos: different player, different power, within 32 cells
	const comboRadius = 32
	for _, ev := range h.recentPowers {
		if ev.playerID == playerID || ev.power == power {
			continue
		}
		dx := ev.x - x
		dy := ev.y - y
		dist2 := dx*dx + dy*dy
		if dist2 <= comboRadius*comboRadius {
			// Combo detected!
			comboX := (ev.x + x) / 2
			comboY := (ev.y + y) / 2
			broadcast := mustMarshal(ComboBroadcastMsg{
				Type:      MsgCombo,
				PlayerIDs: []uint32{ev.playerID, playerID},
				Powers:    []uint8{ev.power, power},
				X:         comboX,
				Y:         comboY,
			})
			h.mu.RLock()
			for client := range h.clients {
				client.sendRaw(broadcast)
			}
			h.mu.RUnlock()
			log.Printf("hub: combo detected between players %d and %d at (%d,%d)", ev.playerID, playerID, comboX, comboY)
			return // One combo per event
		}
	}
}

// BroadcastGoalUpdate sends the current cooperative goal state to all clients.
func (h *Hub) BroadcastGoalUpdate(goalText string, progress, target int, completed bool) {
	msg := mustMarshal(GoalUpdateMsg{
		Type:      MsgGoalUpdate,
		GoalText:  goalText,
		Progress:  progress,
		Target:    target,
		Completed: completed,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.sendRaw(msg)
	}
}

// TickGoals checks goal progress against the world state and broadcasts updates.
// Called periodically from the metrics broadcast loop.
func (h *Hub) TickGoals(stability float32) {
	worldID := h.WorldName

	// Check if rotation is due
	if h.worldMgr.RotateGoalIfDue(worldID) {
		gs := h.worldMgr.GetGoalState(worldID)
		if gs != nil {
			h.BroadcastGoalUpdate(gs.Definition.Text, 0, gs.Definition.Target, false)
		}
		return
	}

	gs := h.worldMgr.GetGoalState(worldID)
	if gs == nil || gs.Completed {
		return
	}

	var progress int
	switch gs.Definition.Type {
	case game.GoalStability:
		progress = int(stability * 100)
	case game.GoalGrowPlants:
		progress = h.countMaterial(5) // MatPlant == 5
	case game.GoalExtinguishFires:
		progress = h.countMaterial(6) // MatFire == 6
	case game.GoalCreateLake:
		progress = h.largestWaterBlob()
	}

	justCompleted := h.worldMgr.UpdateGoalProgress(worldID, progress)
	h.BroadcastGoalUpdate(gs.Definition.Text, progress, gs.Definition.Target, justCompleted)

	// Award bonus to all players on completion
	if justCompleted {
		h.awardGoalBonus()
	}
}

// countMaterial counts cells of a given material type in the world.
func (h *Hub) countMaterial(mat uint8) int {
	count := 0
	for _, m := range h.world.Material {
		if m == mat {
			count++
		}
	}
	return count
}

// largestWaterBlob finds the largest connected region of water using BFS.
func (h *Hub) largestWaterBlob() int {
	w := h.world
	visited := make([]bool, w.Width*w.Height)
	largest := 0

	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			idx := y*w.Width + x
			if visited[idx] || w.Material[idx] != 4 { // MatWater == 4
				continue
			}
			// BFS from this cell
			size := 0
			queue := []int{idx}
			visited[idx] = true
			for len(queue) > 0 {
				ci := queue[0]
				queue = queue[1:]
				size++
				cx, cy := ci%w.Width, ci/w.Width
				for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx < 0 || nx >= w.Width || ny < 0 || ny >= w.Height {
						continue
					}
					ni := ny*w.Width + nx
					if !visited[ni] && w.Material[ni] == 4 {
						visited[ni] = true
						queue = append(queue, ni)
					}
				}
			}
			if size > largest {
				largest = size
			}
		}
	}
	return largest
}

// awardGoalBonus grants all connected players bonus influence and score points.
func (h *Hub) awardGoalBonus() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		// Award 50 bonus influence (direct add to the pool, capped at 200)
		c.Player.AddBonusInfluence(50)
		// Record score bonus
		h.Scoreboard.RecordGoalBonus(h.WorldName, c.Player.ID, 100)
		// Update player score/level
		if ps := h.Scoreboard.GetPlayerScore(h.WorldName, c.Player.ID); ps != nil {
			c.Player.UpdateScore(ps.Score)
		}
		// Notify client
		c.sendJSON(PlayerStateMsg{
			Type:           MsgPlayerState,
			PlayerID:       c.Player.ID,
			Influence:      c.Player.Influence(),
			MaxInfluence:   c.Player.MaxInfluenceCap(),
			Level:          c.Player.Level(),
			Score:          c.Player.Score(),
			NextLevelScore: c.Player.NextLevelScore(),
		})
	}
}
