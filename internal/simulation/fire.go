package simulation

import "github.com/worldweaver/worldweaver/internal/world"

const (
	fireLifetimeMax  = 120 // ticks
	fireSpreadChance = 30  // 1-in-N chance to spread to adjacent flammable material per tick
	fireTempIncrease = 200 // temperature units added to adjacent cells
	oilSpreadChance  = 8   // 1-in-N for oil (much easier to ignite than plant)
)

func simulateFire(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Burn down lifetime
	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = uint16(fireLifetimeMax - w.RNG().Intn(40))
	}
	w.Lifetime[i]--

	if w.Lifetime[i] == 0 {
		// Fire dies — leave smoke or empty
		if w.RNG().Intn(3) == 0 {
			w.SetMaterial(x, y, world.MatSmoke)
			w.Lifetime[i] = uint16(40 + w.RNG().Intn(40))
		} else {
			w.SetMaterial(x, y, world.MatEmpty)
		}
		return
	}

	// Heat nearby cells and spread
	for _, d := range [][2]int{{0, -1}, {-1, 0}, {1, 0}, {0, 1}} {
		nx, ny := x+d[0], y+d[1]
		j := w.Index(nx, ny)
		if j < 0 {
			continue
		}
		t := w.Temperature[j]
		if t < 32767-fireTempIncrease {
			w.Temperature[j] = t + fireTempIncrease
		}

		mat := w.Material[j]

		// Spread to flammable neighbors with material-specific chances
		if world.IsFlammable(mat) {
			spreadChance := fireSpreadChance
			if mat == world.MatOil {
				spreadChance = oilSpreadChance // oil ignites much more easily
			}
			if w.RNG().Intn(spreadChance) == 0 {
				w.SetMaterial(nx, ny, world.MatFire)
				w.Lifetime[j] = 0 // will be initialized on first fire tick
			}
		}

		// Melt adjacent ice
		if mat == world.MatIce {
			if w.RNG().Intn(5) == 0 {
				w.SetMaterial(nx, ny, world.MatWater)
			}
		}
	}
}

// simulateVapor moves vapor upward and condenses back to water after 60-120 ticks.
func simulateVapor(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Initialize lifetime if not set (condensation timer)
	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = uint16(60 + w.RNG().Intn(60))
	}

	// Rise upward with slight horizontal drift
	drift := w.RNG().Intn(3) - 1 // -1, 0, 1
	if w.GetMaterial(x+drift, y-1) == world.MatEmpty {
		w.Swap(x, y, x+drift, y-1)
		markMoved(w, x+drift, y-1)
		// Update index after swap
		i = w.Index(x+drift, y-1)
		if i < 0 {
			return
		}
	} else if w.GetMaterial(x, y-1) == world.MatEmpty {
		w.Swap(x, y, x, y-1)
		markMoved(w, x, y-1)
		i = w.Index(x, y-1)
		if i < 0 {
			return
		}
	}

	// Condense back to water
	w.Lifetime[i]--
	if w.Lifetime[i] == 0 {
		// Condense — become water
		w.Material[i] = world.MatWater
		w.MarkDirty(i%w.Width, i/w.Width)
	}
}

// simulateSmoke moves smoke upward with drift and dissipates after 40-80 ticks.
func simulateSmoke(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Initialize lifetime if not set
	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = uint16(40 + w.RNG().Intn(40))
	}

	// Rise with horizontal drift (faster than vapor)
	drift := w.RNG().Intn(3) - 1
	// Try to rise 1 cell (smoke rises every tick)
	if w.GetMaterial(x+drift, y-1) == world.MatEmpty {
		w.Swap(x, y, x+drift, y-1)
		markMoved(w, x+drift, y-1)
		i = w.Index(x+drift, y-1)
		if i < 0 {
			return
		}
	} else if w.GetMaterial(x, y-1) == world.MatEmpty {
		w.Swap(x, y, x, y-1)
		markMoved(w, x, y-1)
		i = w.Index(x, y-1)
		if i < 0 {
			return
		}
	}

	// Dissipate to empty
	w.Lifetime[i]--
	if w.Lifetime[i] == 0 {
		w.Material[i] = world.MatEmpty
		w.MarkDirty(i%w.Width, i/w.Width)
	}
}
