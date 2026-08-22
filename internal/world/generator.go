package world

import "math/rand"

// generator builds the initial world terrain.
type generator struct {
	w   *World
	rng *rand.Rand
}

func newGenerator(w *World, rng *rand.Rand) *generator {
	return &generator{w: w, rng: rng}
}

// generate produces a simple side-view cross-section:
//   top portion   → empty (air)
//   upper-middle  → scattered soil/sand
//   lower-middle  → more soil/rock base
//   bottom layer  → solid rock
//   water basins  → carved into the terrain
//   plants        → sparse seeds on soil surface
func (g *generator) generate() {
	w := g.w
	skyLine  := w.Height / 5       // top 20% is pure air
	soilLine := w.Height * 3 / 5   // soil/sand zone starts here
	rockLine := w.Height * 4 / 5   // dense rock below here

	for y := range w.Height {
		for x := range w.Width {
			var mat uint8
			switch {
			case y < skyLine:
				mat = MatEmpty
			case y < soilLine:
				// Upper zone: mostly empty with occasional sand
				if g.rng.Intn(100) < 3 {
					mat = MatSand
				} else {
					mat = MatEmpty
				}
			case y < rockLine:
				// Mid zone: soil and sand mixture
				r := g.rng.Intn(100)
				switch {
				case r < 55:
					mat = MatSoil
				case r < 75:
					mat = MatSand
				default:
					mat = MatEmpty
				}
			default:
				// Bottom zone: solid rock
				mat = MatRock
			}
			w.Material[y*w.Width+x] = mat
		}
	}

	// Carve water basins
	g.carveBasin(w.Width/4, soilLine+5, w.Width/8)
	g.carveBasin(w.Width*2/3, soilLine+3, w.Width/10)

	// Seed sparse plants on soil surface
	for x := range w.Width {
		for y := soilLine; y < rockLine-1; y++ {
			if w.Material[y*w.Width+x] == MatSoil &&
				w.Material[(y-1)*w.Width+x] == MatEmpty &&
				g.rng.Intn(100) < 4 {
				w.Material[(y-1)*w.Width+x] = MatPlant
			}
		}
	}

	// Mark all chunks dirty for initial snapshot
	for i := range w.Chunks {
		w.Chunks[i].Dirty = true
		w.Chunks[i].Active = true
	}
}

// carveBasin fills a rectangular depression with water.
func (g *generator) carveBasin(cx, cy, radius int) {
	w := g.w
	for dy := -radius / 4; dy <= radius/4; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			x, y := cx+dx, cy+dy
			if w.Index(x, y) < 0 {
				continue
			}
			w.Material[y*w.Width+x] = MatWater
		}
	}
}
