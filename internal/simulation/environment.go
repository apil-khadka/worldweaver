package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// environment.go handles world-level environmental processes:
//   - global temperature decay (heat dissipates over time)
//   - water evaporation at high temperature
//   - water freezing at very low temperature
//   - lava keeping itself hot

const (
	tempDecayRate     int16 = 2    // temperature units lost per tick
	evapTempThreshold int16 = 1000 // 100.0 °C — water becomes vapor above this
	freezeThreshold   int16 = -80  // water freezes below this temperature
)

// updateEnvironmentChunked runs the environment pass only on non-sleeping chunks.
func updateEnvironmentChunked(w *world.World) {
	chunkSize := w.ChunkSize

	for cy := range w.ChunkH {
		for cx := range w.ChunkW {
			idx := cy*w.ChunkW + cx
			if w.Chunks[idx].Sleeping {
				continue
			}

			startX := cx * chunkSize
			startY := cy * chunkSize
			endX := startX + chunkSize
			endY := startY + chunkSize
			if endX > w.Width {
				endX = w.Width
			}
			if endY > w.Height {
				endY = w.Height
			}

			for y := startY; y < endY; y++ {
				for x := startX; x < endX; x++ {
					i := y*w.Width + x

					// Temperature decay toward ambient (0)
					if w.Temperature[i] > 0 {
						w.Temperature[i] -= tempDecayRate
						if w.Temperature[i] < 0 {
							w.Temperature[i] = 0
						}
					} else if w.Temperature[i] < 0 {
						// Cold temperatures also decay toward 0 (warm up)
						w.Temperature[i] += tempDecayRate
						if w.Temperature[i] > 0 {
							w.Temperature[i] = 0
						}
					}

					mat := w.Material[i]

					// Water evaporation at very high temperature
					if mat == world.MatWater && w.Temperature[i] > evapTempThreshold {
						if w.RNG().Intn(50) == 0 {
							w.SetMaterial(x, y, world.MatVapor)
							w.Lifetime[i] = uint16(60 + w.RNG().Intn(60))
						}
					}

					// Water freezing at very low temperature
					if mat == world.MatWater && w.Temperature[i] < freezeThreshold {
						if w.RNG().Intn(80) == 0 {
							w.SetMaterial(x, y, world.MatIce)
						}
					}

					// Lava maintains high temperature
					if mat == world.MatLava {
						if w.Temperature[i] < 2000 {
							w.Temperature[i] = 2000
						}
					}

					// Ice stays cold (prevents immediate melting from ambient decay)
					if mat == world.MatIce {
						if w.Temperature[i] > -20 {
							w.Temperature[i] = -20
						}
					}
				}
			}
		}
	}
}

// updateEnvironment is the legacy full-world pass (kept for reference/testing).
func updateEnvironment(w *world.World) {
	for y := range w.Height {
		for x := range w.Width {
			i := w.Index(x, y)
			if i < 0 {
				continue
			}

			if w.Temperature[i] > 0 {
				w.Temperature[i] -= tempDecayRate
				if w.Temperature[i] < 0 {
					w.Temperature[i] = 0
				}
			}

			if w.Material[i] == world.MatWater && w.Temperature[i] > evapTempThreshold {
				if w.RNG().Intn(50) == 0 {
					w.SetMaterial(x, y, world.MatVapor)
					if li := w.Index(x, y); li >= 0 {
						w.Lifetime[li] = uint16(60 + w.RNG().Intn(60))
					}
				}
			}
		}
	}
}
