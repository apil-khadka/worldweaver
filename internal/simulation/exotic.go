package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Exotic materials: void, radiation and plasma.
//
// Each is deliberately self-limiting. An unbounded consumer or spreader would
// eventually claim the whole world, which is the same failure the weather cycle
// had when precipitation created water without consuming the cloud.

const (
	// A void consumes one cell at a time and spends lifetime doing so, so the
	// size of the hole it can eat is bounded by the lifetime it was created with.
	voidInitialLifetime = 220
	voidFeedCost        = 6  // lifetime spent per cell consumed
	voidIdleCost        = 1  // lifetime spent when there is nothing to eat
	voidCollapseChance  = 12 // 1-in-N to collapse inward once spent

	// Radiation drifts, decays, and only occasionally replicates, so a source
	// produces a spreading but finite cloud.
	radiationInitialLifetime = 160
	radiationSpreadChance    = 7 // 1-in-N per tick to seed a neighbour
	radiationMutateChance    = 9 // 1-in-N to mutate a plant it touches
	radiationDecayPerTick    = 1

	// Radiation lingers rather than venting upward. Rising as readily as smoke
	// meant a source left its surroundings within a few ticks and irradiated
	// nothing, which is the opposite of what makes it dangerous.
	radiationRiseChance = 8 // 1-in-N per tick

	// Plasma is violent and short-lived.
	plasmaInitialLifetime = 90
	plasmaMeltChance      = 4 // 1-in-N to melt adjacent rock into lava
	plasmaHeat            = 2400

	// Plasma also stays put long enough to do damage.
	plasmaRiseChance = 6 // 1-in-N per tick
)

// ── Void ────────────────────────────────────────────────────────────────────

// simulateVoid consumes a neighbouring cell, paying lifetime for the privilege.
//
// It cannot consume rock, which gives players something to contain it with, and
// it collapses once its lifetime is spent rather than persisting forever.
func simulateVoid(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = voidInitialLifetime
	}

	// Find something to eat among the four neighbours.
	eaten := false
	for _, d := range neighbourOrder(w, x, y) {
		nx, ny := x+d[0], y+d[1]
		m := w.GetMaterial(nx, ny)
		if m == world.MatEmpty || m == world.MatVoid {
			continue
		}
		// Rock resists the void, so it can be walled off.
		if m == world.MatRock {
			continue
		}
		w.SetMaterial(nx, ny, world.MatEmpty)
		w.Lifetime[w.Index(nx, ny)] = 0
		eaten = true
		break
	}

	cost := uint16(voidIdleCost)
	if eaten {
		cost = voidFeedCost
	}
	if w.Lifetime[i] <= cost {
		// Spent: collapse, leaving the hole it has already eaten.
		w.SetMaterial(x, y, world.MatEmpty)
		w.Lifetime[i] = 0
		return
	}
	w.Lifetime[i] -= cost

	// Keep the chunk awake. A transient material that stops marking its chunk
	// dirty stops being simulated, so it would freeze mid-life instead of
	// decaying — the void previously became a permanent fixture this way.
	w.MarkDirty(x, y)

	// Drift into open space. This has to happen whether or not it just ate:
	// once a void has hollowed out its four neighbours it is surrounded by its
	// own hole, and a void that only moves after eating would sit there forever.
	if w.RNG().Intn(voidCollapseChance) == 0 {
		for _, d := range neighbourOrder(w, x, y) {
			nx, ny := x+d[0], y+d[1]
			if w.Index(nx, ny) < 0 {
				continue
			}
			if w.GetMaterial(nx, ny) != world.MatEmpty {
				continue
			}
			life := w.Lifetime[i]
			w.SetMaterial(x, y, world.MatEmpty)
			w.Lifetime[i] = 0
			w.SetMaterial(nx, ny, world.MatVoid)
			w.Lifetime[w.Index(nx, ny)] = life
			markMoved(w, nx, ny)
			return
		}
	}
}

// ── Radiation ───────────────────────────────────────────────────────────────

