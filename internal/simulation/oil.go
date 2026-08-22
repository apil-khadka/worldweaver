package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Oil behavior:
//   - Liquid that falls and spreads laterally (like water but slower)
//   - Floats on water (density 60 < water density 80)
//   - Highly flammable — catches fire at lower temperature than plant
//   - When on fire, burns intensely producing extra smoke

const (
	oilIgnitionTemp int16 = 400  // 40.0 °C — much lower than plant
	oilSpread             = 3    // lateral spread distance (less than water)
	oilFireChance         = 5    // 1-in-N when adjacent to fire (very easy to ignite)
)

func simulateOil(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Check for ignition — adjacent fire or high temperature
	if w.Temperature[i] > oilIgnitionTemp {
		if w.RNG().Intn(10) == 0 {
			w.SetMaterial(x, y, world.MatFire)
			w.Lifetime[i] = uint16(80 + w.RNG().Intn(60)) // burns longer than wood
			return
		}
	}

	// Adjacent fire ignites oil very easily
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		mat := w.GetMaterial(nx, ny)
		if mat == world.MatFire || mat == world.MatLava {
			if w.RNG().Intn(oilFireChance) == 0 {
				w.SetMaterial(x, y, world.MatFire)
				w.Lifetime[i] = uint16(80 + w.RNG().Intn(60))
				return
			}
		}
	}

	// Fall down — but displace water (oil floats on water)
	below := w.GetMaterial(x, y+1)
	if below == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}
	// Oil floats on water: if water is above oil, swap them
	// (handled from oil's perspective when checking below)
	if below == world.MatWater {
		// Oil is lighter — it stays, water goes down (do nothing, water sim handles this)
		// Actually: if oil is falling onto water, it should float (not pass through)
		// Oil stays in place — correct behavior
	} else if below == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Diagonal fall
	leftFirst := (x+int(w.Tick))%2 == 0
	if leftFirst {
		if tryDiagOil(w, x, y, -1) {
			return
		}
		if tryDiagOil(w, x, y, 1) {
			return
		}
	} else {
		if tryDiagOil(w, x, y, 1) {
			return
		}
		if tryDiagOil(w, x, y, -1) {
			return
		}
	}

	// Lateral spread (slower than water)
	if leftFirst {
		for dx := -1; dx >= -oilSpread; dx-- {
			if tryLateralOil(w, x, y, dx) {
				return
			}
		}
		for dx := 1; dx <= oilSpread; dx++ {
			if tryLateralOil(w, x, y, dx) {
				return
			}
		}
	} else {
		for dx := 1; dx <= oilSpread; dx++ {
			if tryLateralOil(w, x, y, dx) {
				return
			}
		}
		for dx := -1; dx >= -oilSpread; dx-- {
			if tryLateralOil(w, x, y, dx) {
				return
			}
		}
	}
}

func tryDiagOil(w *world.World, x, y, dx int) bool {
	m := w.GetMaterial(x+dx, y+1)
	if m == world.MatEmpty {
		w.Swap(x, y, x+dx, y+1)
		markMoved(w, x+dx, y+1)
		return true
	}
	return false
}

func tryLateralOil(w *world.World, x, y, dx int) bool {
	m := w.GetMaterial(x+dx, y)
	if m == world.MatEmpty {
		w.Swap(x, y, x+dx, y)
		markMoved(w, x+dx, y)
		return true
	}
	return false
}
