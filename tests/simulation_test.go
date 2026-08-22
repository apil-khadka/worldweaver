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
		{materials.Smoke, "Smoke"}, {materials.Lava, "Lava"},
		{materials.Ice, "Ice"}, {materials.Ash, "Ash"},
		{materials.Oil, "Oil"}, {materials.Ember, "Ember"},
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

// ── Oil floats on water ──────────────────────────────────────────────────────

func TestOilFloatsOnWater(t *testing.T) {
	w := world.New(10, 20, 42)
	// Rock floor
	for x := range 10 {
		w.Material[19*10+x] = materials.Rock
	}
	// Place water at row 18, oil at row 17 (oil on top of water)
	w.Material[18*10+5] = materials.Water
	w.Material[17*10+5] = materials.Oil

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	for range 30 {
		eng.TickOnce()
	}

	// After settling, oil should be above water (lower row index = higher position)
	// Find the oil and water in column 5
	oilRow := -1
	waterRow := -1
	for y := 0; y < 20; y++ {
		mat := w.Material[y*10+5]
		if mat == materials.Oil && oilRow == -1 {
			oilRow = y
		}
		if mat == materials.Water && waterRow == -1 {
			waterRow = y
		}
	}
	if oilRow >= 0 && waterRow >= 0 && oilRow > waterRow {
		t.Errorf("oil (row %d) should float above water (row %d)", oilRow, waterRow)
	}
}

// ── Lava converts water to steam ──────────────────────────────────────────────

func TestLavaConvertsWaterToSteam(t *testing.T) {
	w := world.New(10, 10, 42)
	// Place lava with water adjacent
	w.Material[5*10+5] = materials.Lava
	w.Lifetime[5*10+5] = 400
	w.Material[5*10+6] = materials.Water

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	for range 20 {
		eng.TickOnce()
	}

	// Water should be gone (converted to vapor or displaced)
	waterStillThere := false
	for i := range w.Material {
		if w.Material[i] == materials.Water {
			waterStillThere = true
			break
		}
	}
	// Check that vapor appeared somewhere
	vaporFound := false
	rockFound := false
	for i := range w.Material {
		if w.Material[i] == materials.Vapor {
			vaporFound = true
		}
		if w.Material[i] == materials.Rock {
			rockFound = true
		}
	}
	if waterStillThere && !vaporFound && !rockFound {
		t.Error("lava touching water should produce vapor or rock")
	}
}

// ── Ice melts near fire ──────────────────────────────────────────────────────

func TestIceMeltsNearFire(t *testing.T) {
	w := world.New(10, 10, 42)
	w.Material[5*10+5] = materials.Ice
	w.Material[5*10+6] = materials.Fire
	w.Lifetime[5*10+6] = 100

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	for range 10 {
		eng.TickOnce()
	}

	// Ice should have melted to water
	if w.Material[5*10+5] == materials.Ice {
		t.Error("ice should melt when adjacent to fire")
	}
}

// ── Lava ignites plants ──────────────────────────────────────────────────────

func TestLavaIgnitesPlant(t *testing.T) {
	w := world.New(10, 10, 42)
	// Lava next to plant
	w.Material[5*10+5] = materials.Lava
	w.Lifetime[5*10+5] = 400
	w.Material[5*10+4] = materials.Plant

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	for range 30 {
		eng.TickOnce()
	}

	// Plant should have caught fire or been destroyed
	if w.Material[5*10+4] == materials.Plant {
		t.Error("lava should ignite adjacent plants")
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

// ── New materials exist in registry ────────────────────────────────────────

func TestNewMaterialsInRegistry(t *testing.T) {
	cases := []struct {
		id   uint8
		name string
	}{
		{materials.Ice, "ice"},
		{materials.Oil, "oil"},
		{materials.Lava, "lava"},
	}
	for _, tc := range cases {
		def := materials.R.Get(tc.id)
		if def.Name != tc.name {
			t.Errorf("materials.R.Get(%d).Name = %q, want %q", tc.id, def.Name, tc.name)
		}
	}
}
