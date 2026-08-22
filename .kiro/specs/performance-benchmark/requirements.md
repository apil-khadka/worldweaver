# Performance Benchmark — Requirements

## REQ-PERF-001: Reproducible Benchmarks
All benchmarks SHALL be deterministic and reproducible. Same hardware, same seed, same world composition produces consistent results (±5% variance).

**Acceptance:** Running benchmark 3× produces p95 tick times within 5% of each other.

## REQ-PERF-002: Fixed Seed
Benchmarks SHALL use a fixed, documented seed (42) to ensure identical world generation across runs and machines.

**Acceptance:** World state checksum after 100 ticks matches expected value for seed 42.

## REQ-PERF-003: Standard World Sizes
The benchmark suite SHALL test the following world sizes: 512×512, 1024×512, 1024×1024, 2048×1024.

**Acceptance:** `go test -bench=.` output shows results for all four sizes.

## REQ-PERF-004: Report Metrics
Benchmarks SHALL report: nanoseconds per operation (ns/op from testing.B), cells per operation as a custom metric, and milliseconds per tick derived.

**Acceptance:** Benchmark output includes ns/op and custom cells/op metric for each world size.

## REQ-PERF-005: No Fabricated Results
Benchmark results SHALL only be reported from actual execution. No hardcoded or estimated values. CI may record results but must re-run on each commit.

**Acceptance:** Removing benchmark binary and re-running produces fresh results; no cached values in source.

## References
- WorldWeaver_Full_Project_Documentation.md § 36 (Performance Targets)
- WorldWeaver_Full_Project_Documentation.md § 37 (Benchmark Methodology)
