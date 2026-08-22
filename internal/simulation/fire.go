package simulation

import "github.com/worldweaver/worldweaver/internal/world"

const (
	fireLifetimeMax = 120 // ticks
	fireSpreadChance = 30 // 1-in-N chance to spread to adjacent flammable material per tick
	fireTempIncrease = 200 // temperature units added to adjacent cells
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
			w.Lifetime[i] = uint16(30 + w.RNG().Intn(30))
		} else {
			w.SetMaterial(x, y, world.MatEmpty)
		}
		return
	}

	// Heat nearby cells
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
		// Spread to flammable neighbors
		if world.IsFlammable(w.Material[j]) && w.RNG().Intn(fireSpreadChance) == 0 {
			w.SetMaterial(nx, ny, world.MatFire)
			w.Lifetime[j] = 0 // will be initialized on first fire tick
		}
	}
}

// simulateVapor moves vapor upward.
func simulateVapor(w *world.World, x, y int) {
	if w.GetMaterial(x, y-1) == world.MatEmpty {
		w.Swap(x, y, x, y-1)
		markMoved(w, x, y-1)
	}
	i := w.Index(x, y)
	if i >= 0 && w.Lifetime[i] > 0 {
		w.Lifetime[i]--
		if w.Lifetime[i] == 0 {
			w.SetMaterial(x, y, world.MatEmpty)
		}
	}
}

// simulateSmoke moves smoke upward and decays.
func simulateSmoke(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	// Rise
	target := w.RNG().Intn(3) - 1 // -1, 0, or 1 horizontal drift
	if w.GetMaterial(x+target, y-1) == world.MatEmpty {
		w.Swap(x, y, x+target, y-1)
		markMoved(w, x+target, y-1)
	}
	// Decay
	if w.Lifetime[i] > 0 {
		w.Lifetime[i]--
	} else {
		w.SetMaterial(x, y, world.MatEmpty)
	}
}
