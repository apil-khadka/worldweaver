// The reaction pass — where the declarative table becomes behaviour.
//
// # Cost control
//
// This pass visits cells, not reactions. For each cell it reads the element's
// reactivity and returns immediately if it is zero, which excludes empty, rock,
// sand, soil and glass — between them the overwhelming majority of a real world.
// Only for a reactive cell does it walk the four orthogonal neighbours, and each
// neighbour costs one indexed array read into the [256][256] reaction index.
//
// The alternative, scanning the 65-row table per neighbour, is 260 comparisons per
// cell per tick and does not fit the 16 ms budget in tech.md at 2M cells.
//
// # Heat cascade
//
// An exothermic reaction writes its HeatDelta into the cells around it. That is the
// entire propagation mechanism: a neighbouring pair whose MinTemp is now satisfied
// reacts on a later tick, so a thermite mass burns through itself without any
// propagation code. The cascade is bounded by cascadeCeiling so one ignition cannot
// heat the whole world, and by the fact that heat diffuses away through the existing
// environment pass.
package simulation

import "github.com/worldweaver/worldweaver/internal/world"

const (
	// cascadeCeiling caps the temperature any single reaction may write, in the
	// simulation's tenths-of-a-degree unit — 32000 is 3200 °C.
	//
	// Without a cap, two mutually-exothermic reactions in contact form a runaway
	// that saturates int16 and then wraps, which reads as the world instantly
	// freezing and is very hard to trace back to a reaction.
	//
	// 3200 °C is just under the 3276 °C ceiling that int16 tenths allows. That is
	// below tungsten's real 5555 °C boiling point, which is a known limitation of
	// the current unit and the reason design.md §2.1 calls for changing it.
	cascadeCeiling int16 = 32000

	// heatShare is the divisor applied to HeatDelta for surrounding cells. The
	// reacting pair takes the full delta; neighbours take a quarter, which carries
	// a chain without every reaction heating a wide area at once.
	heatShare int16 = 4

	// reactionInterval spaces the pass out. Reactions do not need to be evaluated
	// at the full 60 Hz to look continuous, and every third tick is a 3x saving on
	// the most expensive optional pass in the tick.
	reactionInterval uint64 = 3
)

// orthogonal neighbour offsets. Reactions are checked orthogonally rather than
// over all eight neighbours because diagonal contact between two cells is
// ambiguous in a grid this size and doubles the cost for no visible gain.
var reactionNeighbours = [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// simulateReactions applies the reaction table and relaxes temperature.
//
// # Why this does not honour chunk sleep
//
// The obvious implementation skips sleeping chunks like every other pass. That is
// wrong here, and subtly so: a chunk sleeps when nothing MOVED in it, but two
// reactants sitting motionless next to each other are not at equilibrium — they are
// a reaction waiting for its probability roll. A slow reaction like gold dissolving
// in aqua regia (1-in-200) almost never fires on the first pass, so the chunk slept
// and the reaction never happened at all. Only reactions fast enough to fire
// immediately appeared to work, which made the whole table look intermittent.
//
// Scanning every cell is affordable because of the reactivity gate: the first thing
// reactCell does is read the element's reactivity and return if it is zero, which is
// one indexed array read for empty, rock, soil, sand and glass — between them
// essentially all of a real world. The neighbour walk only happens for cells that
// can actually react.
//
// The pass is also spaced over reactionInterval ticks, so the full-grid scan happens
// at 20 Hz rather than 60 Hz.
func simulateReactions(w *world.World) {
	if w.Tick.Load()%reactionInterval != 0 {
		return
	}

	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			reactCell(w, x, y)
		}
	}
}

// relaxTemperatures dissipates reaction heat from registry elements.
//
// # Why it only touches registry elements
//
// Two earlier versions of this pass broke the existing simulation in ways that took
// a bisect to attribute.
//
// The first relaxed every cell toward a flat 20 °C. The world generator builds a
// gradient from about 15 °C at altitude to 35 °C low down, and the weather pass
// evaporates water above 15 °C, so warming the cool regions to 20 °C switched on
// evaporation across the whole map, dried the soil, and killed vegetation by drought.
//
// The second only cooled, and only above 60 °C. That still fought lava: lava pumps
// heat into its neighbours every tick, so cooling those neighbours competed with the
// legacy heat engine and shifted the thermodynamics of exactly the volcanic regions
// the vegetation equilibrium test measures.
//
// The boundary that works is by MATERIAL, not by temperature. Reaction heat lives in
// registry elements, and the legacy materials already have their own temperature
// handling in the environment, lava, fire and ice passes. Restricting this pass to
// IDs above the legacy range means it cannot compete with any of them.
//
// Like the reaction pass it deliberately ignores chunk sleep: a burnt-out crater is
// motionless, so its chunk sleeps, and cooling that honoured sleep would leave it at
// 3000 °C forever and re-ignite anything placed near it much later.
func relaxTemperatures(w *world.World) {
	if w.Tick.Load()%relaxInterval != 0 {
		return
	}

	for i := range w.Temperature {
		// Legacy materials keep their existing thermodynamics. This is the whole
		// safety boundary of the pass.
		if w.Material[i] <= world.MatGrass {
			continue
		}

		t := w.Temperature[i]
		if t <= reactionHeatFloor {
			continue
		}

		// Proportional decay, so a very hot cell sheds heat quickly and one near the
		// floor settles gently.
		//
		// The divisor is deliberately large. An earlier version used /8 every 10
		// ticks, which drained 1600 °C to 1200 °C in a fifth of a second — fast
		// enough that no temperature-gated reaction ever got a window to fire, so
		// thermite and the Haber process silently never happened. A thermite charge
		// burns for seconds in reality, and the simulation has to allow at least
		// that long for the rolls.
		delta := (t - reactionHeatFloor) / 64
		if delta == 0 {
			delta = 1
		}
		w.Temperature[i] = t - delta
	}
}

