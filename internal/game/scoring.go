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

	// Internal: time of connection for PlayTime calculation
	connectedAt time.Time
}

// ComputeScore calculates the composite score from individual metrics.
//
// Formula:
//
//	Score = CellsAffected*1 + CreaturesSpawned*5 + WaterCreated*2 + max(0, StabilityContribution)*10
func (ps *PlayerScore) ComputeScore() int {
	stabilityBonus := float32(0)
	if ps.StabilityContribution > 0 {
		stabilityBonus = ps.StabilityContribution * 10
	}
	score := float32(ps.CellsAffected)*1 +
		float32(ps.CreaturesSpawned)*5 +
		float32(ps.WaterCreated)*2 +
		stabilityBonus
	return int(math.Round(float64(score)))
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
// The caller specifies the power type, how many cells were affected, and
// the influence cost that was spent.
func (sb *Scoreboard) RecordPowerAction(worldName string, playerID uint32, power uint8, cellsAffected int, influenceCost float32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	ps := sb.getOrCreate(worldName, playerID)

	ps.InfluenceSpent += influenceCost
	ps.CellsAffected += cellsAffected

	switch power {
	case PowerRain:
		ps.WaterCreated += cellsAffected
	case PowerHeat:
		ps.FiresStarted += cellsAffected
	case PowerGrowth:
		ps.CreaturesSpawned += cellsAffected
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
