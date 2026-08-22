# Simulation Core — Tasks

- [x] Allocate world with compact uint8 material + int16 temp + uint8 moisture + uint16 lifetime
- [x] Implement World.Index() with bounds checking (returns -1 for OOB)
- [x] Implement World.SetMaterial() that marks chunk dirty
- [x] Implement chunk grid initialisation
- [x] Create Engine with fixed 60 TPS ticker loop
- [x] Implement ClearMoveFlags() at tick start
- [x] Implement bottom-to-top iteration with horizontal alternation
- [x] Create simulateCell() dispatcher (cell.go)
- [x] Implement sand simulation (fall, diagonal)
- [x] Implement water simulation (fall, spread, wet soil, extinguish fire)
- [x] Implement fire simulation (spread, lifetime, smoke)
- [x] Implement plant growth simulation
- [x] Implement environment tick (temperature decay)
- [x] Expose Engine.TickOnce() for tests/benchmarks
- [x] Wire metrics recording in tick()
- [ ] Implement multi-rate scheduler (fire 30Hz, plants 5Hz)
- [ ] Implement chunk sleeping (skip inactive chunks)
- [ ] Add worker pool for parallel chunk processing (benchmark first)
