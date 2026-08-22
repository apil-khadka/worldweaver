# WorldWeaver Benchmark Methodology

## Philosophy

> Performance claims must be measured, not assumed.

WorldWeaver benchmarks use Go's standard `testing.B` framework for reproducibility. Results are never pre-filled.

## Running Benchmarks

```bash
# Full benchmark suite
go test -bench=. -benchmem -count=3 -timeout 120s ./benchmarks/...

# Single configuration
go test -bench=BenchmarkWorld1024x1024 -benchmem -count=5 ./benchmarks/...

# With CPU profile
go test -bench=BenchmarkWorld1024x1024 -cpuprofile=cpu.prof ./benchmarks/...
go tool pprof cpu.prof
```

## Benchmark Matrix

| Configuration | Cells | Purpose |
|---------------|-------|---------|
| 512×512 | 262,144 | CI baseline (fast) |
| 1024×512 | 524,288 | Default world size |
| 1024×1024 | 1,048,576 | Target scale |
| 2048×1024 | 2,097,152 | Stress/stretch |

## What Is Measured

- **ns/op** — Time per single simulation tick
- **cells/op** — Total cells processed (world size)
- **ms/tick** — Milliseconds per tick (derived)
- **B/op** — Bytes allocated per tick
- **allocs/op** — Heap allocations per tick

## Benchmark Environment

Always document when reporting:
- Hardware (CPU, RAM)
- Go version
- OS
- WorldWeaver commit hash
- Number of benchmark iterations (`-count`)

## Results Template

| World | Cells | ns/op | ms/tick | B/op | allocs/op |
|-------|-------|-------|---------|------|-----------|
| 512×512 | 262k | _run benchmark_ | | | |
| 1024×512 | 524k | _run benchmark_ | | | |
| 1024×1024 | 1.0M | _run benchmark_ | | | |
| 2048×1024 | 2.1M | _run benchmark_ | | | |

## Profiling

```bash
# CPU profile
go test -bench=BenchmarkWorld1024x1024 -cpuprofile=cpu.prof ./benchmarks/...
go tool pprof -http=:8081 cpu.prof

# Memory profile
go test -bench=BenchmarkWorld1024x1024 -memprofile=mem.prof ./benchmarks/...
go tool pprof -http=:8081 mem.prof

# Execution trace
go test -bench=BenchmarkWorld512x512 -trace=trace.out ./benchmarks/...
go tool trace trace.out
```

## Optimisation Cycle

```
measure → identify bottleneck → hypothesis → change → measure again
```

Never: guess → rewrite architecture.

## References

- [ADR-009: Multi-rate Simulation](../decisions/ADR-009-multi-rate-simulation.md)
- Source: `benchmarks/simulation_bench_test.go`
