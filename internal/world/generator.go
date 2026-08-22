package world

import "math"
import "math/rand"

// generator builds the initial world terrain.
type generator struct {
	w   *World
	rng *rand.Rand
}

func newGenerator(w *World, rng *rand.Rand) *generator {
	return &generator{w: w, rng: rng}
}

// generate produces a spectacular demo world designed to show off the
// simulation immediately:
//   - Large mountain/plateau in center with cave networks
//   - A water basin/lake pre-filled on the left side
//   - Vegetation growing near the water
//   - A lava pool deep underground
//   - A dramatic overhang where sand can cascade on load
//   - Oil droplets (herbivore stand-in) scattered on vegetation
func (g *generator) generate() {
	w := g.w
	W := w.Width
	H := w.Height

	// Fill everything with empty first
	for i := range w.Material {
		w.Material[i] = MatEmpty
	}

	// ── Terrain heightmap using layered sine waves ───────────────────────────
	// Creates a mountain in the center, valleys on sides
	heights := make([]int, W)
	centerX := float64(W) / 2.0
	for x := range W {
		fx := float64(x)
		// Base terrain: gentle slope
		base := float64(H) * 0.55

		// Central mountain: gaussian peak
		dist := (fx - centerX) / (float64(W) * 0.2)
		mountain := float64(H) * 0.3 * math.Exp(-dist*dist)

		// Add some noise for natural look
		noise := math.Sin(fx*0.05)*8 + math.Sin(fx*0.12)*4 + math.Sin(fx*0.03)*12

		heights[x] = int(base - mountain + noise)
	}

	// ── Fill terrain layers ─────────────────────────────────────────────────
	for x := range W {
		surfaceY := heights[x]
		for y := range H {
			if y < surfaceY {
				continue // air
			}
			depth := y - surfaceY
			switch {
			case depth < 3:
				w.Material[y*W+x] = MatSoil
			case depth < 8:
				if g.rng.Intn(100) < 70 {
					w.Material[y*W+x] = MatSoil
				} else {
					w.Material[y*W+x] = MatSand
				}
			case depth < 20:
				if g.rng.Intn(100) < 85 {
					w.Material[y*W+x] = MatRock
				} else {
					w.Material[y*W+x] = MatSoil
				}
			default:
				w.Material[y*W+x] = MatRock
			}
		}
	}

	// ── Central plateau/flat top ────────────────────────────────────────────
	plateauLeft := W*2/5
	plateauRight := W*3/5
	for x := plateauLeft; x < plateauRight; x++ {
		surfaceY := heights[x]
		// Flatten the top to form a plateau
		flatY := heights[W/2] - 5
		if surfaceY > flatY {
			for y := flatY; y < surfaceY; y++ {
				if y >= 0 && y < H {
					w.Material[y*W+x] = MatSoil
				}
			}
			heights[x] = flatY
		}
	}

	// ── Cave networks inside the mountain ───────────────────────────────────
	g.carveCave(W/2, H*55/100, 60, 15)
	g.carveCave(W/2-30, H*60/100, 40, 10)
	g.carveCave(W/2+25, H*65/100, 35, 12)

	// ── Water basin on the left side ────────────────────────────────────────
	// Carve a depression and fill with water
	basinCX := W / 5
	basinW := W / 6
	basinDepth := 25
	basinTopY := heights[basinCX] - 5

	for x := basinCX - basinW/2; x < basinCX+basinW/2; x++ {
		if x < 0 || x >= W {
			continue
		}
		// Parabolic depression
		dx := float64(x-basinCX) / float64(basinW/2)
		carveDepth := int(float64(basinDepth) * (1.0 - dx*dx))
		if carveDepth < 0 {
			carveDepth = 0
		}
		surfY := heights[x]
		for y := surfY; y < surfY+carveDepth && y < H; y++ {
			w.Material[y*W+x] = MatWater
		}
		// Fill above water surface level to the basin rim with water too
		waterLevel := basinTopY + 8
		for y := waterLevel; y < surfY && y < H; y++ {
			if y >= 0 {
				w.Material[y*W+x] = MatWater
			}
		}
	}

	// ── Vegetation near the water ───────────────────────────────────────────
	vegZoneLeft := basinCX - basinW/2 - 20
	vegZoneRight := basinCX + basinW/2 + 20
	for x := vegZoneLeft; x < vegZoneRight; x++ {
		if x < 0 || x >= W {
			continue
		}
		surfY := heights[x]
		// Place plants on surface where there's soil
		for dy := -2; dy <= 0; dy++ {
			y := surfY + dy
			if y < 1 || y >= H {
				continue
			}
			below := w.Material[y*W+x]
			above := w.Material[(y-1)*W+x]
			if below == MatSoil && above == MatEmpty && g.rng.Intn(100) < 35 {
				w.Material[(y-1)*W+x] = MatPlant
			}
		}
	}

	// ── Lava pool deep underground ──────────────────────────────────────────
	lavaCX := W / 2
	lavaCY := H * 85 / 100
	lavaW := 40
	lavaH := 10
	// First carve a chamber
	for dy := -lavaH; dy <= lavaH; dy++ {
		for dx := -lavaW; dx <= lavaW; dx++ {
			x, y := lavaCX+dx, lavaCY+dy
			if x < 0 || x >= W || y < 0 || y >= H {
				continue
			}
			// Elliptical shape
			ex := float64(dx) / float64(lavaW)
			ey := float64(dy) / float64(lavaH)
			if ex*ex+ey*ey <= 1.0 {
				if dy > lavaH/3 {
					w.Material[y*W+x] = MatLava
				} else {
					w.Material[y*W+x] = MatEmpty // air pocket above lava
				}
			}
		}
	}

	// ── Dramatic sand overhang (sand on thin soil shelf, will cascade) ───────
	overhangX := W * 3 / 4
	overhangY := heights[overhangX] - 3
	// Create a thin soil ledge jutting out
	for dx := 0; dx < 30; dx++ {
		x := overhangX + dx
		if x >= W {
			break
		}
		// The shelf: thin soil layer
		for dy := 0; dy < 2; dy++ {
			y := overhangY + dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatSoil
			}
		}
		// Pile sand on top of the shelf -- this will cascade off edges
		for dy := -8; dy < 0; dy++ {
			y := overhangY + dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatSand
			}
		}
		// Clear below the shelf so sand falls
		for dy := 2; dy < 40; dy++ {
			y := overhangY + dy
			if y >= 0 && y < H && w.Material[y*W+x] != MatRock {
				w.Material[y*W+x] = MatEmpty
			}
		}
	}

	// ── Scatter Oil droplets on vegetation (herbivore stand-in, mat 12) ─────
	for x := vegZoneLeft; x < vegZoneRight; x++ {
		if x < 0 || x >= W {
			continue
		}
		for y := 1; y < H; y++ {
			if w.Material[y*W+x] == MatPlant && w.Material[(y-1)*W+x] == MatEmpty {
				if g.rng.Intn(100) < 8 {
					w.Material[(y-1)*W+x] = MatOil
				}
			}
		}
	}

	// ── Unstable water above the basin (ready to flow on first tick) ────────
	// Add a perched water pocket that will immediately start flowing
	perchedX := basinCX + basinW/2 + 5
	perchedY := heights[basinCX] - 2
	for dy := 0; dy < 6; dy++ {
		for dx := 0; dx < 8; dx++ {
			x, y := perchedX+dx, perchedY+dy
			if x >= 0 && x < W && y >= 0 && y < H {
				w.Material[y*W+x] = MatWater
			}
		}
	}

	// ── Mark all chunks dirty for initial snapshot ──────────────────────────
	for i := range w.Chunks {
		w.Chunks[i].Dirty = true
		w.Chunks[i].Active = true
	}
}

// carveCave creates an organic-looking cave by random-walking a carver.
func (g *generator) carveCave(cx, cy, length, radius int) {
	w := g.w
	x, y := cx, cy
	for i := range length {
		// Carve a roughly circular area
		r := radius - g.rng.Intn(radius/3)
		_ = i
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					px, py := x+dx, y+dy
					if px >= 0 && px < w.Width && py >= 0 && py < w.Height {
						w.Material[py*w.Width+px] = MatEmpty
					}
				}
			}
		}
		// Random walk
		x += g.rng.Intn(7) - 3
		y += g.rng.Intn(5) - 2
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
