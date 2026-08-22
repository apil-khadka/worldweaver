package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// simulateSand implements falling-sand behavior:
//  1. Fall straight down if empty below
//  2. Slide diagonally down-left or down-right
//  3. Stay in place
func simulateSand(w *world.World, x, y int) {
	// Try directly below
	if w.GetMaterial(x, y+1) == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Alternate diagonal preference to avoid bias
	leftFirst := (x+int(w.Tick))%2 == 0

	if leftFirst {
		if w.GetMaterial(x-1, y+1) == world.MatEmpty {
			w.Swap(x, y, x-1, y+1)
			markMoved(w, x-1, y+1)
			return
		}
		if w.GetMaterial(x+1, y+1) == world.MatEmpty {
			w.Swap(x, y, x+1, y+1)
			markMoved(w, x+1, y+1)
		}
	} else {
		if w.GetMaterial(x+1, y+1) == world.MatEmpty {
			w.Swap(x, y, x+1, y+1)
			markMoved(w, x+1, y+1)
			return
		}
		if w.GetMaterial(x-1, y+1) == world.MatEmpty {
			w.Swap(x, y, x-1, y+1)
			markMoved(w, x-1, y+1)
		}
	}
}
