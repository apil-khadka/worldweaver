// Package climate sets initial environmental field values after terrain
// generation completes.
//
// Climate initialization is the last generation stage before the simulation
// warm-up.  It seeds temperature and moisture arrays with values consistent
// with the generated terrain so that the simulation starts in a plausible
// state rather than an artificially cold/dry blank slate.
package climate

import (
	"math/rand"

	"github.com/worldweaver/worldweaver/internal/world"
)

// Config controls initial climate conditions.
type Config struct {
	// BaseTemperature is the starting temperature (fixed-point tenths of °C).
	// 200 = 20.0°C.
	BaseTemperature int16
	// BaseMoisture is the starting moisture byte (0–255) for soil cells.
	BaseMoisture uint8
	// TemperatureVariance is ±random noise applied per cell.
	TemperatureVariance int16
}

// DefaultConfig returns a temperate-world climate.
func DefaultConfig() Config {
	return Config{
		BaseTemperature:     200,
		BaseMoisture:        60,
		TemperatureVariance: 50,
	}
}

// Initializer seeds the environmental fields.
type Initializer struct {
	cfg Config
	rng *rand.Rand
}

// New creates an Initializer.
func New(cfg Config, rng *rand.Rand) *Initializer {
	return &Initializer{cfg: cfg, rng: rng}
}

// Initialize writes initial temperature and moisture values into the world.
func (ci *Initializer) Initialize(w *world.World) {
	for y := range w.Height {
		// Temperature decreases slightly with altitude (higher y = lower altitude
		// in our side-view, so cells near the top get a tiny temperature bonus).
		altitudeFactor := int16((y * 20) / w.Height)
		for x := range w.Width {
			i := w.Index(x, y)
			if i < 0 {
				continue
			}
			jitter := int16(ci.rng.Intn(int(ci.cfg.TemperatureVariance)*2)) - ci.cfg.TemperatureVariance
			w.Temperature[i] = ci.cfg.BaseTemperature + altitudeFactor + jitter

			// Seed soil moisture
			mat := w.Material[i]
			if mat == 2 /* Soil */ {
				w.Moisture[i] = ci.cfg.BaseMoisture + uint8(ci.rng.Intn(30))
			}
		}
	}
}
