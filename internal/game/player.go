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

// Progression curve constants.
//
// The old curve was five levels at [0, 100, 500, 2000, 10000] against scoring that
// awarded a point per CELL affected. A radius-8 brush covers 197 cells, so one Rain
// application scored 591 and a second of holding the button scored 4,728 — max level
// arrived in 2.1 seconds. At the level-1 radius cap of 24 it arrived in 0.23
// seconds. The curve was not the problem on its own; it was overwhelmed because
// reward grew as the square of the brush radius while thresholds grew about 5x per
// level over only five levels.
//
// Scoring is now per ACTION with a sqrt area term (see scoring.go), and the curve is
// exponential over 25 levels so there is a long tail to climb.
const (
	// levelBaseScore is the score needed to reach level 2.
	levelBaseScore = 50.0

	// levelGrowth is the multiplier per level. 2.2 puts level 10 at roughly 27k and
	// level 20 around 72M, which against the post-fix earn rate is hours and months
	// respectively rather than seconds.
	levelGrowth = 2.2

	// MaxLevel is the top of the curve.
	MaxLevel = 25
)

// LevelThresholds maps level → unlock data, generated from the exponential curve.
//
// Generating it rather than hand-listing 25 rows means the curve is stated once and
// cannot drift out of step with itself, and the unlock schedule stays readable.
var LevelThresholds = buildLevelThresholds()

// LevelForScore returns the level a given score qualifies for.
//
// Walks the curve from the top so the highest satisfied threshold wins, which is what
// makes the function correct for any score including those beyond the last level.
func LevelForScore(score int) int {
	for i := len(LevelThresholds) - 1; i >= 0; i-- {
		if score >= LevelThresholds[i].Score {
			return i + 1
		}
	}
	return 1
}

// buildLevelThresholds computes the curve and attaches unlocks to it.
func buildLevelThresholds() []LevelThreshold {
	out := make([]LevelThreshold, 0, MaxLevel)

	for level := 1; level <= MaxLevel; level++ {
		score := 0
		if level > 1 {
			// threshold(level) = base * growth^(level-2), accumulated.
			cumulative := 0.0
			step := levelBaseScore
			for l := 2; l <= level; l++ {
				cumulative += step
				step *= levelGrowth
			}
			score = int(cumulative)
		}

		t := LevelThreshold{
			Score:        score,
			MaxInfluence: 100,
			InflRegen:    0.5,
			PowerRadius:  24,
		}

		// Unlock schedule. Levels 1-5 keep the unlocks they always had so existing
		// expectations (notably the level-4 Life power gate) are unchanged; 6+ are
		// new headroom.
		switch {
		case level >= 20:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 320, 1.6, 48
		case level >= 15:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 260, 1.4, 44
		case level >= 10:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 220, 1.2, 40
		case level >= 7:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 180, 1.0, 36
		case level >= 5:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 150, 0.8, 32
		case level >= 3:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 100, 0.8, 28
		case level >= 2:
			t.MaxInfluence, t.InflRegen, t.PowerRadius = 100, 0.5, 28
		}

		out = append(out, t)
	}

	return out
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
