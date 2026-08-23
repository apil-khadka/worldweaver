# God Sandbox Pivot — Tasks

Phase A is scoped to fit before the submission deadline. Phase B is the larger
reshaping that follows.

## Phase A1 — Remove the 2.5D direction

- [ ] Delete `web/isometric-renderer.ts`
- [ ] Delete `web/pixi-renderer.ts`
- [ ] Remove the `?view=` parameter and renderer branching from `web/main.ts`
- [ ] Remove the view toggle button from `web/play.html` and its `V` key binding
- [ ] Remove `pixi.js` from `web/package.json` and reinstall
- [ ] Confirm bundle size drops and `npm run build` is clean
- [ ] Record the decision in `docs/decisions/ADR-010-2d-only.md`
- [ ] Mark ADR-003 (WebGL2 primary) as superseded where it implies alternatives

## Phase A2 — Stop the visual vibration

- [ ] Replace `hash(cellCoord + floor(u_time * 12.0))` fire flicker with a
      continuous phase-shifted function of position and time
- [ ] Replace the water `sparkle` term likewise; hash on position only
- [ ] Audit the shader for any remaining time-quantised hashing
- [ ] Verify: with the engine stopped, two successive frames of one region are
      near-identical

## Phase A3 — Settling

- [ ] Add `FlagSettled` to `internal/world/fields.go`
- [ ] Clear `FlagSettled` on a cell and its neighbours whenever a cell changes
- [ ] Skip settled cells in `simulateCell` dispatch
- [ ] Mark sand settled when every candidate destination is blocked
- [ ] Mark water settled when it cannot fall and no neighbouring column is lower
- [ ] Fix water lateral spread to require a strictly lower destination, so a
      level surface does not oscillate
- [ ] Test: sealed basin changes <0.1% of cells over 300 ticks and its chunks sleep
- [ ] Confirm chunk sleeping now engages for still water (watch `activeChunks`)

## Phase A4 — Conserved water cycle

- [ ] Make evaporation, condensation and precipitation strictly 1:1
- [ ] Track total water-equivalent cells (water + vapour + cloud) on the world
- [ ] Add a configurable water ceiling as a fraction of world cells
- [ ] Scale evaporation up and precipitation down when over budget
- [ ] Make the Rain power draw against the budget instead of creating water freely
- [ ] Test: 100,000 ticks with no input keeps total water within ±15% and shows
      no monotonic upward trend
- [ ] Delete the flooded `world.snapshot.flooded.bak` once the fix is confirmed

## Phase A5 — God tools

- [ ] Add a `tool` field to `PowerInputMsg` (`force`, `place`, `erase`, `raise`, `lower`)
- [ ] Extend `PowerRequest.Validate` for tool, material and per-tool radius caps
- [ ] Implement `place`: paint the chosen material within the brush
- [ ] Implement `erase`: clear to empty within the brush
- [ ] Implement `raise` / `lower`: add or remove terrain at the surface
- [ ] Area-scaled influence costs per the design table
- [ ] Wake affected chunks and clear `FlagSettled` so edits take effect immediately
- [ ] Client: material palette UI listing placeable materials
- [ ] Client: brush size control with a cursor radius indicator
- [ ] Client: tool selector alongside the existing power bar
- [ ] Test: each tool changes the world as expected; invalid requests are rejected
      and leave the world untouched

## Phase A6 — Camera

- [ ] Change `fitZoom` to fit world height only
- [ ] Clamp horizontal panning to world bounds
- [ ] Verify no empty margin is visible at any zoom level
- [ ] Retune camera acceleration for horizontal traversal

## Phase A7 — Widen the world one step

- [ ] Run the existing benchmark suite and record real numbers
- [ ] If 60 TPS holds, change defaults to 2048x768
- [ ] Re-verify snapshot size and initial load time at the new dimensions
- [ ] Update `docs/performance/benchmark-results.md` with measured values

## Phase B — Full reshaping (after submission)

- [ ] Implement five-band strata generation (sky, surface, underground, cavern, underworld)
- [ ] Implement five horizontal biomes with blended transitions
- [ ] Move to 3072x768 once benchmarks support it
- [ ] Interest-managed chunk streaming keyed on each client's viewport
- [ ] Binary protocol to replace JSON+base64 for chunk updates
- [ ] Cave generation tuned per band rather than globally
- [ ] Aquifers in the underground band feeding surface springs

## Completed already (context for this pivot)

- [x] Fix empty initial snapshot — the blank-canvas bug
- [x] Fix inverted Y axis in the WebGL2 shader
- [x] Populate `activeCells` / `activeChunks` metrics
- [x] Correct TPS to wall-clock measurement
- [x] Session resume so refresh does not force re-login
- [x] HTTP caching: immutable hashed assets, no-cache HTML
- [x] Camera fit so the world fills the canvas
- [x] Rock strata shading, so bedrock reads as geology
- [x] Vegetation: trees with trunks and crowns, 20x more plant coverage
- [x] Fix plant die-off — stem cells count as water-fed via the plant below them
