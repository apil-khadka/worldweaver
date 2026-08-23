# Material System — Tasks

## Phase 1 — Core Materials

- [x] Create `internal/world/materials.go` with material ID constants and helpers
- [x] Define material ID constants: MatEmpty(0), MatRock(1), MatSoil(2), MatSand(3), MatWater(4), MatPlant(5), MatFire(6), MatVapor(7), MatSmoke(8), MatLava(9), MatIce(10), MatAsh(11), MatOil(12), MatEmber(13), MatHerbivore(14), MatPredator(15), MatCloud(16)
- [x] Implement IsSolid(), IsLiquid(), IsGas(), IsFlammable(), IsTransient() helpers
- [x] Implement Density() lookup for density-based displacement
- [x] Implement MaterialName() for debug/logging
- [x] Define client-side palette (17 materials with RGBA values in webgl2-renderer.ts)
- [x] PixiJS renderer with custom GLSL filter for material → color mapping
- [x] WebGL2 renderer shader with water shimmer, fire glow bleed, depth shading, per-cell noise
- [x] Isometric renderer with per-material height values
- [ ] Create MATERIALS.md changelog documenting all IDs and version introduced
- [ ] Add material list sync message for client handshake (currently hardcoded on both sides)

## Notes

- Material constants exist in two places: `internal/world/materials.go` (used by
  the simulation) and `internal/systems/materials` (used by tests and the
  generation sub-packages). Both compile; `go vet ./...` is clean. Consolidating
  them onto one source of truth is worthwhile but is not a blocker.
