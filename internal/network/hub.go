package network

import (
	"log"
	"sync"

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
}

// NewHub creates a Hub wired to the given world and engine.
func NewHub(w *world.World, eng *simulation.Engine, m *metrics.Metrics) *Hub {
	return &Hub{
		clients:     make(map[*Client]struct{}),
		world:       w,
		engine:      eng,
		metrics:     m,
		connLimiter: NewConnRateLimiter(),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.metrics.PlayerCount.Add(1)
	log.Printf("hub: player %d registered (total %d)", c.Player.ID, h.metrics.PlayerCount.Load())
	h.BroadcastPlayerJoin(c.Player.ID)
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	h.metrics.PlayerCount.Add(-1)
	log.Printf("hub: player %d unregistered (total %d)", c.Player.ID, h.metrics.PlayerCount.Load())
	h.BroadcastPlayerLeave(c.Player.ID)
}

// handlePowerInput validates an incoming power request and enqueues it on
// the simulation engine.  Validation happens on the network goroutine to keep
// the simulation loop clean.
func (h *Hub) handlePowerInput(c *Client, msg *PowerInputMsg) {
	req := &game.PowerRequest{
		PlayerID:  c.Player.ID,
		Power:     msg.Power,
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
		Power:     simulation.PowerType(req.Power),
		X:         req.X,
		Y:         req.Y,
		Radius:    req.Radius,
		Intensity: req.Intensity,
	})

	// Immediately send updated influence state so the client UI stays in sync.
	c.sendJSON(PlayerStateMsg{
		Type:         MsgPlayerState,
		PlayerID:     c.Player.ID,
		Influence:    c.Player.Influence(),
		MaxInfluence: 100,
	})
}

// PlayerCount returns the current number of connected clients.
func (h *Hub) PlayerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// handleCursorMove broadcasts a player's cursor position to all other clients.
func (h *Hub) handleCursorMove(sender *Client, msg *CursorMsg) {
	update := mustMarshal(CursorUpdateMsg{
		Type:     MsgCursorUpdate,
		PlayerID: sender.Player.ID,
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
			Type:         MsgPlayerState,
			PlayerID:     c.Player.ID,
			Influence:    c.Player.Influence(),
			MaxInfluence: 100,
		})
	}
}
