package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Ice behavior:
//   - Melts to water when temperature exceeds meltThreshold
//   - Slowly freezes adjacent water cells (cascade freezing)
//   - Does not move (solid)

const (
	iceMeltThreshold int16 = 200  // 20.0 °C — ice melts above this
	iceFreezeChance        = 300  // 1-in-N chance per tick per adjacent water cell
	iceMinFreezeTemp int16 = -100 // below this, freeze chance increases
)

func simulateIce(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Melt if temperature is high enough
	if w.Temperature[i] > iceMeltThreshold {
		w.SetMaterial(x, y, world.MatWater)
		return
	}

	// Check adjacent fire — melt immediately
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatFire || w.GetMaterial(nx, ny) == world.MatLava {
			w.SetMaterial(x, y, world.MatWater)
			return
		}
	}

	// Freeze adjacent water (slow cascade)
	freezeChance := iceFreezeChance
	if w.Temperature[i] < iceMinFreezeTemp {
		freezeChance = iceFreezeChance / 3 // faster freeze when very cold
	}

	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatWater {
			if w.RNG().Intn(freezeChance) == 0 {
				w.SetMaterial(nx, ny, world.MatIce)
				// Set the new ice cell to a cold temperature
				j := w.Index(nx, ny)
				if j >= 0 {
					w.Temperature[j] = -50
				}
			}
		}
	}
}
