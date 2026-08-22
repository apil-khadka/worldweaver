package config

import (
	"errors"
	"fmt"
	"time"
)

// Tuning holds gameplay balance constants.
// These are design decisions, not protocol invariants.
// Centralised here so designers can adjust without hunting through packages.
// Units are explicit in every field name or comment.
type Tuning struct {
	// Influence economy
	InfluenceMax         float32 // max influence points
	InfluenceRegenPerSec float32 // points regenerated per second at 60 TPS

	// Power costs — influence drained per second while active
	RainCostPerSec   float32
	HeatCostPerSec   float32
	WindCostPerSec   float32
	GrowthCostPerSec float32

	// Simulation thresholds
	FireLifetimeTicks        int   // base ticks before fire dies
	PlantGrowthIntervalTicks int   // ticks between plant spread attempts
	EvapTempThreshold        int16 // fixed-point tenths of °C; water→vapor above this
	FreezeTempThreshold      int16 // water→ice below this (future)

	// Power limits — server-enforced, client must respect
	MaxInfluenceRadius int
	MaxIntensity       float32
}

// DefaultTuning returns the baseline gameplay balance.
// All values are intentionally conservative and measurable.
func DefaultTuning() Tuning {
	return Tuning{
		InfluenceMax:             100,
		InfluenceRegenPerSec:     8.0,
		RainCostPerSec:           2.0,
		HeatCostPerSec:           3.0,
		WindCostPerSec:           1.0,
		GrowthCostPerSec:         4.0,
		FireLifetimeTicks:        120,
		PlantGrowthIntervalTicks: 200,
		EvapTempThreshold:        1000, // 100.0 °C
		FreezeTempThreshold:      0,    // 0.0 °C
		MaxInfluenceRadius:       64,
		MaxIntensity:             1.0,
	}
}

// Validate returns a descriptive error if any ServerConfig value is out of range.
// Called at startup before any goroutines are launched — fail fast.
func (c *ServerConfig) Validate() error {
	if c.WorldWidth <= 0 {
		return fmt.Errorf("world width must be > 0, got %d", c.WorldWidth)
	}
	if c.WorldHeight <= 0 {
		return fmt.Errorf("world height must be > 0, got %d", c.WorldHeight)
	}
	if c.ChunkSize <= 0 || c.ChunkSize > c.WorldWidth {
		return fmt.Errorf("chunk size must be 1–WorldWidth, got %d", c.ChunkSize)
	}
	if c.TickRate <= 0 || c.TickRate > 240 {
		return fmt.Errorf("tick rate must be 1–240 Hz, got %d", c.TickRate)
	}
	if c.NetworkRate <= 0 || c.NetworkRate > c.TickRate {
		return errors.New("network rate must be > 0 and <= tick rate")
	}
	if c.MaxPlayers <= 0 {
		return fmt.Errorf("max players must be > 0, got %d", c.MaxPlayers)
	}
	if c.SnapshotInterval < 10*time.Second {
		return fmt.Errorf("snapshot interval must be >= 10s, got %v", c.SnapshotInterval)
	}
	return nil
}
