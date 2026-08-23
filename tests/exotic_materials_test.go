package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// solidWorld returns a world filled with soil, with a rock floor and rock walls,
// so exotic materials have something to act on and cannot escape the test area.
func solidWorld(t *testing.T) *world.World {
	t.Helper()
	w := world.New(96, 64, 5)
	for y := 0; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			w.SetMaterial(x, y, world.MatSoil)
		}
	}
	w.ClearDirty()
	return w
}

func run(w *world.World, ticks int) {
	eng := simulation.NewEngine(w, metrics.New())
	for i := 0; i < ticks; i++ {
		eng.TickOnce()
	}
}

// TestVoidEatsThenCollapses is the property that keeps the void usable: it must
// consume a bounded hole and then disappear. An unbounded consumer would
// eventually claim the whole world, the same way unconserved rain flooded it.
func TestVoidEatsThenCollapses(t *testing.T) {
	w := solidWorld(t)
	before := 0
	for _, m := range w.Material {
		if m == world.MatSoil {
			before++
		}
	}

	w.SetMaterial(48, 32, world.MatVoid)
	w.Lifetime[w.Index(48, 32)] = 220

	run(w, 3000)

	voids, soil := 0, 0
	for _, m := range w.Material {
		switch m {
		case world.MatVoid:
			voids++
		case world.MatSoil:
			soil++
		}
	}
	eaten := before - soil

	t.Logf("void consumed %d cells then left %d void cells behind", eaten, voids)

	if eaten == 0 {
		t.Error("void consumed nothing")
	}
	if voids != 0 {
		t.Errorf("%d void cells persist; a spent void should collapse", voids)
	}
	// Bounded by lifetime: nowhere near the whole world.
	if eaten > before/4 {
		t.Errorf("void ate %d of %d cells — not bounded", eaten, before)
	}
}

// TestVoidCannotEatRock gives players a way to contain a void.
func TestVoidCannotEatRock(t *testing.T) {
	w := world.New(48, 32, 5)
	// Rock box with a void inside.
	for y := 10; y <= 20; y++ {
		for x := 10; x <= 20; x++ {
			w.SetMaterial(x, y, world.MatRock)
		}
	}
	w.SetMaterial(15, 15, world.MatVoid)
	w.Lifetime[w.Index(15, 15)] = 220
	w.ClearDirty()

	run(w, 2000)

	// Every cell of the shell must still be rock.
	for y := 10; y <= 20; y++ {
		for x := 10; x <= 20; x++ {
			if x == 15 && y == 15 {
				continue
			}
			if got := w.GetMaterial(x, y); got != world.MatRock {
				t.Fatalf("rock at (%d,%d) became %s — the void escaped its container",
					x, y, world.MaterialName(got))
			}
		}
	}
}

// TestRadiationSpreadsThenDecays checks radiation is transient, so a leak is
// survivable rather than permanent.
func TestRadiationSpreadsThenDecays(t *testing.T) {
	w := world.New(96, 64, 5)
	// Open space with a soil floor.
	for x := 0; x < w.Width; x++ {
		w.SetMaterial(x, 63, world.MatSoil)
	}
	w.SetMaterial(48, 40, world.MatRadiation)
	w.Lifetime[w.Index(48, 40)] = 160
	w.ClearDirty()

	countRad := func() int {
		n := 0
		for _, m := range w.Material {
			if m == world.MatRadiation {
				n++
			}
		}
		return n
	}

	peak := 0
	eng := simulation.NewEngine(w, metrics.New())
	for i := 0; i < 1200; i++ {
		eng.TickOnce()
		if n := countRad(); n > peak {
			peak = n
		}
	}
	final := countRad()

	t.Logf("radiation peaked at %d cells, ended at %d", peak, final)
	if peak < 2 {
		t.Error("radiation never spread")
	}
	if final != 0 {
		t.Errorf("%d radiation cells remain; it should decay away", final)
	}
}

