package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// waterEquivalent counts every cell holding water in any phase. The weather
// cycle moves mass between these three, so their sum is what must stay bounded.
func waterEquivalent(w *world.World) (water, vapor, cloud int) {
	for _, m := range w.Material {
		switch m {
		case world.MatWater:
			water++
		case world.MatVapor:
			vapor++
		case world.MatCloud:
			cloud++
		}
	}
	return
}

// TestWaterCycleIsBounded asserts the weather cycle does not manufacture water.
//
// A long-running world was observed at 504,872 of 524,288 cells occupied — it had
// flooded solid, because precipitation introduced water that evaporation never
// removed. Beyond being wrong physically, a world that never reaches equilibrium
// never stops moving, so the render shimmers permanently.
func TestWaterCycleIsBounded(t *testing.T) {
	w := world.New(512, 256, 20260823)
	w.Generate()

	eng := simulation.NewEngine(w, metrics.New())

	w0, vap0, c0 := waterEquivalent(w)
	start := w0 + vap0 + c0
	t.Logf("start: water=%d vapor=%d cloud=%d total=%d", w0, vap0, c0, start)

	// Sample the trajectory so a steady climb is distinguishable from noise.
	const stages = 8
	const ticksPerStage = 2500
	totals := []int{start}
	for s := 0; s < stages; s++ {
		for i := 0; i < ticksPerStage; i++ {
			eng.TickOnce()
		}
		wc, vc, cc := waterEquivalent(w)
		totals = append(totals, wc+vc+cc)
	}

	t.Logf("water-equivalent over %d ticks: %v", stages*ticksPerStage, totals)

	final := totals[len(totals)-1]
	growth := float64(final-start) / float64(start) * 100
	t.Logf("net change: %+.1f%%", growth)

	// Mass should be conserved within a tolerance that allows for genuine
	// transient storage in vapour and cloud.
	if growth > 15.0 {
		t.Errorf("water grew %+.1f%%, want at most +15%% — the cycle is creating water", growth)
	}

	// Independently reject a monotonic climb, which indicates divergence even if
	// the total has not yet exceeded the tolerance.
	rising := 0
	for i := 1; i < len(totals); i++ {
		if totals[i] > totals[i-1] {
			rising++
		}
	}
	if rising == len(totals)-1 {
		t.Errorf("water rose in every one of %d windows — the cycle is divergent", rising)
	}
}
