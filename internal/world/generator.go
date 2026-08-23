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

// generate produces a spectacular marketing-ready demo world designed to
// immediately impress within 3 seconds of loading:
//   - VOLCANO: dramatic cone of rock with lava core and smoke plume
//   - WATERFALL: water flowing off a cliff into a lake basin
//   - FOREST: dense vegetation on fertile soil near the lake
//   - CAVES: underground networks with exposed lava geology
//   - DESERT: sand dunes with an occasional oasis
//   - CREATURES: herbivores in the forest, predators on rocky crags
//   - Clear visual ZONES with designed feel, not random noise
func (g *generator) generate() {
	w := g.w
	W := w.Width
	H := w.Height

	// Fill everything with empty first
	for i := range w.Material {
		w.Material[i] = MatEmpty
	}

	// ── Layered terrain heightmap ───────────────────────────────────────────
	// Base terrain uses large-scale sin/cos waves + detail harmonics.
	// The world has distinct biome zones (left→right): Desert | Volcano | Forest+Lake | Cliffs
	heights := make([]int, W)
	for x := range W {
		fx := float64(x)
		frac := fx / float64(W) // 0.0 → 1.0 across the world

		// Base terrain: gentle rolling hills
		base := float64(H) * 0.52

		// Zone shaping: desert is flat, volcano is a peak, forest is a valley, right side has cliffs
		var zoneShape float64

		// Desert zone (left 25%): flat with gentle dunes
		if frac < 0.25 {
			localFrac := frac / 0.25
			dune := math.Sin(fx*0.04)*6 + math.Sin(fx*0.09)*3
			zoneShape = dune + 5.0*math.Sin(localFrac*math.Pi) // slight depression
		}

		// Volcano zone (25-45%): dramatic cone
		if frac >= 0.25 && frac < 0.45 {
			volcanoCenterFrac := 0.35
			dist := (frac - volcanoCenterFrac) / 0.10
			volcanoHeight := float64(H) * 0.35 * math.Exp(-dist*dist*2.5)
			zoneShape = -volcanoHeight // negative = higher terrain
		}

		// Forest/lake valley (45-75%): lower terrain with a lake basin
		if frac >= 0.45 && frac < 0.75 {
			localFrac := (frac - 0.45) / 0.30
			valleyDepth := 30.0 * math.Sin(localFrac * math.Pi)
			zoneShape = valleyDepth
		}

		// Cliff zone (75-100%): dramatic cliff with waterfall source
		if frac >= 0.75 {
			localFrac := (frac - 0.75) / 0.25
			cliffRise := -float64(H) * 0.15 * (1.0 - math.Exp(-localFrac*4.0))
			zoneShape = cliffRise
		}

		// Detail noise (harmonics of sin/cos — no external noise lib)
		detail := math.Sin(fx*0.05)*8 +
			math.Cos(fx*0.12)*4 +
			math.Sin(fx*0.03+1.7)*10 +
			math.Sin(fx*0.2)*2

		heights[x] = int(base + zoneShape + detail)
		if heights[x] < 10 {
			heights[x] = 10
		}
		if heights[x] > H-20 {
			heights[x] = H - 20
		}
	}

	// ── Fill terrain layers ─────────────────────────────────────────────────
	for x := range W {
		surfaceY := heights[x]
		frac := float64(x) / float64(W)
		for y := surfaceY; y < H; y++ {
			depth := y - surfaceY
			if frac < 0.25 {
				// Desert: sand on top, rock below
				switch {
				case depth < 6:
					w.Material[y*W+x] = MatSand
				case depth < 15:
					if g.rng.Intn(100) < 40 {
						w.Material[y*W+x] = MatSand
					} else {
						w.Material[y*W+x] = MatRock
					}
				default:
					w.Material[y*W+x] = MatRock
				}
			} else if frac >= 0.45 && frac < 0.75 {
				// Forest valley: rich soil
				switch {
				case depth < 9:
					w.Material[y*W+x] = MatSoil
				case depth < 18:
					if g.rng.Intn(100) < 60 {
						w.Material[y*W+x] = MatSoil
					} else {
						w.Material[y*W+x] = MatRock
					}
				default:
					w.Material[y*W+x] = MatRock
				}
			} else {
				// Volcano / cliff zones: thin soil over rock
				switch {
				case depth < 4:
					w.Material[y*W+x] = MatSoil
				case depth < 10:
					if g.rng.Intn(100) < 35 {
						w.Material[y*W+x] = MatSoil
					} else {
						w.Material[y*W+x] = MatRock
					}
				default:
					w.Material[y*W+x] = MatRock
				}
			}
		}
	}

	// ── VOLCANO: cone with lava core and smoke ──────────────────────────────
	volcanoX := int(0.35 * float64(W))
	volcanoTopY := heights[volcanoX]
	// Carve crater at the top
	craterW := 16
	craterDepth := 20
	for dx := -craterW; dx <= craterW; dx++ {
		x := volcanoX + dx
		if x < 0 || x >= W {
			continue
		}
		// Parabolic crater
		edgeDist := 1.0 - (float64(dx*dx) / float64(craterW*craterW))
		depth := int(float64(craterDepth) * edgeDist)
		for dy := 0; dy < depth; dy++ {
			y := volcanoTopY + dy
			if y >= 0 && y < H {
				if dy > depth-6 {
					w.Material[y*W+x] = MatLava // lava at bottom of crater
				} else {
					w.Material[y*W+x] = MatEmpty // air in crater
				}
			}
		}
	}
	// Lava tube running down from crater
	lavatubeY := volcanoTopY + craterDepth
	for dy := 0; dy < 60; dy++ {
		for dx := -3; dx <= 3; dx++ {
			x := volcanoX + dx
			y := lavatubeY + dy
			if x >= 0 && x < W && y >= 0 && y < H {
				w.Material[y*W+x] = MatLava
			}
		}
	}
	// Smoke rising above volcano
	for dy := 1; dy < 30; dy++ {
		spread := dy / 4
		for dx := -spread; dx <= spread; dx++ {
			x := volcanoX + dx + g.rng.Intn(3) - 1
			y := volcanoTopY - dy
			if x >= 0 && x < W && y >= 0 && y < H {
				if g.rng.Intn(100) < 50 {
					w.Material[y*W+x] = MatSmoke
				}
			}
		}
	}
	// Set high temperature around volcano
	for dy := -10; dy < craterDepth+60; dy++ {
		for dx := -20; dx <= 20; dx++ {
			x := volcanoX + dx
			y := volcanoTopY + dy
			if x >= 0 && x < W && y >= 0 && y < H {
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				temp := int16(2000.0 / (1.0 + dist*0.1))
				idx := y*W + x
				if temp > w.Temperature[idx] {
					w.Temperature[idx] = temp
				}
			}
		}
	}

	// ── WATERFALL: water source on cliff flowing into lake ───────────────────
	cliffStartX := int(0.72 * float64(W))
	// Find the cliff edge height
	cliffEdgeY := heights[cliffStartX]
	// Place water source at the top of the cliff, flowing left
	waterfallX := cliffStartX
	waterfallTopY := cliffEdgeY - 2
	// Water source pool at cliff top
	for dx := 0; dx < 20; dx++ {
		for dy := -3; dy < 2; dy++ {
			x := waterfallX + dx
			y := waterfallTopY + dy
			if x >= 0 && x < W && y >= 0 && y < H {
				w.Material[y*W+x] = MatWater
			}
		}
	}
	// Waterfall stream flowing down the cliff face
	fallHeight := 50
	for dy := 0; dy < fallHeight; dy++ {
		for dx := -2; dx <= 2; dx++ {
			x := waterfallX + dx
			y := waterfallTopY + 2 + dy
			if x >= 0 && x < W && y >= 0 && y < H && w.Material[y*W+x] == MatEmpty {
				w.Material[y*W+x] = MatWater
			}
		}
	}

	// ── LAKE: large water basin in the forest valley ────────────────────────
	lakeCenter := int(0.60 * float64(W))
	lakeWidth := int(0.12 * float64(W))
	lakeTopY := heights[lakeCenter] + 2
	lakeDepth := 30
	// Carve lake basin and fill with water
	for dx := -lakeWidth; dx <= lakeWidth; dx++ {
		x := lakeCenter + dx
		if x < 0 || x >= W {
			continue
		}
		edgeDist := float64(dx) / float64(lakeWidth)
		depth := int(float64(lakeDepth) * (1.0 - edgeDist*edgeDist))
		for dy := -5; dy < depth; dy++ {
			y := lakeTopY + dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatWater
			}
		}
		// Set moisture high near water
		for dy := -20; dy < depth+10; dy++ {
			y := lakeTopY + dy
			if y >= 0 && y < H {
				idx := y*W + x
				w.Moisture[idx] = 200
			}
		}
	}

	// ── FOREST: dense vegetation on fertile soil near the lake ───────────────
	forestLeft := int(0.48 * float64(W))
	forestRight := int(0.72 * float64(W))
	for x := forestLeft; x < forestRight; x++ {
		if x < 0 || x >= W {
			continue
		}
		surfY := heights[x]
		// Plant dense vegetation on soil surfaces
		for dy := -3; dy <= 1; dy++ {
			y := surfY + dy
			if y < 1 || y >= H {
				continue
			}
			below := w.Material[y*W+x]
			aboveIdx := (y - 1) * W + x
			if aboveIdx < 0 {
				continue
			}
			above := w.Material[aboveIdx]
			if below == MatSoil && above == MatEmpty {
				// Dense forest: 60% coverage
				if g.rng.Intn(100) < 60 {
					w.Material[aboveIdx] = MatPlant
					// Stack plants for tree canopy effect
					if y-2 >= 0 && g.rng.Intn(100) < 40 {
						w.Material[(y-2)*W+x] = MatPlant
					}
					if y-3 >= 0 && g.rng.Intn(100) < 20 {
						w.Material[(y-3)*W+x] = MatPlant
					}
				}
			}
		}
		// Set moisture for forest area
		for dy := -5; dy < 15; dy++ {
			y := surfY + dy
			if y >= 0 && y < H {
				idx := y*W + x
				if w.Moisture[idx] < 150 {
					w.Moisture[idx] = 150
				}
			}
		}
	}

	// ── GENERAL SURFACE VEGETATION ──────────────────────────────────────────
	// The forest pass above only covers roughly a quarter of the map width, and
	// single-cell growth is invisible when the whole world is on screen. This
	// pass spreads vegetation across every fertile surface and grows it upward
	// into trees, so the landscape reads as living rather than as bare bedrock.
	for x := range W {
		frac := float64(x) / float64(W)

		// Locate the topmost solid surface cell near the recorded height.
		groundY := -1
		var ground uint8
		for dy := -3; dy <= 3; dy++ {
			y := heights[x] + dy
			if y < 3 || y >= H {
				continue
			}
			m := w.Material[y*W+x]
			if (m == MatSoil || m == MatSand) && w.Material[(y-1)*W+x] == MatEmpty {
				groundY, ground = y, m
				break
			}
		}
		if groundY < 0 {
			continue
		}

		// Biome determines how likely a tree is and how tall it grows.
		chance, minH, maxH := 0, 0, 0
		switch ground {
		case MatSoil:
			chance, minH, maxH = 48, 5, 14
		case MatSand:
			chance, minH, maxH = 7, 2, 4 // sparse desert scrub
		default:
			continue
		}
		if frac >= 0.30 && frac < 0.44 {
			chance /= 4 // volcanic slopes stay mostly barren
		}
		if g.rng.Intn(100) >= chance {
			continue
		}

		height := minH + g.rng.Intn(maxH-minH+1)

		// Vegetation only persists where its roots find water: the simulation
		// withers plants whose soil is bone dry. Seed enough moisture beneath a
		// new tree for it to survive, rather than spawning growth that dies in
		// the first few seconds.
		for d := 0; d < 6; d++ {
			ry := groundY + d
			if ry >= H {
				break
			}
			ri := ry*W + x
			if (w.Material[ri] == MatSoil || w.Material[ri] == MatSand) && w.Moisture[ri] < 90 {
				w.Moisture[ri] = 90
			}
		}

		// Trunk.
		topY := groundY - 1
		for i := 0; i < height; i++ {
			y := groundY - 1 - i
			if y < 2 || w.Material[y*W+x] != MatEmpty {
				break
			}
			w.Material[y*W+x] = MatPlant
			topY = y
		}

		// Canopy: a small crown around the top of the trunk. Skipped for scrub,
		// which should stay low and sparse.
		if height >= 5 {
			radius := 1 + g.rng.Intn(2)
			for cy := topY - radius; cy <= topY+1; cy++ {
				for cx := x - radius; cx <= x+radius; cx++ {
					if cx < 0 || cx >= W || cy < 2 || cy >= H {
						continue
					}
					// Round the crown off rather than leaving a square block.
					dx := cx - x
					dy := cy - topY
					if dx*dx+dy*dy > (radius+1)*(radius+1) {
						continue
					}
					if w.Material[cy*W+cx] == MatEmpty && g.rng.Intn(100) < 72 {
						w.Material[cy*W+cx] = MatPlant
					}
				}
			}
		}
	}

	// ── DESERT: sand dunes with oasis ───────────────────────────────────────
	// Add extra sand shaping for dune appearance
	desertRight := int(0.23 * float64(W))
	for x := 10; x < desertRight; x++ {
		surfY := heights[x]
		// Add dune ridges
		fx := float64(x)
		duneHeight := int(math.Sin(fx*0.025)*5 + math.Sin(fx*0.06)*3)
		for dy := duneHeight; dy > 0; dy-- {
			y := surfY - dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatSand
			}
		}
	}
	// Oasis: small water pool with plants in the desert
	oasisX := int(0.12 * float64(W))
	oasisY := heights[oasisX] + 3
	for dx := -8; dx <= 8; dx++ {
		for dy := -2; dy <= 5; dy++ {
			x := oasisX + dx
			y := oasisY + dy
			if x >= 0 && x < W && y >= 0 && y < H {
				dist := float64(dx*dx+dy*dy) / 64.0
				if dist < 1.0 {
					w.Material[y*W+x] = MatWater
				}
			}
		}
	}
	// Plants around oasis
	for dx := -12; dx <= 12; dx++ {
		x := oasisX + dx
		if x < 0 || x >= W {
			continue
		}
		surfY := heights[x]
		for dy := -2; dy <= 0; dy++ {
			y := surfY + dy
			if y >= 1 && y < H {
				aboveIdx := (y - 1) * W + x
				if w.Material[y*W+x] == MatSand && w.Material[aboveIdx] == MatEmpty {
					if g.rng.Intn(100) < 30 {
						w.Material[aboveIdx] = MatPlant
					}
				}
			}
		}
	}

	// ── CAVES with interesting geology ──────────────────────────────────────
	// Main cave system under the volcano
	g.carveCave(volcanoX, H*60/100, 80, 14)
	g.carveCave(volcanoX-40, H*65/100, 50, 10)
	g.carveCave(volcanoX+30, H*70/100, 45, 12)
	// Cave under the forest (with exposed lava vein)
	g.carveCave(lakeCenter, H*75/100, 60, 12)
	// Desert caves
	g.carveCave(int(0.15*float64(W)), H*62/100, 35, 8)

	// Deep lava veins visible in caves
	for _, cave := range []struct{ x, y int }{
		{volcanoX, H * 60 / 100},
		{volcanoX - 40, H * 65 / 100},
		{lakeCenter, H * 75 / 100},
	} {
		// Place lava pools at bottom of caves
		for dx := -8; dx <= 8; dx++ {
			for dy := 2; dy < 6; dy++ {
				x := cave.x + dx + g.rng.Intn(5) - 2
				y := cave.y + dy
				if x >= 0 && x < W && y >= 0 && y < H {
					if w.Material[y*W+x] == MatEmpty {
						w.Material[y*W+x] = MatLava
					}
				}
			}
		}
	}

	// ── CREATURES: herbivores in forest, predators on rocky crags ────────────
	// Herbivores scattered through the forest
	for x := forestLeft; x < forestRight; x += 8 {
		if g.rng.Intn(100) > 30 {
			continue
		}
		surfY := heights[x]
		// Place herbivore on first available surface
		for y := surfY - 5; y < surfY+2; y++ {
			if y < 1 || y >= H {
				continue
			}
			below := w.Material[y*W+x]
			above := w.Material[(y-1)*W+x]
			if (below == MatSoil || below == MatPlant) && above == MatEmpty {
				w.Material[(y-1)*W+x] = MatHerbivore
				break
			}
		}
	}
	// Predators on volcanic rocky areas
	volcanoLeft := int(0.28 * float64(W))
	volcanoRight := int(0.42 * float64(W))
	for x := volcanoLeft; x < volcanoRight; x += 15 {
		if g.rng.Intn(100) > 25 {
			continue
		}
		surfY := heights[x]
		for y := surfY - 3; y < surfY+2; y++ {
			if y < 1 || y >= H {
				continue
			}
			below := w.Material[y*W+x]
			above := w.Material[(y-1)*W+x]
			if below == MatRock && above == MatEmpty {
				w.Material[(y-1)*W+x] = MatPredator
				break
			}
		}
	}

	// ── Set temperature gradients for biome feel ────────────────────────────
	for x := range W {
		frac := float64(x) / float64(W)
		for y := range H {
			idx := y*W + x
			// Desert is hot
			if frac < 0.25 {
				w.Temperature[idx] = 350 // 35°C
			} else if frac >= 0.45 && frac < 0.75 {
				// Forest is temperate
				w.Temperature[idx] = 200 // 20°C
			} else {
				w.Temperature[idx] = 150 // 15°C base
			}
			// Deeper = hotter
			depth := y - heights[x]
			if depth > 0 {
				w.Temperature[idx] += int16(depth / 2)
			}
		}
	}

	// ── Perched water ready to flow (immediate action on load) ──────────────
	// Small water pocket above the lake that will cascade down
	perchedX := lakeCenter - lakeWidth - 10
	perchedY := heights[perchedX] - 5
	for dx := 0; dx < 12; dx++ {
		for dy := 0; dy < 5; dy++ {
			x := perchedX + dx
			y := perchedY + dy
			if x >= 0 && x < W && y >= 0 && y < H {
				w.Material[y*W+x] = MatWater
			}
		}
	}

	// ── Sand cascade: sand pile above a gap (will fall immediately) ─────────
	cascadeX := int(0.20 * float64(W))
	cascadeY := heights[cascadeX] - 2
	for dx := 0; dx < 15; dx++ {
		x := cascadeX + dx
		if x >= W {
			break
		}
		// Thin ledge
		for dy := 0; dy < 2; dy++ {
			y := cascadeY + dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatSoil
			}
		}
		// Sand on top
		for dy := -6; dy < 0; dy++ {
			y := cascadeY + dy
			if y >= 0 && y < H {
				w.Material[y*W+x] = MatSand
			}
		}
		// Clear below
		for dy := 2; dy < 25; dy++ {
			y := cascadeY + dy
			if y >= 0 && y < H && w.Material[y*W+x] != MatRock {
				w.Material[y*W+x] = MatEmpty
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
		r := radius - g.rng.Intn(radius/3+1)
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
