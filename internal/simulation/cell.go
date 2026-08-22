package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// simulateCell dispatches per-material behavior for cell (x, y).
func simulateCell(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	if w.Flags[i]&world.FlagMoved != 0 {
		return // already processed this tick
	}

	switch w.Material[i] {
	case world.MatSand:
		simulateSand(w, x, y)
	case world.MatWater:
		simulateWater(w, x, y)
	case world.MatSoil:
		simulateSoil(w, x, y)
	case world.MatPlant:
		simulatePlant(w, x, y)
	case world.MatFire:
		simulateFire(w, x, y)
	case world.MatVapor:
		simulateVapor(w, x, y)
	case world.MatSmoke:
		simulateSmoke(w, x, y)
	case world.MatIce:
		simulateIce(w, x, y)
	case world.MatOil:
		simulateOil(w, x, y)
	case world.MatLava:
		simulateLava(w, x, y)
	case world.MatHerbivore:
		simulateHerbivore(w, x, y)
	case world.MatPredator:
		simulatePredator(w, x, y)
	case world.MatCloud:
		simulateCloud(w, x, y)
	}
}

// markMoved sets FlagMoved on cell (x, y) so it isn't processed again this tick.
func markMoved(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i >= 0 {
		w.Flags[i] |= world.FlagMoved
	}
}