// reactionHeatFloor is the temperature above which a cell is considered to be
// carrying reaction heat rather than ordinary climate, in tenths of a degree —
// 600 is 60.0 °C.
//
// The world generator tops out around 35 °C, so nothing in a natural climate
// reaches this. That gap is what lets the pass distinguish "hot because something
// exploded" from "hot because it is a desert" without tracking provenance.
const reactionHeatFloor int16 = 600

// relaxInterval spaces the cooling scan out. Cooling does not need to be evaluated
// every tick to look continuous, and a full-grid linear scan is the one pass here
// that cannot use the reactivity gate.
const relaxInterval uint64 = 30

// reactCell tries the reaction table for one cell against its neighbours.
func reactCell(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	mat := w.Material[i]

	// The gate that makes this pass affordable: a cell whose element cannot react
	// never walks its neighbours. Rock, sand, soil, glass and empty all land here.
	reactivity := world.ReactivityOf(mat)
	if reactivity == 0 {
		return
	}

	for _, d := range reactionNeighbours {
		nx, ny := x+d[0], y+d[1]
		j := w.Index(nx, ny)
		if j < 0 {
			continue
		}

		other := w.Material[j]
		r := lookupReaction(mat, other)
		if r == nil {
			continue
		}

		// Temperature gate. The hotter of the two cells decides, because a cold
		// reactant touching a hot one is exactly how ignition spreads.
		if r.MinTemp != world.TempNone {
			hotter := w.Temperature[i]
			if w.Temperature[j] > hotter {
				hotter = w.Temperature[j]
			}
			if hotter < r.MinTemp {
				continue
			}
		}

		// Catalyst gate. Searched around BOTH reacting cells, over the full 3x3 of
		// each. A catalyst is present in a region rather than aligned to one
		// operand, and checking only the first cell made the outcome depend on
		// which reactant the pass happened to visit first — so aqua regia worked or
		// failed depending on scan order.
		if r.Catalyst != 0 &&
			!catalystNearby(w, x, y, r.Catalyst) &&
			!catalystNearby(w, nx, ny, r.Catalyst) {
			continue
		}

		// Probability roll, scaled by how reactive the element is. Caesium should
		// not react at iron's pace, and this is where that difference lands.
		chance := int(r.Chance)
		if chance > 1 {
			chance = chance * 128 / (int(reactivity) + 1)
			if chance < 1 {
				chance = 1
			}
		}
		if chance > 1 && w.RNG().Intn(chance) != 0 {
			continue
		}

		applyReaction(w, i, j, x, y, nx, ny, r)
		// One reaction per cell per pass. Without this a cell surrounded by
		// reactants would transform four times in one tick, and only the last
		// would be visible — the intermediate products would vanish silently.
		return
	}
}

// catalystNearby reports whether the catalyst element is in the 3x3 neighbourhood.
func catalystNearby(w *world.World, x, y int, catalyst uint8) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if k := w.Index(x+dx, y+dy); k >= 0 && w.Material[k] == catalyst {
				return true
			}
		}
	}
	return false
}

// applyReaction writes the products and distributes the heat.
func applyReaction(w *world.World, i, j, x, y, nx, ny int, r *Reaction) {
	w.SetMaterial(x, y, r.ProductA)
	w.SetMaterial(nx, ny, r.ProductB)

	// Transient products need a lifetime or they persist forever as static cells.
	// The legacy transient materials own this state, so it is set here rather than
	// inferred from the registry.
	seedProductLifetime(w, i, r.ProductA)
	seedProductLifetime(w, j, r.ProductB)

	if r.HeatDelta == 0 {
		return
	}

	// The reacting pair takes the full delta.
	addHeat(w, i, r.HeatDelta)
	addHeat(w, j, r.HeatDelta)

	// Surrounding cells take a share, which is what lets a chain propagate.
	share := r.HeatDelta / heatShare
	if share == 0 {
		return
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if k := w.Index(x+dx, y+dy); k >= 0 {
				addHeat(w, k, share)
			}
		}
	}
}

// addHeat applies a temperature delta with saturation.
//
// Saturating rather than wrapping matters: int16 overflow on a runaway chain would
// flip a 4000 °C fire to -32000 °C, which reads as the world instantly freezing and
// is very hard to trace back to a reaction.
func addHeat(w *world.World, i int, delta int16) {
	t := int32(w.Temperature[i]) + int32(delta)
	if t > int32(cascadeCeiling) {
		t = int32(cascadeCeiling)
	}
	if t < -2730 { // absolute zero: -273.0 °C in tenths
		t = -2730
	}
	w.Temperature[i] = int16(t)
}

// seedProductLifetime gives a transient product the lifetime its handler expects.
func seedProductLifetime(w *world.World, i int, mat uint8) {
	switch mat {
	case world.MatFire:
		w.Lifetime[i] = 120
	case world.MatSmoke:
		w.Lifetime[i] = 90
	case world.MatVapor, world.ElSteam:
		w.Lifetime[i] = 90
	case world.MatEmber:
		w.Lifetime[i] = 60
	case world.ElHydrogen, world.ElOxygen, world.ElChlorine,
		world.ElNitrogen, world.ElCO2, world.ElMethane, world.ElAmmonia:
		// Gases disperse rather than decaying on a timer, but a lifetime keeps a
		// sealed pocket from lasting forever and is what stops a hydrogen
		// explosion leaving permanent gas behind.
		w.Lifetime[i] = 240
	}
}
