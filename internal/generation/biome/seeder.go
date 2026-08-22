// Package biome seeds water basins and initial vegetation onto generated terrain.
// It is the fourth stage of the world generation pipeline:
//
//	TerrainGenerator → GeologyGenerator → CaveGenerator → BiomeSeeder ← (here)
//	→ ClimateInitializer → Simulation warm-up → Initial World
package biome

import (
	"math/rand"

	"github.com/worldweaver/worldweaver/internal/systems/materials"
	"github.com/worldweaver/worldweaver/internal/world"
)

// Seeder places water basins and initial plant seeds on suitable terrain.
type Seeder struct {
	rng *rand.Rand
}

// New creates a Seeder with the given seeded RNG.
func New(rng *rand.Rand) *Seeder { return &Seeder{rng: rng} }

// Seed carves water basins and sprinkles plant seeds across exposed soil.
func (s *Seeder) Seed(w *world.World) {
	// 2–4 randomised water basins
	nBasins := 2 + s.rng.Intn(3)
	for range nBasins {
		cx := w.Width/6 + s.rng.Intn(w.Width*2/3)
		radius := w.Width/12 + s.rng.Intn(w.Width/10)
		s.carveBasin(w, cx, radius)
	}

	// Sparse plant seeds on exposed soil surface
	for x := range w.Width {
		for y := 1; y < w.Height-1; y++ {
			if w.Material[y*w.Width+x] == materials.Soil &&
				w.Material[(y-1)*w.Width+x] == materials.Empty &&
				s.rng.Intn(100) < 4 {
				w.Material[(y-1)*w.Width+x] = materials.Plant
			}
		}
	}

	// Mark all chunks dirty so the initial snapshot broadcasts the full world.
	for i := range w.Chunks {
		w.Chunks[i].Dirty = true
		w.Chunks[i].Active = true
	}
}

func (s *Seeder) carveBasin(w *world.World, cx, radius int) {
	// Locate surface at cx
	surfaceY := w.Height / 2
	for y := range w.Height {
		if w.Material[y*w.Width+clampInt(cx, 0, w.Width-1)] != materials.Empty {
			surfaceY = y
			break
		}
	}
	depth := 4 + s.rng.Intn(8)
	for dy := range depth {
		for dx := -radius; dx <= radius; dx++ {
			x, y := cx+dx, surfaceY+dy
			if w.Index(x, y) < 0 {
				continue
			}
			w.Material[y*w.Width+x] = materials.Water
		}
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
