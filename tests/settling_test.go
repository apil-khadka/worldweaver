package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// buildSealedBasin returns a world containing a rock basin partly filled with
// water and nothing else. Isolating the liquid from weather, erosion neighbours
// and terrain lets the test attribute any ongoing change to the water itself.
func buildSealedBasin(t *testing.T) *world.World {
	t.Helper()

	w := world.New(128, 64, 7)

	// Rock shell around the lower half.
	for x := 20; x < 108; x++ {
		w.SetMaterial(x, 50, world.MatRock)
	}
	for y := 30; y <= 50; y++ {
		w.SetMaterial(20, y, world.MatRock)
		w.SetMaterial(107, y, world.MatRock)
	}

	// Fill part of the basin with water, leaving headroom above it.
	for y := 40; y < 50; y++ {
		for x := 21; x < 107; x++ {
			w.SetMaterial(x, y, world.MatWater)
		}
	}

	w.ClearDirty()
	return w
}

func snapshotMaterials(w *world.World) []uint8 {
	out := make([]uint8, len(w.Material))
	copy(out, w.Material)
	return out
}

func countDifferences(a, b []uint8) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

// TestSealedBasinSettles asserts that still water stops changing.
//
// Perpetual cell churn is not only wasted work: every move marks its chunk dirty,
// which keeps the chunk awake, streams updates for a visually static body of
// water, and makes the surface shimmer on screen.
func TestSealedBasinSettles(t *testing.T) {
	w := buildSealedBasin(t)
	eng := simulation.NewEngine(w, metrics.New())

	// Let the body find its level first.
	for i := 0; i < 400; i++ {
		eng.TickOnce()
	}

	before := snapshotMaterials(w)
	waterCells := 0
	for _, m := range before {
		if m == world.MatWater {
			waterCells++
		}
	}
	if waterCells == 0 {
		t.Fatal("basin lost all its water")
	}

	// Now measure change over a window during which nothing should happen.
	const window = 300
	for i := 0; i < window; i++ {
		eng.TickOnce()
	}

	changed := countDifferences(before, snapshotMaterials(w))
	churn := float64(changed) / float64(waterCells) * 100
	t.Logf("%d water cells; %d cells changed over %d ticks (%.3f%% churn)",
		waterCells, changed, window, churn)

	if churn > 0.1 {
		t.Errorf("settled water churned %.3f%% of its cells, want at most 0.1%%", churn)
	}
}

// TestSettledBasinChunksSleep verifies the churn actually translates into the
// chunk scheduler standing down, which is what removes the cost and the
// network traffic.
func TestSettledBasinChunksSleep(t *testing.T) {
	w := buildSealedBasin(t)
	eng := simulation.NewEngine(w, metrics.New())

	for i := 0; i < 700; i++ {
		eng.TickOnce()
	}

	sleeping := w.SleepingChunkCount()
	total := len(w.Chunks)
	t.Logf("%d of %d chunks sleeping", sleeping, total)

	if sleeping == 0 {
		t.Error("no chunks are sleeping — a settled world should be almost entirely idle")
	}
}
