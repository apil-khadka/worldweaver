package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/world"
)

// Fixtures use a 320-cell-wide world deliberately. pastureWorld lays a single
// grass row, so the producer count equals the width, and recovery will not fire
// below 12*20 = 240 producer cells — the carrying-capacity gate that distinguishes
// "the ecosystem collapsed" from "this world never had one". A 128-wide fixture
// silently tested the gate rather than the recovery.

// A world with no producer base is not an ecosystem that collapsed — it is a world
// that never had one, and seeding grazers into it just starves them.
//
// This gate is also what keeps recovery out of worlds it has no business touching. A
// scratch world built to exercise one material has zero creatures by construction,
// and before the gate existed recovery fired there: the seeded grazers ate the grass
// a regrowth test was measuring, and the extra RNG draws shifted an unrelated
// plasma test off its expected outcome.
func TestBarrenWorldIsNotRecolonised(t *testing.T) {
	// Soil floor, no grass at all.
	w := world.New(320, 64, 11)
	for x := 0; x < w.Width; x++ {
		for y := w.Height - 6; y < w.Height; y++ {
			w.SetMaterial(x, y, world.MatSoil)
		}
	}
	w.ClearDirty()

	tick(w, 800)

	grazers, predators := countTier(w)
	if grazers > 0 || predators > 0 {
		t.Errorf("barren world was recolonised with %d grazers and %d predators; "+
			"there is no producer base to feed them", grazers, predators)
	}
}

// A world with only a token amount of vegetation must also be left alone, or a
// recolonised flock strips it immediately and the world is worse off than before.
func TestTokenVegetationDoesNotTriggerRecolonisation(t *testing.T) {
	w := world.New(64, 48, 13) // one grass row = 64 producers, well under the gate

	floor := w.Height - 6
	for x := 0; x < w.Width; x++ {
		for y := floor; y < w.Height; y++ {
			w.SetMaterial(x, y, world.MatSoil)
			w.SetMoisture(x, y, 200)
		}
		w.SetMaterial(x, floor-1, world.MatGrass)
	}
	w.ClearDirty()

	tick(w, 800)

	if grazers, _ := countTier(w); grazers > 0 {
		t.Errorf("%d grazers were seeded into a world with too little vegetation "+
			"to support them", grazers)
	}
}

// countTier reports the grazer and predator populations as tiers rather than
// species: sheep and plain herbivores fill the same role in the food chain.
func countTier(w *world.World) (grazers, predators int) {
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

// An empty grazer tier must not stay empty. Reproduction needs a living parent,
// so population zero is an absorbing state: without a floor, a world that loses
// its grazers can never get them back and the food chain is permanently broken.
// This is exactly the state the shipped snapshot had drifted into — no grass, no
// grazers, and a residue of starving predators.
func TestExtinctGrazersAreReintroduced(t *testing.T) {
	w := pastureWorld(320, 64, 200)

	if g, _ := countTier(w); g != 0 {
		t.Fatalf("fixture should start with no grazers, found %d", g)
	}

	// The census runs on an interval, so allow several windows.
	tick(w, 800)

	grazers, _ := countTier(w)
	if grazers == 0 {
		t.Fatal("grazer tier stayed extinct: recovery never fired, so a world " +
			"that loses its grazers can never get them back")
	}
	t.Logf("grazers recolonised: %d", grazers)
}

// Recovery is an extinction floor, not population control. A world that still
// has grazers must be left alone, or the predator-prey cycles the simulation is
// built on get flattened.
func TestHealthyGrazerPopulationIsNotTopUpped(t *testing.T) {
	w := pastureWorld(320, 64, 200)

	// Seed a living population by hand, on the grass row where there is food.
	const seeded = 40
	placed := 0
	grassRow := w.Height - 7
	for x := 0; x < w.Width && placed < seeded; x += 3 {
		if w.GetMaterial(x, grassRow) != world.MatGrass {
			continue
		}
		placeCreature(w, x, grassRow, world.MatSheep, 200, 0)
		placed++
	}
	if placed == 0 {
		t.Fatal("fixture placed no grazers")
	}

	before, _ := countTier(w)
	tick(w, 400)
	after, _ := countTier(w)

	// The population may fall (starvation) or rise (breeding) on its own. What
	// must not happen is an injection on top of a living population, which shows
	// up as a jump far beyond what breeding can manage in this window.
	if after > before*3 {
		t.Errorf("grazer population jumped %d → %d: recovery appears to top up "+
			"a living population instead of only refilling an extinct tier",
			before, after)
	}
	t.Logf("living population left alone: %d → %d", before, after)
}

// Predators must not be reintroduced into a world with nothing to eat: they
// would starve immediately and the pass would achieve nothing.
func TestPredatorsNotSeededWithoutPrey(t *testing.T) {
	w := pastureWorld(320, 64, 200)

	tick(w, 800)

	grazers, predators := countTier(w)
	if predators > 0 && grazers < 60 {
		t.Errorf("predators (%d) introduced with only %d grazers to feed on: "+
			"they will starve", predators, grazers)
	}
	t.Logf("grazers=%d predators=%d", grazers, predators)
}

// Recolonised creatures must arrive with a real energy reserve, or they die on
// the tick after they appear and nothing is actually recovered.
func TestSeededGrazersHaveEnergy(t *testing.T) {
	w := pastureWorld(320, 64, 200)
	tick(w, 500)

	found, starving := 0, 0
	for i, m := range w.Material {
		if m != world.MatHerbivore && m != world.MatSheep {
			continue
		}
		found++
		if w.Energy[i] == 0 {
			starving++
		}
	}

	if found == 0 {
		t.Skip("no grazers placed in this window; covered by the recovery test")
	}
	if starving > 0 {
		t.Errorf("%d of %d recolonised grazers have zero energy and die "+
			"immediately", starving, found)
	}
	t.Logf("%d recolonised grazers, all with energy", found)
}
