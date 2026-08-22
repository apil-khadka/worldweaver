package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// simulateSoil handles moisture decay under heat.
// Soil does not move on its own.
func simulateSoil(w *world.World, x, y int) {
	// Dry out under high temperature
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	if w.Temperature[i] > 500 { // > 50.0 °C
		if w.Moisture[i] > 0 {
			w.Moisture[i]--
		}
	}
}
