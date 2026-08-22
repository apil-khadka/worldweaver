// Package terrain provides the base terrain generation stage for WorldWeaver.
//
// # Generation pipeline position
//
//  WorldSeed
//      → TerrainGenerator   ← (this package)
//      → GeologyGenerator
//      → CaveGenerator
//      → BiomeGenerator
//      → WaterSeeder
//      → VegetationSeeder
//      → ClimateInitializer
//      → Simulation warm-up
//
// # Design
//
// Each generator stage receives a *world.World and writes its output directly
// into the world arrays.  Stages are independently benchmarkable:
//
//	go test -bench=BenchmarkTerrainGenerator ./internal/generation/terrain/
//
// Noise is generated using the included simplex implementation (no external dep).
// Domain warping is applied to break up perfectly smooth noise bands and produce
// more natural-looking terrain boundaries.
package terrain

import (
	"math"
	"math/rand"

	"github.com/worldweaver/worldweaver/internal/systems/materials"
	"github.com/worldweaver/worldweaver/internal/world"
)

// Config controls the terrain generator.
// All fields have sensible defaults from DefaultConfig().
type Config struct {
	// HeightScale controls how jagged the surface is (0.0–1.0).
	HeightScale float64
	// SoilDepth is the average number of soil layers below surface.
	SoilDepth int
	// RockDepth is the y-fraction at which rock becomes solid.
	RockDepth float64
	// DomainWarpAmplitude controls lateral warping of height bands.
	DomainWarpAmplitude float64
}

// DefaultConfig returns conservative defaults suitable for a 1024×512 world.
func DefaultConfig() Config {
	return Config{
		HeightScale:         0.35,
		SoilDepth:           12,
		RockDepth:           0.75,
		DomainWarpAmplitude: 0.15,
	}
}

// Generator is the terrain generation stage.
type Generator struct {
	cfg Config
	rng *rand.Rand
}

// New creates a Generator with the given config and seeded RNG.
func New(cfg Config, rng *rand.Rand) *Generator {
	return &Generator{cfg: cfg, rng: rng}
}

// Generate writes base terrain into the world.
// Later stages (geology, water, vegetation) build on top of this.
func (g *Generator) Generate(w *world.World) {
	midY := float64(w.Height) * 0.50 // horizon line

	for x := range w.Width {
		fx := float64(x) / float64(w.Width)

		// Domain-warped simplex for surface height
		warpX := fx + g.cfg.DomainWarpAmplitude*simplex1(fx*3.1+0.7)
		h := simplex1(warpX*4.0)*0.5 + simplex1(warpX*8.0)*0.25 + simplex1(warpX*16.0)*0.125
		h = (h + 1) / 2 // normalize 0–1
		surfaceY := int(midY + h*float64(w.Height)*g.cfg.HeightScale - float64(w.Height)*g.cfg.HeightScale*0.5)
		surfaceY = clamp(surfaceY, 4, w.Height-4)

		rockLine := int(float64(w.Height) * g.cfg.RockDepth)

		for y := range w.Height {
			idx := y*w.Width + x
			switch {
			case y < surfaceY:
				w.Material[idx] = materials.Empty
			case y < surfaceY+g.cfg.SoilDepth:
				w.Material[idx] = materials.Soil
			case y < rockLine:
				// Below soil: sand and soil mix
				if g.rng.Intn(3) == 0 {
					w.Material[idx] = materials.Sand
				} else {
					w.Material[idx] = materials.Soil
				}
			default:
				w.Material[idx] = materials.Rock
			}
		}
	}
}

// ─── simplex noise (1-D, no external dep) ────────────────────────────────────

var perm [512]int

func init() {
	p := [256]int{}
	for i := range p {
		p[i] = i
	}
	// fixed shuffle for consistent results across seedings
	for i := 255; i > 0; i-- {
		j := int(math.Mod(float64(i*i+i+41), float64(i+1)))
		p[i], p[j] = p[j], p[i]
	}
	for i := range 512 {
		perm[i] = p[i&255]
	}
}

func simplex1(x float64) float64 {
	i0 := int(math.Floor(x))
	i1 := i0 + 1
	x0 := x - float64(i0)
	x1 := x0 - 1
	t0 := 1 - x0*x0
	t0 *= t0
	t1 := 1 - x1*x1
	t1 *= t1
	g0 := float64(perm[i0&255]%512 - 256)
	g1 := float64(perm[i1&255]%512 - 256)
	return 0.395 * (t0*t0*g0*x0 + t1*t1*g1*x1)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
