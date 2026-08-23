package game

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// PlayerScore tracks all scoring metrics for a single player within one world.
type PlayerScore struct {
	PlayerID             uint32  `json:"playerID"`
	InfluenceSpent       float32 `json:"influenceSpent"`
	CellsAffected        int     `json:"cellsAffected"`
	CreaturesSpawned     int     `json:"creaturesSpawned"`
	FiresStarted         int     `json:"firesStarted"`
	WaterCreated         int     `json:"waterCreated"`
	StabilityContribution float32 `json:"stabilityContribution"`
	PlayTime             int     `json:"playTime"` // seconds connected
	Score                int     `json:"score"`

	// EarnedScore accumulates the per-action award after anti-farm damping. It is
	// the authoritative progression number; the counters above are activity
	// statistics that used to double as score inputs and no longer do.
	EarnedScore float64 `json:"earnedScore"`

	// Internal: time of connection for PlayTime calculation
	connectedAt time.Time

	// ── Anti-farm state ──────────────────────────────────────────────────────
	//
	// Previously absent entirely, which is why holding one power on one spot paid
	// out indefinitely.

	// lastX/lastY are where the previous application landed, for the repetition and
	// movement rules.
	lastX, lastY int
	hasLast      bool

	// actionsThisMinute counts applications inside the current window, for the rate
	// rule. windowStart is when that window opened.
	actionsThisMinute int
	windowStart       time.Time
}

// Anti-farm tuning.
const (
	// areaBonusWeight scales the sqrt(cells) term. Low enough that the flat base
	// dominates for ordinary brush sizes.
	areaBonusWeight = 0.55

	// repeatRadius is how close an application must be to the previous one to count
	// as painting the same spot again.
	repeatRadius = 6

	// repeatPenalty is the multiplier applied when it is. Not zero: repeating a spot
	// is sometimes legitimate (building up a wall), it just should not be the
	// optimal way to progress.
	repeatPenalty = 0.1

	// movementBonus rewards working across the world rather than into one hole.
	movementBonus = 1.5

	// movementDistance is how far an application must be from the last one to earn
	// that bonus.
	movementDistance = 40

	// rateFullActions is how many actions per minute score at full value.
	rateFullActions = 30
	// rateHalfActions is the count after which actions score at a tenth.
	rateHalfActions = 60
)

// antiFarmMultiplier computes the damping for one application.
//
// Three independent rules, multiplied:
//
//   - Repetition: an application within repeatRadius of the previous one scores a
//     tenth. This is what stops holding the button on one spot from paying out.
//   - Rate: the first 30 actions in a minute score fully, the next 30 at half, the
//     rest at a tenth. This bounds the ceiling regardless of technique.
//   - Movement: an application more than movementDistance from the last scores 1.5x,
//     rewarding working across the world.
func (ps *PlayerScore) antiFarmMultiplier(x, y, cellsAffected int, now time.Time) float64 {
	multiplier := 1.0

	// ── Rate rule ────────────────────────────────────────────────────────────
	if ps.windowStart.IsZero() || now.Sub(ps.windowStart) >= time.Minute {
		ps.windowStart = now
		ps.actionsThisMinute = 0
	}
	ps.actionsThisMinute++
	switch {
	case ps.actionsThisMinute > rateHalfActions:
		multiplier *= 0.1
	case ps.actionsThisMinute > rateFullActions:
		multiplier *= 0.5
	}

	// ── Repetition and movement rules ────────────────────────────────────────
	if ps.hasLast {
		dx := x - ps.lastX
		dy := y - ps.lastY
		distSq := dx*dx + dy*dy

		if distSq <= repeatRadius*repeatRadius {
			multiplier *= repeatPenalty
		} else if distSq >= movementDistance*movementDistance {
			multiplier *= movementBonus
		}
	}
	ps.lastX, ps.lastY = x, y
	ps.hasLast = true

	return multiplier
}