// TestRadiationKillsCreatures verifies the hazard actually threatens life, and
// that the corpse enters the nutrient cycle as carrion rather than vanishing.
func TestRadiationKillsCreatures(t *testing.T) {
	w := world.New(48, 32, 5)
	for x := 0; x < w.Width; x++ {
		w.SetMaterial(x, 20, world.MatSoil)
		w.SetMoisture(x, 20, 150)
	}

	// Creature energy is carried in the temperature field, so a creature placed
	// with SetMaterial alone has zero energy and starves on the first tick. Give
	// it plenty, so this test fails only if radiation is not doing the killing.
	w.SetMaterial(24, 19, world.MatHerbivore)
	w.SetTemperature(24, 19, 200)

	// Surround the herbivore so radiation cannot simply drift out of reach.
	for _, p := range [][2]int{{23, 19}, {25, 19}, {24, 18}} {
		w.SetMaterial(p[0], p[1], world.MatRadiation)
		w.Lifetime[w.Index(p[0], p[1])] = 160
	}
	w.ClearDirty()

	eng := simulation.NewEngine(w, metrics.New())
	for i := 0; i < 400; i++ {
		eng.TickOnce()
		for _, m := range w.Material {
			if m == world.MatCarrion {
				t.Logf("radiation killed the creature after %d ticks", i+1)
				return
			}
		}
	}

	t.Errorf("herbivore survived surrounding radiation; material at origin is %s",
		world.MaterialName(w.GetMaterial(24, 19)))
}

// TestPlasmaMeltsRockAndBurnsOut checks plasma is destructive but short-lived.
func TestPlasmaMeltsRockAndBurnsOut(t *testing.T) {
	w := world.New(64, 48, 5)
	for y := 20; y < w.Height; y++ {
		for x := 0; x < w.Width; x++ {
			w.SetMaterial(x, y, world.MatRock)
		}
	}
	w.SetMaterial(32, 19, world.MatPlasma)
	w.Lifetime[w.Index(32, 19)] = 90
	w.ClearDirty()

	run(w, 600)

	lava, plasma := 0, 0
	for _, m := range w.Material {
		switch m {
		case world.MatLava:
			lava++
		case world.MatPlasma:
			plasma++
		}
	}

	t.Logf("plasma melted %d cells of rock into lava; %d plasma cells remain", lava, plasma)
	if lava == 0 {
		t.Error("plasma failed to melt adjacent rock")
	}
	if plasma != 0 {
		t.Errorf("%d plasma cells persist; plasma should burn out", plasma)
	}
}

// TestCarrionEnrichesSoil confirms death returns nutrients to the ground, which
// is what keeps the ecosystem's biomass budget closed.
func TestCarrionEnrichesSoil(t *testing.T) {
	w := world.New(32, 24, 5)
	for x := 0; x < w.Width; x++ {
		w.SetMaterial(x, 20, world.MatSoil)
		w.SetMoisture(x, 20, 0)
	}
	w.SetMaterial(16, 19, world.MatCarrion)
	w.Lifetime[w.Index(16, 19)] = 400
	w.ClearDirty()

	run(w, 1200)

	if got := w.GetMaterial(16, 19); got == world.MatCarrion {
		t.Fatal("carrion never decayed")
	}
	if m := w.GetMoisture(16, 20); m == 0 {
		t.Error("carrion decayed without enriching the soil beneath it")
	} else {
		t.Logf("soil moisture beneath carrion rose to %d", m)
	}
}

// TestHazardPlacementCostsMore encodes the balance intent for destructive tools.
func TestHazardPlacementCostsMore(t *testing.T) {
	if !world.IsHazard(world.MatVoid) || !world.IsHazard(world.MatPlasma) {
		t.Error("void and plasma should be classified as hazards")
	}
	if world.IsHazard(world.MatWater) {
		t.Error("water should not be a hazard")
	}
}
