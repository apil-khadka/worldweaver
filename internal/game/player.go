// Package game owns player state, influence economy, powers, and world
// stability — the game mechanics layer on top of the raw simulation.
package game

import (
	"sync"
	"sync/atomic"
)

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
