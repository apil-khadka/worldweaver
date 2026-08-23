package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// newFlatTestWorld returns a small world with a flat rock floor and open space
// above it, so tool effects are unambiguous.
func newFlatTestWorld() *world.World {
	w := world.New(64, 48, 11)
	for x := 0; x < w.Width; x++ {
		for y := 30; y < w.Height; y++ {
			w.SetMaterial(x, y, world.MatRock)
		}
	}
	w.ClearDirty()
	return w
}

func countMaterial(w *world.World, mat uint8) int {
	n := 0
	for _, m := range w.Material {
		if m == mat {
			n++
		}
	}
	return n
}

// applyTool enqueues a tool action and advances one tick so it is applied.
func applyTool(w *world.World, a simulation.PlayerAction) {
	eng := simulation.NewEngine(w, metrics.New())
	eng.EnqueueAction(a)
	eng.TickOnce()
}

func TestPlaceToolPaintsMaterial(t *testing.T) {
	w := newFlatTestWorld()
	before := countMaterial(w, world.MatWater)

	applyTool(w, simulation.PlayerAction{
		Tool:     simulation.ToolPlace,
		Material: world.MatWater,
		X:        32, Y: 20, Radius: 4,
		Intensity: 1.0,
	})

	after := countMaterial(w, world.MatWater)
	if after <= before {
		t.Fatalf("place tool created no water: %d -> %d", before, after)
	}
	t.Logf("water cells %d -> %d", before, after)
}

func TestEraseToolClearsCells(t *testing.T) {
	w := newFlatTestWorld()
	before := countMaterial(w, world.MatRock)

	applyTool(w, simulation.PlayerAction{
		Tool: simulation.ToolErase,
		X:    32, Y: 40, Radius: 5,
		Intensity: 1.0,
	})

	after := countMaterial(w, world.MatRock)
	if after >= before {
		t.Fatalf("erase tool removed no rock: %d -> %d", before, after)
	}
	t.Logf("rock cells %d -> %d", before, after)
}

// TestRaiseToolBuildsOnGround checks that raise only adds material where it has
// something to rest on, rather than painting soil into open air.
func TestRaiseToolBuildsOnGround(t *testing.T) {
	w := newFlatTestWorld()

	applyTool(w, simulation.PlayerAction{
		Tool: simulation.ToolRaise,
		X:    32, Y: 29, Radius: 6,
		Intensity: 1.0,
	})

	soil := countMaterial(w, world.MatSoil)
	if soil == 0 {
		t.Fatal("raise tool added no ground")
	}

	// Every new soil cell must sit directly on top of solid ground.
	for x := 0; x < w.Width; x++ {
		for y := 0; y < w.Height; y++ {
			if w.GetMaterial(x, y) != world.MatSoil {
				continue
			}
			below := w.GetMaterial(x, y+1)
			if below == world.MatEmpty {
				t.Fatalf("soil at (%d,%d) is floating in mid-air", x, y)
			}
		}
	}
	t.Logf("raise added %d soil cells, all supported", soil)
}

// TestLowerToolOnlyStripsExposedSurface checks that lower digs the top layer
// instead of hollowing out cells buried inside the terrain.
func TestLowerToolOnlyStripsExposedSurface(t *testing.T) {
	w := newFlatTestWorld()

	// Aim well below the surface; nothing there is exposed, so nothing should go.
	rockBefore := countMaterial(w, world.MatRock)
	applyTool(w, simulation.PlayerAction{
		Tool: simulation.ToolLower,
		X:    32, Y: 42, Radius: 3,
		Intensity: 1.0,
	})
	if got := countMaterial(w, world.MatRock); got != rockBefore {
		t.Errorf("lower removed buried rock: %d -> %d", rockBefore, got)
	}

	// Now aim at the surface row, which is exposed to the air above.
	applyTool(w, simulation.PlayerAction{
		Tool: simulation.ToolLower,
		X:    32, Y: 30, Radius: 3,
		Intensity: 1.0,
	})
	if got := countMaterial(w, world.MatRock); got >= rockBefore {
		t.Errorf("lower failed to strip the exposed surface: %d -> %d", rockBefore, got)
	}
}

