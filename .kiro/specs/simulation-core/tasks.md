# Simulation Core — Tasks

## Phase 1 — Engine & Materials

- [x] Allocate world with compact uint8 material + int16 temp + uint8 moisture + uint16 lifetime
- [x] Implement World.Index() with bounds checking (returns -1 for OOB)
- [x] Implement World.SetMaterial() that marks chunk dirty
- [x] Implement chunk grid initialisation with sleep/wake optimization
- [x] Create Engine with fixed 60 TPS ticker loop
- [x] Implement ClearMoveFlags() at tick start
- [x] Implement bottom-to-top iteration with horizontal alternation
- [x] Create simulateCell() dispatcher (cell.go) — handles 13 material types
- [x] Implement sand simulation (fall, diagonal, angle-of-repose friction, density displacement)
- [x] Implement water simulation (fall, spread 4 cells/tick, extinguish fire, wet soil, freeze near ice)
- [x] Implement fire simulation (spread to flammable neighbors, lifetime 120 ticks, heat radiation, ice melting)
- [x] Implement plant growth simulation (temperature/moisture dependent, spread to adjacent soil)
- [x] Implement vapor and smoke simulation (rising, drift, condensation)
- [x] Implement oil simulation (density layering, floats on water, highly flammable)
- [x] Implement ice simulation (melts near fire/lava, freezes adjacent water)
- [x] Implement lava simulation (converts water to steam, ignites plants, melts ice)
- [x] Implement environment tick (temperature decay) via updateEnvironmentChunked()
- [x] Expose Engine.TickOnce() for tests/benchmarks (tick_export.go)
- [x] Wire metrics recording in tick()
- [x] Implement multi-rate scheduler (fire 30Hz, plants 5Hz, creatures 10Hz, weather 2Hz)
- [x] Implement chunk sleeping (skip inactive chunks, wake on player action/neighbor activity)
- [ ] Add worker pool for parallel chunk processing (benchmark first)

## Phase 2 — Ecosystem & Weather

- [x] Implement creature system — Lotka-Volterra predator-prey (herbivores eat plants, predators hunt)
- [x] Add creature spawning rules (population caps, energy system, reproduction probability)
- [x] Implement weather system (evaporation → vapor → cloud formation → rain precipitation)
- [x] Add wind-driven weather pattern movement (global sinusoidal oscillation)
- [x] Implement hydraulic erosion (water erodes soil→sand→empty, probability-based)
- [x] Add sediment deposition in low-velocity water areas
- [ ] Implement creature migration (seasonal movement between biomes)
- [ ] Add food chain energy transfer model
- [ ] Implement day/night cycle affecting temperature and creature behavior

## Verified state

- `go build ./...`, `go vet ./...` and `go test ./tests/` all pass on the
  installed toolchain, so the `go 1.27.0` directive in go.mod and the
  `internal/systems/materials` package are not blockers as previously recorded.

## Known simulation issues

- [ ] Materials never come to rest: water and sand churn indefinitely, which
      keeps chunks awake and makes the render shimmer. See the
      `god-sandbox-pivot` spec, Phase A3.
- [ ] The weather cycle is not mass-conserving, so total water grows without
      bound. A long-running world reached 96% of cells occupied. See
      `god-sandbox-pivot` Phase A4.