// ComputeScore calculates the composite score.
//
// Score is now EarnedScore — the accumulated per-action award after anti-farm
// damping — plus a stability bonus for keeping the world healthy.
//
// The old formula was CellsAffected*1 + CreaturesSpawned*5 + WaterCreated*2 +
// stability*10, where the first three terms were all per-cell counters. That made
// score a function of total brush area swept, which is why max level was reachable
// in about two seconds. Those counters are still tracked and reported, but as
// activity statistics rather than score inputs.
func (ps *PlayerScore) ComputeScore() int {
	stabilityBonus := float64(0)
	if ps.StabilityContribution > 0 {
		stabilityBonus = float64(ps.StabilityContribution) * 10
	}
	return int(math.Round(ps.EarnedScore + stabilityBonus))
}

// Scoreboard manages per-world, per-player scores thread-safely.
type Scoreboard struct {
	mu     sync.RWMutex
	worlds map[string]map[uint32]*PlayerScore // worldName → playerID → score
}

// NewScoreboard creates a fresh scoreboard.
func NewScoreboard() *Scoreboard {
	return &Scoreboard{
		worlds: make(map[string]map[uint32]*PlayerScore),
	}
}

// PlayerConnected registers a player connection for PlayTime tracking.
func (sb *Scoreboard) PlayerConnected(worldName string, playerID uint32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)
	ps.connectedAt = time.Now()
}

// PlayerDisconnected finalizes PlayTime for a disconnecting player.
func (sb *Scoreboard) PlayerDisconnected(worldName string, playerID uint32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if worldScores, ok := sb.worlds[worldName]; ok {
		if ps, ok := worldScores[playerID]; ok {
			if !ps.connectedAt.IsZero() {
				ps.PlayTime += int(time.Since(ps.connectedAt).Seconds())
				ps.connectedAt = time.Time{}
			}
		}
	}
}

// RecordPowerAction updates a player's score based on a power usage event.
//
// # Scoring is per ACTION, not per cell
//
// This function used to add cellsAffected to every counter, so score grew as the
// SQUARE of the brush radius: a radius-8 brush covers 197 cells, making one Rain
// application worth 591 points and one second of holding the button worth 4,728
// against a max-level threshold of 10,000. Max level arrived in 2.1 seconds, and in
// 0.23 seconds at the radius-24 cap. Brush size, not skill or time, was the entire
// progression system.
//
// Reward is now a flat per-action base plus a sqrt(cells) area term, so a wider brush
// is a convenience rather than a multiplier — doubling the radius roughly doubles the
// area term instead of quadrupling the score.
//
// Three anti-farm rules then apply, because the scoreboard previously had none at
// all: a player could hold one power on one spot forever and score every tick even
// though nothing about the world was changing.
func (sb *Scoreboard) RecordPowerAction(
	worldName string, playerID uint32, power uint8,
	cellsAffected int, influenceCost float32,
) {
	sb.recordPowerActionAt(worldName, playerID, power, cellsAffected, influenceCost, 0, 0, time.Now())
}

// RecordPowerActionAt is RecordPowerAction with the application's world position, so
// the repetition and movement rules can be applied. Prefer this from the hub.
func (sb *Scoreboard) RecordPowerActionAt(
	worldName string, playerID uint32, power uint8,
	cellsAffected int, influenceCost float32, x, y int,
) {
	sb.recordPowerActionAt(worldName, playerID, power, cellsAffected, influenceCost, x, y, time.Now())
}

func (sb *Scoreboard) recordPowerActionAt(
	worldName string, playerID uint32, power uint8,
	cellsAffected int, influenceCost float32, x, y int, now time.Time,
) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)

	// Influence and raw cell count are still tracked verbatim: they are honest
	// activity statistics and are shown in the HUD. They just no longer drive score.
	ps.InfluenceSpent += influenceCost
	ps.CellsAffected += cellsAffected

	multiplier := ps.antiFarmMultiplier(x, y, cellsAffected, now)

	// Base reward per action, before the area term. Powers that do more interesting
	// work to the world are worth more.
	base := 4.0
	switch power {
	case PowerGrowth:
		base = 7.0
	case PowerLife:
		base = 10.0
	case PowerRain:
		base = 5.0
	}

	// Sub-linear area term: sqrt keeps a big brush worthwhile without making it
	// strictly dominant.
	area := math.Sqrt(float64(cellsAffected))

	ps.EarnedScore += (base + area*areaBonusWeight) * multiplier

	// Per-power activity counters, kept for the leaderboard's flavour stats. These
	// are counts of what the player did, not score inputs.
	switch power {
	case PowerRain:
		ps.WaterCreated += cellsAffected
	case PowerHeat:
		ps.FiresStarted += cellsAffected
	case PowerGrowth:
		ps.CreaturesSpawned += cellsAffected
	case PowerLife:
		ps.CreaturesSpawned += cellsAffected * 2 // Life is stronger creature spawning
	}

	ps.Score = ps.ComputeScore()
}

