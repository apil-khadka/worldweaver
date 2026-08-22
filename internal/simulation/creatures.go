package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Creature ecosystem based on Lotka-Volterra probabilistic cellular automata.
//
// Energy is stored in the Temperature array (int16). This is safe because
// creature cells are excluded from the environment temperature decay pass.
//
// Herbivore behavior:
//   - Falls due to gravity (like sand)
//   - Moves randomly 1 cell in any direction
//   - Eats adjacent plants: plant -> empty, gains energy
//   - Reproduces (prob 0.3) when energy > 5 and adjacent empty cell exists
//   - Loses 1 energy every 30 ticks; dies (-> empty) at energy <= 0
//
// Predator behavior:
//   - Falls due to gravity
//   - Moves randomly 1 cell, biased toward nearest herbivore within 5 cells
//   - Eats adjacent herbivores: herbivore -> empty, gains energy
//   - Reproduces (prob 0.2) when energy > 8 and adjacent empty cell exists
//   - Loses 1 energy every 20 ticks; dies (-> empty) at energy <= 0

const (
	herbivoreInitialEnergy int16 = 3
	herbivoreEatGain       int16 = 3
	herbivoreReproEnergy   int16 = 5
	herbivoreReproProb     int   = 3  // 3/10 = 0.3
	herbivoreDecayInterval uint64 = 30

	predatorInitialEnergy int16 = 5
	predatorEatGain       int16 = 5
	predatorReproEnergy   int16 = 8
	predatorReproProb     int   = 2  // 2/10 = 0.2
	predatorDecayInterval uint64 = 20
	predatorSenseRange    int   = 5
)

// 4-directional offsets for movement (cardinal directions)
var cardinalDirs = [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// 8-directional offsets for adjacency checks
var eightDirs = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

// getEnergy reads creature energy from Temperature array.
func getEnergy(w *world.World, i int) int16 {
	return w.Temperature[i]
}

// setEnergy writes creature energy to Temperature array.
func setEnergy(w *world.World, i int, e int16) {
	w.Temperature[i] = e
}

// simulateHerbivore implements herbivore behavior.
func simulateHerbivore(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	energy := getEnergy(w, i)

	// Energy decay every N ticks
	if w.Tick%herbivoreDecayInterval == 0 {
		energy--
		setEnergy(w, i, energy)
	}

	// Death check
	if energy <= 0 {
		w.SetMaterial(x, y, world.MatEmpty)
		setEnergy(w, i, 0)
		return
	}

	// Eat adjacent plants
	energy = creatureEat(w, x, y, i, energy, world.MatPlant, herbivoreEatGain)
	setEnergy(w, i, energy)

	// Reproduction
	if energy > herbivoreReproEnergy {
		if creatureReproduce(w, x, y, i, world.MatHerbivore, herbivoreInitialEnergy, herbivoreReproProb) {
			energy -= herbivoreInitialEnergy
			setEnergy(w, i, energy)
		}
	}

	// Gravity — fall if empty below
	if w.GetMaterial(x, y+1) == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Random movement in cardinal direction
	rng := w.RNG()
	dir := cardinalDirs[rng.Intn(4)]
	nx, ny := x+dir[0], y+dir[1]
	if w.GetMaterial(nx, ny) == world.MatEmpty {
		w.Swap(x, y, nx, ny)
		markMoved(w, nx, ny)
	}
}

// simulatePredator implements predator behavior.
func simulatePredator(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	energy := getEnergy(w, i)

	// Energy decay every N ticks
	if w.Tick%predatorDecayInterval == 0 {
		energy--
		setEnergy(w, i, energy)
	}

	// Death check
	if energy <= 0 {
		w.SetMaterial(x, y, world.MatEmpty)
		setEnergy(w, i, 0)
		return
	}

	// Eat adjacent herbivores
	energy = creatureEat(w, x, y, i, energy, world.MatHerbivore, predatorEatGain)
	setEnergy(w, i, energy)

	// Reproduction
	if energy > predatorReproEnergy {
		if creatureReproduce(w, x, y, i, world.MatPredator, predatorInitialEnergy, predatorReproProb) {
			energy -= predatorInitialEnergy
			setEnergy(w, i, energy)
		}
	}

	// Gravity — fall if empty below
	if w.GetMaterial(x, y+1) == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Movement: biased toward nearest herbivore within sense range
	dx, dy := findPrey(w, x, y, predatorSenseRange)
	rng := w.RNG()

	if dx != 0 || dy != 0 {
		// Bias: 70% chance to move toward prey, 30% random
		if rng.Intn(10) < 7 {
			// Normalize to single-step direction
			mx, my := sign(dx), sign(dy)
			// Prefer cardinal (pick one axis)
			if rng.Intn(2) == 0 && mx != 0 {
				my = 0
			} else if my != 0 {
				mx = 0
			}
			nx, ny := x+mx, y+my
			if w.GetMaterial(nx, ny) == world.MatEmpty {
				w.Swap(x, y, nx, ny)
				markMoved(w, nx, ny)
				return
			}
		}
	}

	// Fallback: random cardinal movement
	dir := cardinalDirs[rng.Intn(4)]
	nx, ny := x+dir[0], y+dir[1]
	if w.GetMaterial(nx, ny) == world.MatEmpty {
		w.Swap(x, y, nx, ny)
		markMoved(w, nx, ny)
	}
}

// creatureEat checks all 8 neighbors for prey material and consumes the first found.
func creatureEat(w *world.World, x, y, i int, energy int16, prey uint8, gain int16) int16 {
	for _, d := range eightDirs {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == prey {
			w.SetMaterial(nx, ny, world.MatEmpty)
			// Clear prey's energy
			if j := w.Index(nx, ny); j >= 0 {
				w.Temperature[j] = 0
			}
			energy += gain
			return energy // eat one per tick
		}
	}
	return energy
}

// creatureReproduce attempts to spawn a new creature in an adjacent empty cell.
// Returns true if reproduction succeeded.
func creatureReproduce(w *world.World, x, y, _ int, mat uint8, childEnergy int16, probTenths int) bool {
	rng := w.RNG()
	if rng.Intn(10) >= probTenths {
		return false
	}

	// Shuffle direction order to avoid directional bias
	perm := rng.Perm(8)
	for _, pi := range perm {
		d := eightDirs[pi]
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatEmpty {
			w.SetMaterial(nx, ny, mat)
			if j := w.Index(nx, ny); j >= 0 {
				setEnergy(w, j, childEnergy)
			}
			markMoved(w, nx, ny)
			return true
		}
	}
	return false
}

// findPrey scans a square area around (x,y) for the nearest herbivore.
// Returns the direction vector (dx, dy) to the closest one, or (0,0) if none found.
func findPrey(w *world.World, x, y, radius int) (int, int) {
	bestDist := radius*radius + 1
	bestDx, bestDy := 0, 0

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			dist := dx*dx + dy*dy
			if dist >= bestDist {
				continue
			}
			if w.GetMaterial(x+dx, y+dy) == world.MatHerbivore {
				bestDist = dist
				bestDx = dx
				bestDy = dy
			}
		}
	}
	return bestDx, bestDy
}

// sign returns -1, 0, or 1.
func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