// simulateRadiation drifts upward through open space, harms life it touches, and
// decays. Replication is rare and inherits a reduced lifetime, so a cloud
// spreads outward and then dies rather than growing without limit.
func simulateRadiation(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = radiationInitialLifetime
	}

	// Affect neighbours: kill creatures, mutate or wither plants.
	for _, d := range neighbourOrder(w, x, y) {
		nx, ny := x+d[0], y+d[1]
		switch w.GetMaterial(nx, ny) {
		case world.MatHerbivore, world.MatPredator:
			w.SetMaterial(nx, ny, world.MatCarrion)
			w.Lifetime[w.Index(nx, ny)] = carrionInitialLifetime
		case world.MatPlant:
			if w.RNG().Intn(radiationMutateChance) == 0 {
				w.SetMaterial(nx, ny, world.MatAsh)
			}
		}
	}

	// Occasionally seed an adjacent open cell with weaker radiation.
	if w.Lifetime[i] > radiationInitialLifetime/3 && w.RNG().Intn(radiationSpreadChance) == 0 {
		for _, d := range neighbourOrder(w, x, y) {
			nx, ny := x+d[0], y+d[1]
			if w.GetMaterial(nx, ny) == world.MatEmpty {
				w.SetMaterial(nx, ny, world.MatRadiation)
				w.Lifetime[w.Index(nx, ny)] = w.Lifetime[i] / 2
				break
			}
		}
	}

	// Drift upward like a gas. GetMaterial reports MatEmpty for out-of-bounds
	// coordinates, so the target row must be bounds-checked explicitly or the
	// top row would rise off the map and index -1.
	if above := w.Index(x, y-1); above >= 0 &&
		w.Material[above] == world.MatEmpty && w.RNG().Intn(radiationRiseChance) == 0 {
		w.Swap(x, y, x, y-1)
		markMoved(w, x, y-1)
		y--
		i = above
	}

	// Keep the chunk awake so this cell continues to decay.
	w.MarkDirty(x, y)

	if w.Lifetime[i] <= radiationDecayPerTick {
		w.SetMaterial(x, y, world.MatEmpty)
		w.Lifetime[i] = 0
		return
	}
	w.Lifetime[i] -= radiationDecayPerTick
}

// ── Plasma ──────────────────────────────────────────────────────────────────

// simulatePlasma burns fiercely, melts rock into lava, flashes water to vapour,
// and burns out quickly into fire.
func simulatePlasma(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = plasmaInitialLifetime
	}
	w.Temperature[i] = plasmaHeat

	for _, d := range neighbourOrder(w, x, y) {
		nx, ny := x+d[0], y+d[1]
		ni := w.Index(nx, ny)
		if ni < 0 {
			continue
		}
		switch w.GetMaterial(nx, ny) {
		case world.MatRock, world.MatSoil, world.MatSand:
			if w.RNG().Intn(plasmaMeltChance) == 0 {
				w.SetMaterial(nx, ny, world.MatLava)
				w.Temperature[ni] = 1200
			}
		case world.MatWater, world.MatIce:
			w.SetMaterial(nx, ny, world.MatVapor)
			w.Lifetime[ni] = 90
		case world.MatPlant, world.MatOil:
			w.SetMaterial(nx, ny, world.MatFire)
			w.Lifetime[ni] = 120
		case world.MatHerbivore, world.MatPredator:
			w.SetMaterial(nx, ny, world.MatAsh)
		default:
			// Heat everything else it touches.
			if w.Temperature[ni] < plasmaHeat/2 {
				w.Temperature[ni] += 400
			}
		}
	}

	// Rise, being lighter than air at this temperature. Bounds-checked for the
	// same reason as radiation: GetMaterial cannot distinguish empty from
	// off-map.
	if above := w.Index(x, y-1); above >= 0 &&
		w.Material[above] == world.MatEmpty && w.RNG().Intn(plasmaRiseChance) == 0 {
		w.Swap(x, y, x, y-1)
		markMoved(w, x, y-1)
		y--
		i = above
	}

	// Keep the chunk awake so this cell continues to burn down.
	w.MarkDirty(x, y)

	if w.Lifetime[i] <= 1 {
		// Burns down into ordinary fire rather than vanishing.
		w.SetMaterial(x, y, world.MatFire)
		w.Lifetime[i] = 60
		return
	}
	w.Lifetime[i]--
}

// ── Carrion ─────────────────────────────────────────────────────────────────

const carrionInitialLifetime = 400

// simulateCarrion decays a dead creature into soil nutrients.
//
// Returning biomass to the ground is what keeps the ecosystem's nutrient budget
// closed; deleting corpses outright would leak biomass out of the world.
func simulateCarrion(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	if w.Lifetime[i] == 0 {
		w.Lifetime[i] = carrionInitialLifetime
	}

	// Keep the chunk awake so decay continues to completion.
	w.MarkDirty(x, y)

	// Fall, so corpses come to rest on the ground.
	if below := w.Index(x, y+1); below >= 0 && w.Material[below] == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	if w.Lifetime[i] <= 1 {
		// Enrich the soil beneath before disappearing.
		if bi := w.Index(x, y+1); bi >= 0 {
			if m := w.Material[bi]; m == world.MatSoil || m == world.MatSand {
				if w.Moisture[bi] < 200 {
					w.Moisture[bi] += 55
				}
			}
		}
		w.SetMaterial(x, y, world.MatEmpty)
		w.Lifetime[i] = 0
		return
	}
	w.Lifetime[i]--
}

// neighbourOrder returns the four orthogonal offsets in a tick-dependent order,
// so repeated passes do not always favour the same direction.
func neighbourOrder(w *world.World, x, y int) [4][2]int {
	base := [4][2]int{{0, 1}, {0, -1}, {-1, 0}, {1, 0}}
	if (x+y+int(w.Tick))%2 == 0 {
		return [4][2]int{base[3], base[2], base[1], base[0]}
	}
	return base
}
