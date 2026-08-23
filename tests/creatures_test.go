package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// pastureWorld returns a world with a damp soil floor covered in grass, so
// grazers have renewable food and something to stand on.
func pastureWorld(width, height int, moisture uint8) *world.World {
	w := world.New(width, height, 3)
	floor := height - 6
	for x := 0; x < width; x++ {
		for y := floor; y < height; y++ {
			w.SetMaterial(x, y, world.MatSoil)
			w.SetMoisture(x, y, moisture)
		}
		w.SetMaterial(x, floor-1, world.MatGrass)
	}
	w.ClearDirty()
	return w
}

func countMaterial2(w *world.World, mat uint8) int {
	n := 0
	for _, m := range w.Material {
		if m == mat {
			n++
		}
	}
	return n
}

func placeCreature(w *world.World, x, y int, mat uint8, energy, thirst uint8) {
	w.SetMaterial(x, y, mat)
	i := w.Index(x, y)
	w.Energy[i] = energy
	w.Thirst[i] = thirst
}

func countCreatures(w *world.World) map[uint8]int {
	out := map[uint8]int{}
	for _, m := range w.Material {
		if world.IsCreature(m) {
			out[m]++
		}
	}
	return out
}

func tick(w *world.World, n int) {
	eng := simulation.NewEngine(w, metrics.New())
	for i := 0; i < n; i++ {
		eng.TickOnce()
	}
}

// TestHeatDoesNotFeedCreatures is a regression test for energy and temperature
// sharing one array.
//
// Creature energy used to be stored in the Temperature field, so applying the
// Heat power to a creature wrote directly into its food reserve and made it
// effectively immortal. Energy now has its own field.
func TestHeatDoesNotFeedCreatures(t *testing.T) {
	w := pastureWorld(64, 48, 200)

	// A creature with a tiny reserve, standing where there is nothing to eat.
	for x := 0; x < w.Width; x++ {
		if w.GetMaterial(x, 43) == world.MatPlant {
			w.SetMaterial(x, 43, world.MatEmpty)
		}
	}
	placeCreature(w, 32, 43, world.MatHerbivore, 4, 0)

	// Crank the temperature the way the Heat power would.
	i := w.Index(32, 43)
	w.Temperature[i] = 5000

	if got := w.Energy[i]; got != 4 {
		t.Fatalf("setup: energy is %d, expected it to be independent of temperature", got)
	}

	tick(w, 400)

	if world.IsCreature(w.GetMaterial(32, 43)) {
		t.Error("creature survived with no food while being heated — energy is still coupled to temperature")
	}
}

// TestCreatureDiesOfThirst verifies water is a real requirement.
func TestCreatureDiesOfThirst(t *testing.T) {
	// Bone-dry ground, no water anywhere.
	w := pastureWorld(48, 32, 0)
	placeCreature(w, 24, 27, world.MatSheep, 255, 200)

	tick(w, 2000)

	if world.IsCreature(w.GetMaterial(24, 27)) {
		t.Error("creature with a full belly but no water survived indefinitely")
	}
}

// TestGrazersPersistOnGrassland is the counterpart to the thirst test: given
// renewable food and damp ground, a flock should still be present later.
//
// It asserts persistence rather than a stable head count, because a flock on a
// finite pasture is expected to rise and fall as it grazes and the grass recovers.
func TestGrazersPersistOnGrassland(t *testing.T) {
	w := pastureWorld(160, 48, 220)
	for x := 20; x < 140; x += 20 {
		placeCreature(w, x, 41, world.MatSheep, 200, 0)
	}
	w.ClearDirty()

	start := countCreatures(w)[world.MatSheep]
	tick(w, 3000)
	end := countCreatures(w)[world.MatSheep]

	t.Logf("sheep %d -> %d, grass %d", start, end, countMaterial2(w, world.MatGrass))

	if end == 0 {
		t.Error("the whole flock died on watered grassland")
	}
}

// TestGrassRegrowsAfterGrazing checks the food web has a renewable base. Woody
// growth alone recovers too slowly for a grazing population to be sustainable.
func TestGrassRegrowsAfterGrazing(t *testing.T) {
	w := pastureWorld(96, 40, 220)

	// Strip the grass from the middle of the pasture.
	for x := 30; x < 66; x++ {
		w.SetMaterial(x, 33, world.MatEmpty)
	}
	w.ClearDirty()

	stripped := countMaterial2(w, world.MatGrass)
	tick(w, 3000)
	regrown := countMaterial2(w, world.MatGrass)

	t.Logf("grass after stripping %d, after regrowth %d", stripped, regrown)
	if regrown <= stripped {
		t.Errorf("grass did not regrow: %d -> %d", stripped, regrown)
	}
}

