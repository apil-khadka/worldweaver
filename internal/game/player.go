// Package game owns player state, influence economy, powers, and world
// stability — the game mechanics layer on top of the raw simulation.
package game

import (
	"sync"
	"sync/atomic"
)

// LevelThreshold defines score thresholds and unlocks for each level.
type LevelThreshold struct {
	Score        int
	MaxInfluence float32
	InflRegen    float32
	PowerRadius  int
}

// LevelThresholds maps level → unlock data. Level 1 is the default (no threshold).
var LevelThresholds = []LevelThreshold{
	{Score: 0, MaxInfluence: 100, InflRegen: 0.5, PowerRadius: 24},      // Level 1
	{Score: 100, MaxInfluence: 100, InflRegen: 0.5, PowerRadius: 28},    // Level 2: larger radius
	{Score: 500, MaxInfluence: 100, InflRegen: 0.8, PowerRadius: 28},    // Level 3: faster regen
	{Score: 2000, MaxInfluence: 100, InflRegen: 0.8, PowerRadius: 28},   // Level 4: Life power unlocked
	{Score: 10000, MaxInfluence: 150, InflRegen: 0.8, PowerRadius: 28},  // Level 5: max influence 150
}

// Player holds the runtime state of a connected player.
// All fields that are read/written from multiple goroutines must be accessed
// through the provided methods to ensure thread safety.
type Player struct {
	ID       uint32
	Nickname string
	mu       sync.RWMutex
	influence float32
	maxInfluence float32
	cameraX  int32
	cameraY  int32
	viewW    uint16
	viewH    uint16
	power    uint8
	colorR   uint8
	colorG   uint8
	colorB   uint8

	// Influence regeneration is applied per simulation tick.
	inflRegen float32

	// Level progression
	level int
	score int
}

// playerIDCounter is used to assign unique IDs to new players.
var playerIDCounter atomic.Uint32

// NewPlayer creates a new player with default settings.
func NewPlayer() *Player {
	return &Player{
		ID:           playerIDCounter.Add(1),
		influence:    100,
		maxInfluence: 100,
		inflRegen:    0.5, // 0.5 influence points per simulation tick
		viewW:        800,
		viewH:        600,
		level:        1,
	}
}

// NewPlayerWithID creates a player with a specific ID and nickname (for authenticated sessions).
func NewPlayerWithID(id uint32, nickname string) *Player {
	return &Player{
		ID:           id,
		Nickname:     nickname,
		influence:    100,
		maxInfluence: 100,
		inflRegen:    0.5,
		viewW:        800,
		viewH:        600,
		level:        1,
	}
}

// Influence returns the player's current influence.
func (p *Player) Influence() float32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.influence
}

// ConsumeInfluence deducts cost from the player's influence.
// Returns false (and does not deduct) if influence is insufficient.
func (p *Player) ConsumeInfluence(cost float32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.influence < cost {
		return false
	}
	p.influence -= cost
	return true
}

// RegenerateInfluence is called once per simulation tick to restore influence.
func (p *Player) RegenerateInfluence() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.influence += p.inflRegen
	if p.influence > p.maxInfluence {
		p.influence = p.maxInfluence
	}
}

// SetCamera updates the player's camera viewport position.
func (p *Player) SetCamera(x, y int32, w, h uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cameraX = x
	p.cameraY = y
	p.viewW = w
	p.viewH = h
}

// Camera returns the current viewport position and size.
func (p *Player) Camera() (x, y int32, w, h uint16) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cameraX, p.cameraY, p.viewW, p.viewH
}

// SetPower sets the player's active power selection.
func (p *Player) SetPower(power uint8) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.power = power
}

// Power returns the player's currently selected power.
func (p *Player) Power() uint8 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.power
}

// CursorPos returns the center of the player's viewport as an approximation
// of their cursor position (used for chat bubble placement).
func (p *Player) CursorPos() (int, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cx := int(p.cameraX) + int(p.viewW)/2
	cy := int(p.cameraY) + int(p.viewH)/2
	return cx, cy
}

// AddBonusInfluence adds bonus influence from goal completion, temporarily boosting the cap.
func (p *Player) AddBonusInfluence(amount float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.influence += amount
	if p.influence > 200 {
		p.influence = 200
	}
}

// Level returns the player's current level.
func (p *Player) Level() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.level
}

// Score returns the player's current score.
func (p *Player) Score() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.score
}

// NextLevelScore returns the score required for the next level, or -1 if max level.
func (p *Player) NextLevelScore() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.level >= len(LevelThresholds) {
		return -1 // max level
	}
	return LevelThresholds[p.level].Score
}

// UpdateScore sets the player's score and recalculates level.
// Returns true if the player leveled up.
func (p *Player) UpdateScore(score int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.score = score
	oldLevel := p.level
	newLevel := 1
	for i := 1; i < len(LevelThresholds); i++ {
		if score >= LevelThresholds[i].Score {
			newLevel = i + 1
		}
	}
	p.level = newLevel
	if newLevel > oldLevel {
		// Apply level-up bonuses
		t := LevelThresholds[newLevel-1]
		p.maxInfluence = t.MaxInfluence
		p.inflRegen = t.InflRegen
		return true
	}
	return false
}

// PowerRadius returns the player's power radius based on level.
func (p *Player) PowerRadius() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.level <= 0 || p.level > len(LevelThresholds) {
		return 24
	}
	return LevelThresholds[p.level-1].PowerRadius
}

// MaxInfluenceCap returns the player's current max influence (level-aware).
func (p *Player) MaxInfluenceCap() float32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxInfluence
}

// CanUsePower returns whether the player's level permits using the given power.
func (p *Player) CanUsePower(power uint8) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if power == PowerLife {
		return p.level >= 4
	}
	return true // Powers 0-3 are always available
}
