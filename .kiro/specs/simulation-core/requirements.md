# Simulation Core — Requirements

## REQ-SIM-001: Fixed Timestep
The simulation SHALL advance world state using a fixed timestep of 60 TPS, independent of client rendering frequency.

**Acceptance:** Server runs with zero clients; tick count increases at 60/sec ±1.

## REQ-SIM-002: World Array
The world SHALL store material state as compact uint8 arrays with separate environmental field arrays (int16 temperature, uint8 moisture, uint16 lifetime).

**Acceptance:** Memory layout confirmed via sizeof; no per-cell heap allocation.

## REQ-SIM-003: Boundary Handling
Simulation SHALL handle world edges gracefully — cells at boundaries must not cause panics or access violations.

**Acceptance:** World.Index() returns -1 for all OOB coordinates.

## REQ-SIM-004: Bottom-to-Top Processing
Falling materials (sand, water) SHALL be processed from bottom row upward to prevent double-movement within a single tick.

**Acceptance:** Sand placed at row 0 moves exactly 1 cell per tick in an empty column.

## REQ-SIM-005: Horizontal Alternation
Horizontal scan direction SHALL alternate each tick to avoid visible left/right bias.

**Acceptance:** Visual inspection shows symmetric sand pile formation.

## REQ-SIM-006: Move Flag
Each cell SHALL have a FlagMoved bit cleared at tick start and set after processing to prevent multi-processing.

**Acceptance:** Unit test confirms cell processed exactly once per tick.

## REQ-SIM-007: Active Chunk Tracking
Chunks SHALL track active/dirty state. Future optimisation may skip inactive chunks.

**Acceptance:** MarkDirty/ClearDirty tests pass.

## REQ-SIM-008: Deterministic Seeding
All randomness in the simulation SHALL use an explicit seeded RNG. Given the same seed and inputs, simulation produces identical output.

**Acceptance:** Two fresh worlds with same seed produce byte-identical material arrays after N ticks with no player input.

## REQ-SIM-009: Observable Metrics
The engine SHALL report tick duration to the metrics subsystem after every tick.

**Acceptance:** metrics.TotalTicks increases; TickP95() returns non-zero after 60+ ticks.

## REQ-SIM-010: No Network Dependency
The simulation engine SHALL operate correctly with zero connected clients. It must not import any network or transport package.

**Acceptance:** `go build ./internal/simulation/` succeeds with no network imports in dependency tree.

## References
- WorldWeaver_Full_Project_Documentation.md § 18 (Simulation Loop)
- WorldWeaver_Full_Project_Documentation.md § 22 (Fixed Timestep)
- WorldWeaver_Full_Project_Documentation.md § 27 (Go Concurrency)
- WorldWeaver_Full_Project_Documentation.md § 28 (Simulation Ordering)