// TestPredatorsHuntGrazers verifies the top of the food chain actually feeds on
// the level below it.
func TestPredatorsHuntGrazers(t *testing.T) {
	w := pastureWorld(120, 40, 220)

	for x := 20; x < 100; x += 4 {
		placeCreature(w, x, 33, world.MatHerbivore, 200, 0)
	}
	placeCreature(w, 60, 32, world.MatPredator, 120, 0)
	w.ClearDirty()

	grazersBefore := countCreatures(w)[world.MatHerbivore]
	predatorEnergyBefore := int(w.Energy[w.Index(60, 32)])

	tick(w, 2000)

	counts := countCreatures(w)
	t.Logf("herbivores %d -> %d, predators now %d",
		grazersBefore, counts[world.MatHerbivore], counts[world.MatPredator])

	// A predator dropped into a dense herd should feed, which shows up either as
	// grazers being removed or as the predator line growing.
	if counts[world.MatHerbivore] >= grazersBefore && counts[world.MatPredator] <= 1 {
		t.Errorf("predator neither reduced the herd nor bred (energy started at %d)",
			predatorEnergyBefore)
	}
}

// TestTrophicPyramid checks the food web is bottom-heavy.
//
// Energy transfer between trophic levels is lossy — only around a tenth of the
// energy at one level reaches the next — so each level must support fewer
// individuals than the one beneath it. A world with more predators than grazers,
// or more grazers than plants, is not modelling that.
func TestTrophicPyramid(t *testing.T) {
	w := world.New(2048, 768, 20260823)
	w.Generate()

	producers := countMaterial2(w, world.MatGrass) + countMaterial2(w, world.MatPlant)
	grazers := countMaterial2(w, world.MatHerbivore) + countMaterial2(w, world.MatSheep)
	predators := countMaterial2(w, world.MatPredator)

	t.Logf("producers %d, grazers %d, predators %d", producers, grazers, predators)

	if producers <= grazers {
		t.Errorf("producers (%d) must outnumber grazers (%d)", producers, grazers)
	}
	if grazers <= predators {
		t.Errorf("grazers (%d) must outnumber predators (%d)", grazers, predators)
	}
}

// TestClimateLimitsBreeding checks that temperature tolerance shapes where a
// species can establish, so biomes carry different life.
func TestClimateLimitsBreeding(t *testing.T) {
	grow := func(temp int16) int {
		w := pastureWorld(200, 48, 220)
		for i := range w.Temperature {
			w.Temperature[i] = temp
		}
		for x := 20; x < 180; x += 20 {
			placeCreature(w, x, 41, world.MatHerbivore, 200, 0)
		}
		w.ClearDirty()

		eng := simulation.NewEngine(w, metrics.New())
		peak := countCreatures(w)[world.MatHerbivore]
		for i := 0; i < 4000; i++ {
			eng.TickOnce()
			if n := countCreatures(w)[world.MatHerbivore]; n > peak {
				peak = n
			}
		}
		return peak
	}

	temperate := grow(200) // 20 C — inside the herbivore band
	frozen := grow(-900)   // -90 C — far outside it

	t.Logf("herbivore peak population: temperate %d, frozen %d", temperate, frozen)

	// Breeding is blocked outside the tolerance band, so the temperate run should
	// reach a higher peak. The frozen run cannot exceed its starting count.
	if temperate <= frozen {
		t.Errorf("temperate peak %d did not exceed frozen peak %d — climate has no effect",
			temperate, frozen)
	}
}

// TestGeneratedWorldHasMultipleSpecies checks the generator seeds a food web
// across the map rather than a handful of animals in one band.
func TestGeneratedWorldHasMultipleSpecies(t *testing.T) {
	w := world.New(2048, 768, 20260823)
	w.Generate()

	counts := countCreatures(w)
	for mat, n := range counts {
		t.Logf("%-10s %d", world.MaterialName(mat), n)
	}

	total := 0
	for _, n := range counts {
		total += n
	}
	if total < 50 {
		t.Errorf("only %d creatures generated across a 2048-wide world", total)
	}
	if len(counts) < 2 {
		t.Errorf("only %d species present; expected a food web", len(counts))
	}
	if counts[world.MatPredator] == 0 {
		t.Error("no predators generated, so the food web has no top level")
	}

	// Every creature must start with a usable reserve, or the population dies
	// off in the first seconds of simulation.
	for idx, m := range w.Material {
		if world.IsCreature(m) && w.Energy[idx] == 0 {
			t.Fatalf("%s at index %d generated with no energy", world.MaterialName(m), idx)
		}
	}
}

// TestCreaturePopulationScalesWithWidth confirms seeding is density-based.
func TestCreaturePopulationScalesWithWidth(t *testing.T) {
	count := func(width int) int {
		w := world.New(width, 768, 20260823)
		w.Generate()
		n := 0
		for _, m := range w.Material {
			if world.IsCreature(m) {
				n++
			}
		}
		return n
	}

	narrow := count(1024)
	wide := count(2048)
	ratio := float64(wide) / float64(narrow)

	t.Logf("creatures: 1024 wide -> %d, 2048 wide -> %d (ratio %.2f)", narrow, wide, ratio)

	if narrow == 0 {
		t.Fatal("no creatures generated at 1024 wide")
	}
	// Doubling the width should roughly double the population.
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("population ratio %.2f is not proportional to width", ratio)
	}
}
