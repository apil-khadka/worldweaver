# Performance Benchmark — Tasks

## Phase 1 — Go Benchmarks

- [x] Create `benchmarks/simulation_bench_test.go`
- [x] Implement populateStandard() with deterministic material distribution
- [x] Implement BenchmarkWorld512x512 (262,144 cells)
- [x] Implement BenchmarkWorld1024x512 (524,288 cells)
- [x] Implement BenchmarkWorld1024x1024 (1,048,576 cells)
- [x] Implement BenchmarkWorld2048x1024 (2,097,152 cells)
- [x] Add 100-tick warm-up before measurement in each benchmark
- [x] Report custom cells/op metric via b.ReportMetric
- [x] Report ms/tick derived metric
- [x] Add b.ReportAllocs() to track per-tick allocations
- [ ] Run benchmarks and capture actual results for docs/performance/benchmark-results.md
- [ ] Add benchmark comparison script (benchstat integration)
- [ ] Set up CI job to run benchmarks on each merge and store results
- [ ] Add BenchmarkChunkUpdate for network encoding overhead
- [ ] Add BenchmarkSave and BenchmarkLoad for persistence throughput

## Notes

- The suite compiles and runs; the previously recorded blockers about
  `internal/systems/materials` and the go.mod version were incorrect.
- Benchmark numbers are needed to decide the world size in the
  `god-sandbox-pivot` spec (Phase A7 / B).
