package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// simulateWater implements liquid behavior:
//  1. Fall down if empty below
//  2. Diagonal fall
//  3. Lateral spread (up to 4 cells per tick)
//  4. Wet adjacent soil
//  5. Extinguish adjacent fire
func simulateWater(w *world.World, x, y int) {
	// Fall down
	below := w.GetMaterial(x, y+1)
	if below == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		wetSoilAround(w, x, y+1)
		return
	}

	// Extinguish fire below
	if below == world.MatFire {
		w.SetMaterial(x, y+1, world.MatEmpty)
		w.SetMaterial(x, y, world.MatEmpty)
		return
	}

	// Diagonal fall
	leftFirst := (x+int(w.Tick))%2 == 0
	if leftFirst {
		if tryDiagWater(w, x, y, -1) {
			return
		}
		if tryDiagWater(w, x, y, 1) {
			return
		}
	} else {
		if tryDiagWater(w, x, y, 1) {
			return
		}
		if tryDiagWater(w, x, y, -1) {
			return
		}
	}

	// Lateral spread
	spread := 4
	if leftFirst {
		for dx := -1; dx >= -spread; dx-- {
			if !tryLateralWater(w, x, y, dx) {
				break
			}
			return
		}
		for dx := 1; dx <= spread; dx++ {
			if !tryLateralWater(w, x, y, dx) {
				break
			}
			return
		}
	} else {
		for dx := 1; dx <= spread; dx++ {
			if !tryLateralWater(w, x, y, dx) {
				break
			}
			return
		}
		for dx := -1; dx >= -spread; dx-- {
			if !tryLateralWater(w, x, y, dx) {
				break
			}
			return
		}
	}

	wetSoilAround(w, x, y)
}

func tryDiagWater(w *world.World, x, y, dx int) bool {
	if w.GetMaterial(x+dx, y+1) == world.MatEmpty {
		w.Swap(x, y, x+dx, y+1)
		markMoved(w, x+dx, y+1)
		wetSoilAround(w, x+dx, y+1)
		return true
	}
	return false
}

func tryLateralWater(w *world.World, x, y, dx int) bool {
	if w.GetMaterial(x+dx, y) == world.MatEmpty {
		w.Swap(x, y, x+dx, y)
		markMoved(w, x+dx, y)
		wetSoilAround(w, x+dx, y)
		return true
	}
	return false
}

// wetSoilAround increases moisture of adjacent soil cells.
func wetSoilAround(w *world.World, x, y int) {
	for _, d := range [][2]int{{0, 1}, {0, -1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatSoil {
			m := w.GetMoisture(nx, ny)
			if m < 230 {
				w.SetMoisture(nx, ny, m+20)
			}
		}
	}
}