// RecordCreatureSpawn adds creature spawns to the player's tally.
func (sb *Scoreboard) RecordCreatureSpawn(worldName string, playerID uint32, count int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)
	ps.CreaturesSpawned += count
	ps.Score = ps.ComputeScore()
}

// UpdateStability adjusts a player's stability contribution delta.
func (sb *Scoreboard) UpdateStability(worldName string, playerID uint32, delta float32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)
	ps.StabilityContribution += delta
	ps.Score = ps.ComputeScore()
}

// RecordGoalBonus adds bonus points from completing a cooperative goal.
func (sb *Scoreboard) RecordGoalBonus(worldName string, playerID uint32, bonus int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)
	ps.Score += bonus
}

// TopScores returns the top N players for the given world, sorted by score descending.
func (sb *Scoreboard) TopScores(worldName string, n int) []PlayerScore {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	worldScores, ok := sb.worlds[worldName]
	if !ok {
		return nil
	}

	scores := make([]PlayerScore, 0, len(worldScores))
	for _, ps := range worldScores {
		// Update PlayTime for currently connected players
		entry := *ps
		if !ps.connectedAt.IsZero() {
			entry.PlayTime += int(time.Since(ps.connectedAt).Seconds())
		}
		scores = append(scores, entry)
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	if len(scores) > n {
		scores = scores[:n]
	}
	return scores
}

// GetPlayerScore returns a single player's score for the given world.
func (sb *Scoreboard) GetPlayerScore(worldName string, playerID uint32) *PlayerScore {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	if worldScores, ok := sb.worlds[worldName]; ok {
		if ps, ok := worldScores[playerID]; ok {
			entry := *ps
			if !ps.connectedAt.IsZero() {
				entry.PlayTime += int(time.Since(ps.connectedAt).Seconds())
			}
			return &entry
		}
	}
	return nil
}

// getOrCreate returns the PlayerScore entry, creating it if it doesn't exist.
// Caller must hold sb.mu.
// ScoreOf returns a player's current computed score, or zero if unknown.
//
// A small convenience over GetPlayerScore for callers and tests that only need the
// number, so they do not have to nil-check a struct pointer to read one field.
func (sb *Scoreboard) ScoreOf(worldName string, playerID uint32) int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	if worldScores, ok := sb.worlds[worldName]; ok {
		if ps, ok := worldScores[playerID]; ok {
			return ps.ComputeScore()
		}
	}
	return 0
}

func (sb *Scoreboard) getOrCreate(worldName string, playerID uint32) *PlayerScore {
	if _, ok := sb.worlds[worldName]; !ok {
		sb.worlds[worldName] = make(map[uint32]*PlayerScore)
	}
	if _, ok := sb.worlds[worldName][playerID]; !ok {
		sb.worlds[worldName][playerID] = &PlayerScore{
			PlayerID:    playerID,
			connectedAt: time.Now(),
		}
	}
	return sb.worlds[worldName][playerID]
}

// ScoresAPIHandler returns an HTTP handler for GET /api/scores?world=<name>.
func ScoresAPIHandler(sb *Scoreboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		worldName := r.URL.Query().Get("world")
		if worldName == "" {
			worldName = "genesis"
		}

		scores := sb.TopScores(worldName, 10)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{
			"world":  worldName,
			"scores": scores,
		})
	}
}
