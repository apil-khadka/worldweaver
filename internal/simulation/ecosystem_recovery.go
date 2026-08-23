package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Ecosystem recovery — an extinction floor for the grazer tier.
//
// # Why this exists
//
// soil.go already gives grass a floor: damp soil holds a seed bank, so grass
// can return after being eaten to nothing. Grazers had no equivalent. Their only
// route into the world was the generator at world creation and the level-4 Life
// power, and reproduction needs a living parent, so grazer population zero is an
// ABSORBING STATE — mathematically unreachable in reverse.
//
// The consequence was not theoretical. A long-running world reliably ended up
// with grass and grazers gone and a residue of starving predators, and the food
// chain the game is built around could never restart. A player joining that world
// sees no animals at all and no way to make any, which reads as the ecosystem
// being broken rather than dead.
//
// # What it does
//
// When the grazer tier is empty, vegetated ground is recolonised at a slow rate:
// think of it as immigration from beyond the map edge, not spontaneous
// generation. It only fires when the tier is ACTUALLY empty, so a healthy world
// never sees it and predator-prey dynamics are left alone. Recolonising a species
// that is merely scarce would flatten the population cycles that make the
// simulation interesting.
//
// The predator tier gets the same treatment for the same reason, gated behind a
// grazer population large enough to feed it — reintroducing predators into an
// empty world would just starve them again.

const (
	// ecoCheckInterval is how often the census runs. Scanning the whole grid is
	// O(cells), so it happens on the order of once every few seconds rather than
	// every tick.
	ecoCheckInterval uint64 = 240

	// ecoGrazerSeedCount is how many grazers are introduced per recovery pass.
	// Small on purpose: enough to establish a breeding population, not enough to
	// hand the player a full flock the instant one dies.
	ecoGrazerSeedCount = 12

	// ecoPredatorSeedCount is the same for predators, kept lower because the ten
	// percent rule means a few go a long way.
	ecoPredatorSeedCount = 3

	// ecoPredatorPreyFloor is the grazer population a world needs before
	// predators are reintroduced, so they have something to eat on arrival.
	ecoPredatorPreyFloor = 60

	// ecoProducerPerGrazer is how many producer cells each recolonised grazer needs
	// before recovery will fire — the ten percent rule applied one tier down.
	//
	// This is what distinguishes "the ecosystem collapsed and should recover" from
	// "this world never had an ecosystem". Both have zero grazers, but only the
	// first has the plant biomass to support any.
	ecoProducerPerGrazer = 20

	// ecoMaxPlacementAttempts bounds the search for somewhere to put a creature.
	// A world with no vegetated ground at all should give up rather than spin.
	ecoMaxPlacementAttempts = 4000
)

// recoverEcosystem reintroduces an extinct tier of the food chain.
//
// Called once per tick from the engine; almost every call returns immediately on
// the interval check.
func recoverEcosystem(w *world.World) {
	if w.Tick.Load()%ecoCheckInterval != 0 {
		return
	}

	grazers, predators := censusCreatures(w)

	// Bottom of the chain first: predators seeded into a grazerless world would
	// only starve, so grazer recovery has to land before predator recovery is
	// even considered.
	if grazers == 0 {
		// Carrying capacity gate. A world with no producers is not an ecosystem
		// that collapsed — it is a world that never had one, and seeding grazers
		// into it just starves them.
		//
		// This also keeps recovery out of worlds it has no business touching. A
		// simulation fixture of bare soil, or a scratch world built to test one
		// material, has zero creatures by construction; without this gate recovery
		// fired there, and the seeded grazers ate the very grass a grass-regrowth
		// test was measuring.
		//
		// The ratio is the ten percent rule the predator gate already relies on,
		// applied one tier down: a grazer needs an order of magnitude more plant
		// biomass than its own mass to live off.
		if countProducers(w) < ecoGrazerSeedCount*ecoProducerPerGrazer {
			return
		}
		seedCreatures(w, ecoGrazerSeedCount, grazerForGround)
		return
	}

	if predators == 0 && grazers >= ecoPredatorPreyFloor {
		seedCreatures(w, ecoPredatorSeedCount, func(uint8) (uint8, bool) {
			return world.MatPredator, true
		})
	}
}

// countProducers counts the photosynthesising base of the food web.
func countProducers(w *world.World) int {
	n := 0
	for _, m := range w.Material {
		if world.IsProducer(m) {
			n++
		}
	}
	return n
}

// censusCreatures counts the grazer tier and the predator tier.
//
// Grazers are counted as one tier rather than per species: sheep and plain
// herbivores fill the same role, so a world with sheep but no herbivores is not
// missing a tier and needs no intervention.
func censusCreatures(w *world.World) (grazers, predators int) {
	for _, m := range w.Material {
		switch m {
		case world.MatHerbivore, world.MatSheep:
			grazers++
		case world.MatPredator:
			predators++
		}
	}
	return grazers, predators
}

// grazerForGround picks which grazer suits the ground beneath a candidate cell,
// matching the generator's rule so recolonised animals are distributed the same
// way the world was originally populated: sheep favour grassy soil, plain
// herbivores are less fussy and are the only grazer that takes to sand.
func grazerForGround(ground uint8) (uint8, bool) {
	switch ground {
	case world.MatSoil, world.MatGrass:
		return world.MatSheep, true
	case world.MatSand:
		return world.MatHerbivore, true
	case world.MatPlant:
		return world.MatHerbivore, true
	default:
		return 0, false
	}
}

// seedCreatures places up to n creatures on ground the chooser accepts.
//
// Placement is random rather than clustered so a recovering population is not
// wiped out by one local fire, and so the animals do not all appear in front of
// whichever player happens to be looking at that spot.
func seedCreatures(w *world.World, n int, choose func(ground uint8) (uint8, bool)) {
	if w.Width == 0 || w.Height == 0 {
		return
	}

	rng := w.RNG()
	placed := 0

	for attempt := 0; attempt < ecoMaxPlacementAttempts && placed < n; attempt++ {
		x := rng.Intn(w.Width)
		// The top row cannot hold a creature that needs ground below it, and the
		// bottom row has no cell beneath to stand on.
		y := rng.Intn(w.Height-1) + 1

		i := w.Index(x, y)
		if i < 0 {
			continue
		}

		// A creature needs an empty cell to occupy. Grass counts as empty here
		// for the same reason the generator allows it: the animal stands in it.
		if m := w.Material[i]; m != world.MatEmpty && m != world.MatGrass {
			continue
		}

		below := w.Index(x, y+1)
		if below < 0 {
			continue
		}

		species, ok := choose(w.Material[below])
		if !ok {
			continue
		}

		// Refuse ground that is on fire, flooded or otherwise lethal, so a
		// recovery pass does not spend its whole budget on animals that die in
		// the next few ticks.
		if !survivableSpot(w, i) {
			continue
		}

		w.Material[i] = species
		// Creatures carry their energy in the same field the traits read, so a
		// seeded animal has to start with a real reserve or it dies immediately.
		if t, hasTrait := Traits[species]; hasTrait {
			w.Energy[i] = t.StartEnergy
		}
		w.MarkDirty(x, y)
		placed++
	}
}

// survivableSpot reports whether a cell is safe enough to place a creature in.
func survivableSpot(w *world.World, i int) bool {
	// Standing in water or fire is immediately fatal.
	switch w.Material[i] {
	case world.MatWater, world.MatFire, world.MatLava, world.MatPlasma:
		return false
	}
	// Extreme temperature kills before the animal can move away.
	if t := w.Temperature[i]; t < -300 || t > 600 {
		return false
	}
	return true
}
