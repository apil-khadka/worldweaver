package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Hydraulic erosion parameters — kept LOW so terrain evolves
// visibly over 30-60 seconds at 60 TPS, not per-tick.
const (
	// Erosion probabilities (1 in N chance per tick when water is flowing).
	erosionSoilToSand = 200 // ~0.005 per tick: flowing water converts soil → sand
	erosionSandToEmpty = 500 // ~0.002 per tick: flowing water washes sand away

	// Sediment deposition probability (1 in N when water is still over empty).
	depositionChance = 333 // ~0.003 per tick

	// Moisture threshold indicating recent erosion activity nearby.
	// We use the Moisture field on water cells as an "erosion activity" counter.
	erosionActivityThreshold uint8 = 50
)

// applyFlowErosion is called when water at (wx, wy) has just moved laterally
// or diagonally (i.e., it is flowing). It erodes the material directly below
// the water's NEW position.
func applyFlowErosion(w *world.World, wx, wy int) {
	belowMat := w.GetMaterial(wx, wy+1)

	switch belowMat {
	case world.MatSoil:
		// Flowing water over soil: chance to erode soil → sand
		if w.RNG().Intn(erosionSoilToSand) == 0 {
			w.SetMaterial(wx, wy+1, world.MatSand)
			markErosionActivity(w, wx, wy)
		}
	case world.MatSand:
		// Flowing water over sand: chance to wash sand away entirely
		if w.RNG().Intn(erosionSandToEmpty) == 0 {
			w.SetMaterial(wx, wy+1, world.MatEmpty)
			markErosionActivity(w, wx, wy)
			// Wake the chunk below since material was removed
			w.WakeChunkAt(wx, wy+1)
		}
	// MatRock is bedrock — never eroded
	}
}

// applyStillDeposition is called when water at (wx, wy) did NOT move this tick.
// If there's empty space below, and there's been recent erosion activity,
// deposit sediment (sand).
func applyStillDeposition(w *world.World, wx, wy int) {
	// Only deposit if the cell below is empty (water sitting over a gap)
	if w.GetMaterial(wx, wy+1) != world.MatEmpty {
		return
	}

	// Check for recent erosion activity via moisture on the water cell
	i := w.Index(wx, wy)
	if i < 0 {
		return
	}
	if w.Moisture[i] < erosionActivityThreshold {
		return
	}

	// Probabilistic deposition
	if w.RNG().Intn(depositionChance) == 0 {
		w.SetMaterial(wx, wy+1, world.MatSand)
		w.WakeChunkAt(wx, wy+1)
		// Reduce erosion activity counter after depositing
		if w.Moisture[i] > 30 {
			w.Moisture[i] -= 30
		} else {
			w.Moisture[i] = 0
		}
	}
}

// markErosionActivity marks the water cell and nearby water cells as having
// recent erosion activity by boosting their Moisture field. This enables
// sediment deposition downstream.
func markErosionActivity(w *world.World, wx, wy int) {
	// Mark the water cell itself
	i := w.Index(wx, wy)
	if i >= 0 && w.Moisture[i] < 200 {
		w.Moisture[i] += 50
	}

	// Spread activity marker to adjacent water cells (simulates sediment
	// being carried downstream)
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		nx, ny := wx+d[0], wy+d[1]
		if w.GetMaterial(nx, ny) == world.MatWater {
			j := w.Index(nx, ny)
			if j >= 0 && w.Moisture[j] < 200 {
				w.Moisture[j] += 25
			}
		}
	}
}

// decayErosionActivity slowly reduces the erosion activity counter on water cells,
// so deposition only happens near recent erosion events.
func decayErosionActivity(w *world.World, wx, wy int) {
	i := w.Index(wx, wy)
	if i < 0 {
		return
	}
	if w.Moisture[i] > 0 {
		w.Moisture[i]--
	}
}
