package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// TestGeneratedWorldHasVisibleVariety guards against the world generating as a
// near-uniform bedrock mass.
//
// An earlier generation pass produced 50% rock but only 0.61% soil and 0.03%
// plants, because topsoil was a few cells thick and vegetation was restricted to
// a quarter of the map width. The result rendered as a flat brown slab with no
// discernible landscape features.
func TestGeneratedWorldHasVisibleVariety(t *testing.T) {
	w := world.New(1024, 512, 20260823)
	w.Generate()

	counts := map[uint8]int{}
	for _, m := range w.Material {
		counts[m]++
	}
	total := len(w.Material)
	pct := func(id uint8) float64 { return float64(counts[id]) / float64(total) * 100 }

	for id := uint8(0); id <= 16; id++ {
		if counts[id] > 0 {
			t.Logf("%-10s %8d  %5.2f%%", world.MaterialName(id), counts[id], pct(id))
		}
	}

	if got := pct(world.MatPlant); got < 0.4 {
		t.Errorf("plant coverage %.2f%%, want at least 0.4%%", got)
	}

	// Vegetation has to be spread across the map, not clustered in one band.
	// The original generator only planted between 48% and 72% of the width.
	columnsWithPlants := 0
	tallestRun := 0
	for x := 0; x < w.Width; x++ {
		has := false
		run := 0
		for y := 0; y < w.Height; y++ {
			if w.Material[y*w.Width+x] == world.MatPlant {
				has = true
				run++
				if run > tallestRun {
					tallestRun = run
				}
			} else {
				run = 0
			}
		}
		if has {
			columnsWithPlants++
		}
	}

	spread := float64(columnsWithPlants) / float64(w.Width) * 100
	t.Logf("vegetation spans %.1f%% of columns, tallest unbroken run %d cells",
		spread, tallestRun)

	if spread < 25.0 {
		t.Errorf("vegetation spans %.1f%% of map width, want at least 25%%", spread)
	}

	// Growth must be more than a one-cell fringe, otherwise it disappears when
	// the whole world is in view.
	if tallestRun < 5 {
		t.Errorf("tallest vegetation run is %d cells, want at least 5 so trees are visible", tallestRun)
	}

	// Water features should exist for the weather cycle to act on.
	if counts[world.MatWater] == 0 {
		t.Error("world generated with no water")
	}

	// Sanity: the world must not be overwhelmingly one material.
	if got := pct(world.MatRock); got > 70.0 {
		t.Errorf("rock coverage %.2f%%, want at most 70%%", got)
	}
}

// TestVegetationSurvivesSimulation guards against generated plants withering
// immediately once the simulation starts.
//
// The drought rule originally required every plant cell to sit next to moist
// soil. Cells partway up a trunk only touch other plant cells, so trees taller
// than one cell died within seconds and the world rendered as bare rock despite
// generating thousands of plant cells.
func TestVegetationSurvivesSimulation(t *testing.T) {
	w := world.New(512, 256, 20260823)
	w.Generate()

	countPlants := func() int {
		n := 0
		for _, m := range w.Material {
			if m == world.MatPlant {
				n++
			}
		}
		return n
	}

	before := countPlants()
	if before == 0 {
		t.Fatal("world generated with no vegetation")
	}

	eng := simulation.NewEngine(w, metrics.New())

	// Losing plants near lava, on volcanic slopes or in the desert is expected.
	// What matters is that the population settles into an equilibrium instead of
	// trending to zero, so the trajectory is sampled rather than one endpoint.
	samples := []int{before}
	for stage := 0; stage < 6; stage++ {
		for i := 0; i < 500; i++ {
			eng.TickOnce()
		}
		samples = append(samples, countPlants())
	}
	t.Logf("vegetation over 3000 ticks: %v", samples)

	final := samples[len(samples)-1]
	if final == 0 {
		t.Fatal("all vegetation died — the world cannot sustain plant life")
	}

	// Compare the last two windows: the population must have levelled off, not
	// still be draining away.
	prev := samples[len(samples)-2]
	if prev > 0 {
		lateLoss := float64(prev-final) / float64(prev) * 100
		t.Logf("loss over the final 500 ticks: %.1f%%", lateLoss)
		if lateLoss > 10.0 {
			t.Errorf("vegetation still declining %.1f%% per 500 ticks, expected equilibrium", lateLoss)
		}
	}

	// A healthy world keeps a meaningful share of its initial growth.
	if got := float64(final) / float64(before) * 100; got < 30.0 {
		t.Errorf("only %.1f%% of vegetation remains, want at least 30%%", got)
	}
}
