package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Lava behavior:
//   - Flows like a viscous liquid (very slow lateral spread)
//   - Ignites all flammable materials on contact (plant, oil, wood)
//   - Touching water → creates rock + steam
//   - Cools to rock over time (300-500 ticks)
//   - Emits extreme heat to surrounding cells
//   - Very high density — sinks below water/oil

const (
	lavaFlowChance     = 3     // 1-in-N chance to move laterally per tick (viscous)
	lavaCoolLifetime   = 400   // base ticks before cooling to rock
	lavaHeatOutput     = 500   // temperature units emitted to neighbors per tick
	lavaIgniteChance   = 2     // 1-in-N chance to ignite adjacent flammable per tick
)

func simulateLava(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Initialize lifetime if not set
	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = uint16(lavaCoolLifetime + w.RNG().Intn(100))
	}

	// Cool down over time
	w.Lifetime[i]--
	if w.Lifetime[i] == 0 {
		w.SetMaterial(x, y, world.MatRock)
		w.Temperature[i] = 300 // newly formed rock is still warm
		return
	}

	// Emit extreme heat to surroundings
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		nx, ny := x+d[0], y+d[1]
		j := w.Index(nx, ny)
		if j < 0 {
			continue
		}

		mat := w.Material[j]

		// Lava + Water → Rock + Steam
		if mat == world.MatWater {
			w.SetMaterial(nx, ny, world.MatVapor)
			w.Lifetime[j] = uint16(60 + w.RNG().Intn(60))
			// The lava itself becomes rock from this interaction
			if w.RNG().Intn(4) == 0 {
				w.SetMaterial(x, y, world.MatRock)
				w.Temperature[i] = 200
				return
			}
			continue
		}

		// Ignite flammable materials on contact
		if world.IsFlammable(mat) {
			if w.RNG().Intn(lavaIgniteChance) == 0 {
				w.SetMaterial(nx, ny, world.MatFire)
				w.Lifetime[j] = 0
			}
		}

		// Heat transfer
		if w.Temperature[j] < 32767-lavaHeatOutput {
			w.Temperature[j] += lavaHeatOutput
		} else {
			w.Temperature[j] = 32767
		}
	}

	// Flow downward (heavy, always falls)
	below := w.GetMaterial(x, y+1)
	if below == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}
	// Lava displaces lighter liquids (water, oil)
	if below == world.MatWater || below == world.MatOil {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Diagonal fall
	leftFirst := (x+int(w.Tick))%2 == 0
	if leftFirst {
		if tryDiagLava(w, x, y, -1) {
			return
		}
		if tryDiagLava(w, x, y, 1) {
			return
		}
	} else {
		if tryDiagLava(w, x, y, 1) {
			return
		}
		if tryDiagLava(w, x, y, -1) {
			return
		}
	}

	// Slow lateral spread (viscous)
	if w.RNG().Intn(lavaFlowChance) == 0 {
		if leftFirst {
			if !tryLateralLava(w, x, y, -1) {
				tryLateralLava(w, x, y, 1)
			}
		} else {
			if !tryLateralLava(w, x, y, 1) {
				tryLateralLava(w, x, y, -1)
			}
		}
	}
}

func tryDiagLava(w *world.World, x, y, dx int) bool {
	m := w.GetMaterial(x+dx, y+1)
	if m == world.MatEmpty || m == world.MatWater || m == world.MatOil {
		w.Swap(x, y, x+dx, y+1)
		markMoved(w, x+dx, y+1)
		return true
	}
	return false
}

func tryLateralLava(w *world.World, x, y, dx int) bool {
	m := w.GetMaterial(x+dx, y)
	if m == world.MatEmpty || m == world.MatWater || m == world.MatOil {
		w.Swap(x, y, x+dx, y)
		markMoved(w, x+dx, y)
		return true
	}
	return false
}
