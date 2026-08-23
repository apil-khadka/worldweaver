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

	if element, ok := world.Lookup(w.Material[i]); ok {
		// Only freshly painted registry elements are loose. Reaction-born and
		// generated registry cells are considered settled, so chemistry scenes
		// keep their geometry instead of gases drifting away mid-reaction.
		if w.Flags[i]&world.FlagMobile != 0 {
			simulateRegisteredElement(w, x, y, i, element)
		}
		return
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
	case world.MatGrass:
		simulateGrass(w, x, y)
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
	case world.MatHerbivore, world.MatPredator, world.MatSheep:
		// One routine drives every species; behaviour comes from its Trait entry.
		if t, ok := Traits[w.Material[i]]; ok {
			simulateCreature(w, x, y, t)
		}
	case world.MatCloud:
		simulateCloud(w, x, y)
	case world.MatVoid:
		simulateVoid(w, x, y)
	case world.MatRadiation:
		simulateRadiation(w, x, y)
	case world.MatPlasma:
		simulatePlasma(w, x, y)
	case world.MatCarrion:
		simulateCarrion(w, x, y)
	}
}

// markMoved sets FlagMoved on cell (x, y) so it isn't processed again this tick.
func markMoved(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i >= 0 {
		w.Flags[i] |= world.FlagMoved
	}
}