// ── Validation ──────────────────────────────────────────────────────────────

func TestToolValidationRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		req  game.PowerRequest
	}{
		{"unknown tool", game.PowerRequest{Tool: "teleport", X: 10, Y: 10, Radius: 4}},
		{"creature material", game.PowerRequest{Tool: game.ToolPlace, Material: 14, X: 10, Y: 10, Radius: 4}},
		{"empty material", game.PowerRequest{Tool: game.ToolPlace, Material: 0, X: 10, Y: 10, Radius: 4}},
		{"out of bounds", game.PowerRequest{Tool: game.ToolErase, X: -5, Y: 10, Radius: 4}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := game.NewPlayer()
			req := tc.req
			if err := req.Validate(p, 64, 48); err == nil {
				t.Fatal("expected validation to reject the request")
			}
		})
	}
}

// TestToolRadiusIsClamped ensures a client cannot request an unbounded brush.
func TestToolRadiusIsClamped(t *testing.T) {
	p := game.NewPlayer()
	req := game.PowerRequest{
		Tool: game.ToolErase, Material: world.MatWater,
		X: 100, Y: 100, Radius: 100000,
	}
	if err := req.Validate(p, 1024, 512); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if req.Radius > game.MaxToolRadius {
		t.Errorf("radius %d exceeds cap %d", req.Radius, game.MaxToolRadius)
	}
}

// TestDirectEditCostsMoreThanForce encodes the balance intent: painting material
// is a more expensive way to change the world than using an elemental force.
func TestDirectEditCostsMoreThanForce(t *testing.T) {
	const radius = 8

	forcePlayer := game.NewPlayer()
	forceReq := game.PowerRequest{Tool: game.ToolForce, Power: game.PowerRain, X: 50, Y: 50, Radius: radius}
	if err := forceReq.Validate(forcePlayer, 512, 256); err != nil {
		t.Fatalf("force request rejected: %v", err)
	}
	forceSpent := forcePlayer.MaxInfluenceCap() - forcePlayer.Influence()

	placePlayer := game.NewPlayer()
	placeReq := game.PowerRequest{Tool: game.ToolPlace, Material: world.MatWater, X: 50, Y: 50, Radius: radius}
	if err := placeReq.Validate(placePlayer, 512, 256); err != nil {
		t.Fatalf("place request rejected: %v", err)
	}
	placeSpent := placePlayer.MaxInfluenceCap() - placePlayer.Influence()

	t.Logf("radius %d — force cost %.4f, place cost %.4f", radius, forceSpent, placeSpent)
	if placeSpent <= forceSpent {
		t.Errorf("place cost %.4f should exceed force cost %.4f", placeSpent, forceSpent)
	}
}

// TestInfluenceExhaustionBlocksEditing verifies the budget actually constrains
// god-mode editing rather than merely being reported.
func TestInfluenceExhaustionBlocksEditing(t *testing.T) {
	p := game.NewPlayer()

	accepted := 0
	for i := 0; i < 10000; i++ {
		req := game.PowerRequest{
			Tool: game.ToolPlace, Material: world.MatRock,
			X: 50, Y: 50, Radius: game.MaxToolRadius,
		}
		if err := req.Validate(p, 512, 256); err != nil {
			break
		}
		accepted++
	}

	if accepted == 0 {
		t.Fatal("no edits were accepted at full influence")
	}
	t.Logf("%d max-radius edits accepted before influence ran out", accepted)

	req := game.PowerRequest{
		Tool: game.ToolPlace, Material: world.MatRock,
		X: 50, Y: 50, Radius: game.MaxToolRadius,
	}
	if err := req.Validate(p, 512, 256); err == nil {
		t.Error("editing was still permitted after influence was exhausted")
	}
}
