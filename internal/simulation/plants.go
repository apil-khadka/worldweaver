package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Plant growth requires:
//   - adjacent soil with moisture >= threshold
//   - temperature below maximum
//   - available empty neighboring cell
//   - random growth chance (slow spread)

const (
	plantMoistureMin  = 30
	plantTempMax      = 800 // 80.0 °C
	plantGrowthChance = 200 // 1-in-200 chance per tick
)

func simulatePlant(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Check fire adjacency — handled in fire.go, not here

	// Only grow if temperature is acceptable
	if w.Temperature[i] > plantTempMax {
		return
	}

	// Attempt to spread to an adjacent empty cell on soil
	if w.RNG().Intn(plantGrowthChance) != 0 {
		return
	}

	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}}
	for _, d := range dirs {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatEmpty &&
			w.GetMaterial(nx, ny+1) == world.MatSoil &&
			w.GetMoisture(nx, ny+1) >= plantMoistureMin {
			w.SetMaterial(nx, ny, world.MatPlant)
			return
		}
	}
}
