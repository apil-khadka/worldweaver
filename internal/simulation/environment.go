package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// environment.go handles world-level environmental processes:
//   - global temperature decay (heat dissipates over time)
//   - water evaporation at high temperature (stretch)

const (
	tempDecayRate    int16 = 2   // temperature units lost per tick
	evapTempThreshold int16 = 1000 // 100.0 °C — water becomes vapor above this
)

// updateEnvironment is called once per tick after all cell simulations.
func updateEnvironment(w *world.World) {
	for y := range w.Height {
		for x := range w.Width {
			i := w.Index(x, y)
			if i < 0 {
				continue
			}

			// Temperature decay toward ambient (0)
			if w.Temperature[i] > 0 {
				w.Temperature[i] -= tempDecayRate
				if w.Temperature[i] < 0 {
					w.Temperature[i] = 0
				}
			}

			// Water evaporation at very high temperature (stretch feature)
			if w.Material[i] == world.MatWater && w.Temperature[i] > evapTempThreshold {
				if w.RNG().Intn(50) == 0 {
					w.SetMaterial(x, y, world.MatVapor)
					li := w.Index(x, y)
					if li >= 0 {
						w.Lifetime[li] = uint16(60 + w.RNG().Intn(60))
					}
				}
			}
		}
	}
}
