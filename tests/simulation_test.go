// Package tests contains integration and acceptance tests for WorldWeaver.
// Tests here exercise observable behaviour rather than implementation details.
// No WebSocket or HTTP required — simulation is tested in isolation.
package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/systems/materials"
	"github.com/worldweaver/worldweaver/internal/world"
)

// ── Material ID uniqueness ───────────────────────────────────────────────────

func TestMaterialIDsAreUnique(t *testing.T) {
	seen := map[uint8]string{}
	list := []struct {
		id   uint8
		name string
	}{
		{materials.Empty, "Empty"}, {materials.Rock, "Rock"},
		{materials.Soil, "Soil"}, {materials.Sand, "Sand"},
		{materials.Water, "Water"}, {materials.Plant, "Plant"},
		{materials.Fire, "Fire"}, {materials.Vapor, "Vapor"},
		{materials.Smoke, "Smoke"},
	}
	for _, m := range list {
		if prev, ok := seen[m.id]; ok {
			t.Errorf("duplicate material ID %d: %q and %q", m.id, prev, m.name)
		}
		seen[m.id] = m.name
	}
}

// ── World index bounds ───────────────────────────────────────────────────────

func TestWorldIndexBounds(t *testing.T) {
	w := world.New(100, 50, 1)
	cases := []struct{ x, y, want int }{
		{-1, 0, -1}, {0, -1, -1}, {100, 0, -1}, {0, 50, -1},
		{0, 0, 0}, {1, 0, 1}, {0, 1, 100},
	}
	for _, tc := range cases {
		if got := w.Index(tc.x, tc.y); got != tc.want {
			t.Errorf("Index(%d,%d) = %d, want %d", tc.x, tc.y, got, tc.want)
		}
	}
}

// ── Chunk dirty tracking ─────────────────────────────────────────────────────

func TestChunkDirtyTracking(t *testing.T) {
	w := world.New(128, 128, 1)
	w.SetMaterial(0, 0, materials.Sand)
	if !w.Chunks[0].Dirty {
		t.Error("chunk 0 should be dirty after SetMaterial at (0,0)")
	}
	w.ClearDirty()
	if w.Chunks[0].Dirty {
		t.Error("chunk 0 should not be dirty after ClearDirty")
	}
}

// ── Rock is static ───────────────────────────────────────────────────────────

func TestRockIsStatic(t *testing.T) {
	w := world.New(10, 10, 42)
	w.Material[5*10+5] = materials.Rock
	if w.Flags[5*10+5]&world.FlagMoved != 0 {
		t.Error("rock must not have FlagMoved set at initialization")
	}
}

// ── Sand falls one cell per tick ─────────────────────────────────────────────

func TestSandFallsDown(t *testing.T) {
	w := world.New(10, 20, 42)
	// Empty world, place sand at row 2
	w.Material[2*10+5] = materials.Sand

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	eng.TickOnce()

	// After one tick, sand should have moved down one row
	if w.Material[2*10+5] == materials.Sand {
		t.Error("sand should have moved down after one tick")
	}
	if w.Material[3*10+5] != materials.Sand {
		t.Errorf("sand should be at row 3 after one tick, got material %d", w.Material[3*10+5])
	}
}

// ── Sand stays on rock ───────────────────────────────────────────────────────

func TestSandStaysOnRock(t *testing.T) {
	w := world.New(10, 10, 42)
	// Solid rock bottom row
	for x := range 10 {
		w.Material[9*10+x] = materials.Rock
	}
	w.Material[8*10+5] = materials.Sand

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	// Run enough ticks for sand to settle
	for range 20 {
		eng.TickOnce()
	}

	// Sand must not have fallen through rock
	if w.Material[9*10+5] == materials.Sand {
		t.Error("sand must not fall through rock")
	}
}

// ── Water extinguishes fire ───────────────────────────────────────────────────

func TestWaterExtinguishesFire(t *testing.T) {
	w := world.New(10, 10, 42)
	// Place fire with water directly above
	w.Material[5*10+5] = materials.Fire
	w.Material[4*10+5] = materials.Water

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	// Run several ticks for water to fall and interact
	for range 15 {
		eng.TickOnce()
	}

	// Fire should be gone
	if w.Material[5*10+5] == materials.Fire {
		t.Error("water should have extinguished fire")
	}
}

// ── World generation produces a non-empty world ──────────────────────────────

func TestWorldGenerateProducesContent(t *testing.T) {
	w := world.New(128, 64, 20260823)
	w.Generate()

	rockCount := 0
	soilCount := 0
	for _, m := range w.Material {
		switch m {
		case materials.Rock:
			rockCount++
		case materials.Soil:
			soilCount++
		}
	}
	if rockCount == 0 {
		t.Error("generated world should contain rock cells")
	}
	if soilCount == 0 {
		t.Error("generated world should contain soil cells")
	}
}

// ── SetMaterial + GetMaterial round-trip ────────────────────────────────────

func TestSetGetMaterial(t *testing.T) {
	w := world.New(50, 50, 1)
	w.SetMaterial(10, 10, materials.Water)
	if got := w.GetMaterial(10, 10); got != materials.Water {
		t.Errorf("GetMaterial(10,10) = %d, want %d (Water)", got, materials.Water)
	}
}
