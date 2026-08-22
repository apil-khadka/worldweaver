package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Sand physics with angle of repose (friction between grains).
//
// Based on probabilistic cellular automata research (arXiv:2008.06341):
// Sand doesn't always slide diagonally — it has internal friction that creates
// stable slopes. The probability of diagonal movement depends on how many
// neighboring sand cells support it (simulating grain-to-grain friction).
//
// Angle of repose: ~33° for dry sand means roughly 2:1 rise:run.
// In grid terms: sand should only slide diagonally with probability inversely
// related to the number of supporting neighbors.
const (
	// sandFriction is the base probability (1-in-N) that sand WON'T slide
	// diagonally even when space is available. Higher = more friction.
	sandFriction = 3

	// sandInertia: once sand starts moving (has velocity), friction is reduced.
	// We use FlagHot (repurposed) as a "recently moved" inertia marker.
	sandInertiaFlag = world.FlagHot
)

// simulateSand implements falling-sand behavior with angle-of-repose friction:
//  1. Fall straight down if empty below (gravity always wins)
//  2. Slide diagonally only if friction check passes
//  3. Friction increases when sand has neighbors on both sides (stable pile)
func simulateSand(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Try directly below — gravity always dominates
	if w.GetMaterial(x, y+1) == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		// Mark as having inertia (recently moved)
		j := w.Index(x, y+1)
		if j >= 0 {
			w.Flags[j] |= sandInertiaFlag
		}
		return
	}

	// Sink through water (sand is denser than water)
	if w.GetMaterial(x, y+1) == world.MatWater {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Diagonal slide with friction model.
	// Count lateral support: sand neighbors on the same row increase friction.
	leftSupport := isSandOrSolid(w, x-1, y)
	rightSupport := isSandOrSolid(w, x+1, y)

	// Calculate effective friction.
	// More lateral support = higher friction = less likely to slide.
	friction := sandFriction
	if leftSupport && rightSupport {
		friction *= 3 // wedged between two supports — very stable
	} else if leftSupport || rightSupport {
		friction *= 2 // one-sided support — moderate stability
	}

	// Recently-moved sand (has inertia) overcomes friction more easily
	hasInertia := w.Flags[i]&sandInertiaFlag != 0
	if hasInertia {
		friction = friction / 2
		if friction < 1 {
			friction = 1
		}
		w.Flags[i] &^= sandInertiaFlag // consume inertia
	}

	// Friction check: skip diagonal movement with probability (friction-1)/friction
	if w.RNG().Intn(friction) != 0 {
		return // friction holds — sand stays in place
	}

	// Alternate diagonal preference to avoid directional bias
	leftFirst := (x+int(w.Tick))%2 == 0

	if leftFirst {
		if trySandDiag(w, x, y, -1) {
			return
		}
		trySandDiag(w, x, y, 1)
	} else {
		if trySandDiag(w, x, y, 1) {
			return
		}
		trySandDiag(w, x, y, -1)
	}
}

// trySandDiag attempts to move sand diagonally.
func trySandDiag(w *world.World, x, y, dx int) bool {
	target := w.GetMaterial(x+dx, y+1)
	if target == world.MatEmpty {
		w.Swap(x, y, x+dx, y+1)
		markMoved(w, x+dx, y+1)
		return true
	}
	// Sand can also displace water diagonally
	if target == world.MatWater {
		w.Swap(x, y, x+dx, y+1)
		markMoved(w, x+dx, y+1)
		return true
	}
	return false
}

// isSandOrSolid returns true if the cell at (x,y) is sand or another solid material.
func isSandOrSolid(w *world.World, x, y int) bool {
	m := w.GetMaterial(x, y)
	return m == world.MatSand || m == world.MatRock || m == world.MatSoil
}
