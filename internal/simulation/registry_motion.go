package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// simulateRegisteredElement gives registry-backed elements the same visible
// movement rules as the legacy materials. The registry already describes phase
// and density; keeping the motion here means a newly registered element falls,
// flows, or rises without another material-ID switch.
func simulateRegisteredElement(w *world.World, x, y, i int, e *world.Element) {
	switch e.Phase {
	case world.PhasePowder:
		simulateRegisteredPowder(w, x, y, e.ID)
	case world.PhaseLiquid:
		simulateRegisteredLiquid(w, x, y, e.ID)
	case world.PhaseGas:
		simulateRegisteredGas(w, x, y, e.ID)
	case world.PhaseEnergy:
		// Energy elements are transient effects; they do not obey gravity.
	case world.PhaseRigid, world.PhaseLife:
		// Rigid and life elements are stationary here. Creature-specific movement
		// remains owned by the legacy trait system until those IDs are migrated.
	}
	_ = i
}

func simulateRegisteredPowder(w *world.World, x, y int, material uint8) {
	if tryRegisteredMove(w, x, y, x, y+1, material) {
		return
	}

	leftFirst := (x+int(w.Tick.Load()))%2 == 0
	if leftFirst {
		if tryRegisteredMove(w, x, y, x-1, y+1, material) {
			return
		}
		tryRegisteredMove(w, x, y, x+1, y+1, material)
		return
	}
	if tryRegisteredMove(w, x, y, x+1, y+1, material) {
		return
	}
	tryRegisteredMove(w, x, y, x-1, y+1, material)
}

func simulateRegisteredLiquid(w *world.World, x, y int, material uint8) {
	if tryRegisteredMove(w, x, y, x, y+1, material) {
		return
	}

	leftFirst := (x+int(w.Tick.Load()))%2 == 0
	diags := [][2]int{{-1, 1}, {1, 1}}
	if !leftFirst {
		diags[0], diags[1] = diags[1], diags[0]
	}
	for _, d := range diags {
		if tryRegisteredMove(w, x, y, x+d[0], y+d[1], material) {
			return
		}
	}

	// A short lateral spread makes liquids visibly settle instead of looking
	// frozen when the cell below is occupied.
	lateral := [][2]int{{-1, 0}, {1, 0}}
	if !leftFirst {
		lateral[0], lateral[1] = lateral[1], lateral[0]
	}
	for _, d := range lateral {
		if tryRegisteredMove(w, x, y, x+d[0], y, material) {
			return
		}
	}
}

func simulateRegisteredGas(w *world.World, x, y int, material uint8) {
	if tryRegisteredMove(w, x, y, x, y-1, material) {
		return
	}

	leftFirst := (x+int(w.Tick.Load()))%2 == 0
	diags := [][2]int{{-1, -1}, {1, -1}}
	if !leftFirst {
		diags[0], diags[1] = diags[1], diags[0]
	}
	for _, d := range diags {
		if tryRegisteredMove(w, x, y, x+d[0], y+d[1], material) {
			return
		}
	}

	for _, dx := range []int{-1, 1} {
		if tryRegisteredMove(w, x, y, x+dx, y, material) {
			return
		}
	}
}

// tryRegisteredMove moves into empty space or displaces a less-dense mobile
// material. Rigid terrain remains a hard boundary, so powders and liquids do not
// tunnel through rock just because their density is higher.
func tryRegisteredMove(w *world.World, x, y, nx, ny int, material uint8) bool {
	if w.Index(nx, ny) < 0 {
		return false
	}
	target := w.GetMaterial(nx, ny)
	if target != world.MatEmpty {
		phase := world.PhaseOf(target)
		if phase != world.PhasePowder && phase != world.PhaseLiquid && phase != world.PhaseGas {
			return false
		}
		if world.DensityOf(material) <= world.DensityOf(target) {
			return false
		}
	}

	w.Swap(x, y, nx, ny)
	// Travel with the moved cell: a freshly-painted element must stay loose
	// wherever gravity carries it, or it would freeze on its first step.
	if i := w.Index(nx, ny); i >= 0 {
		w.Flags[i] |= world.FlagMobile
	}
	markMoved(w, nx, ny)
	return true
}
