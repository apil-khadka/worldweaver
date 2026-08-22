# Performance Benchmark — Design

## Framework

Using Go's built-in `testing.B` benchmark framework for reproducibility and toolchain integration:

```go
func BenchmarkWorld512x512(b *testing.B)   { benchmarkWorld(b, 512, 512) }
func BenchmarkWorld1024x512(b *testing.B)  { benchmarkWorld(b, 1024, 512) }
func BenchmarkWorld1024x1024(b *testing.B) { benchmarkWorld(b, 1024, 1024) }
func BenchmarkWorld2048x1024(b *testing.B) { benchmarkWorld(b, 2048, 1024) }
```

## Pre-Generated World

Each benchmark function:
1. Creates a world with fixed seed (42) at the specified dimensions
2. Populates it with a standard material distribution (40% empty, 30% sand, 15% water, 10% stone, 5% mixed)
3. Runs 100 warm-up ticks (outside timer) to reach steady-state
4. Resets timer and benchmarks single-tick execution

```go
func benchmarkWorld(b *testing.B, width, height int) {
    w := simulation.NewWorld(width, height, 42)
    populateStandard(w)

    engine := simulation.NewEngine(w)
    for i := 0; i < 100; i++ {
        engine.TickOnce()
    }

    cells := width * height
    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        engine.TickOnce()
    }

    b.ReportMetric(float64(cells), "cells/op")
    b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1e6, "ms/tick")
}
```

## Standard Material Distribution

The `populateStandard()` function fills the world deterministically using the world's seeded RNG:

- Bottom 20% of rows: 60% sand, 30% stone, 10% soil
- Middle 40%: 40% empty, 25% sand falling, 20% water, 15% mixed
- Top 40%: 80% empty, 10% smoke, 5% fire, 5% steam

This ensures the benchmark exercises all material simulation paths.

## Measurement

Each `b.N` iteration measures exactly one `TickOnce()` call. The Go test framework automatically determines the number of iterations needed for stable measurement.

Custom metrics:
- `cells/op` — total cells in the world (for normalisation across sizes)
- `ms/tick` — wall-clock milliseconds per tick (for budget checking against 16.67ms target)

## Running Benchmarks

```bash
# All benchmarks
go test -bench=BenchmarkWorld -benchtime=5s ./internal/simulation/

# Single size
go test -bench=BenchmarkWorld1024x1024 -benchtime=10s ./internal/simulation/

# With memory profiling
go test -bench=BenchmarkWorld1024x1024 -benchmem -memprofile=mem.prof ./internal/simulation/

# CPU profiling
go test -bench=BenchmarkWorld1024x1024 -cpuprofile=cpu.prof ./internal/simulation/
```

## Performance Budget

Target: single tick must complete within 16.67ms (60 FPS budget) for the target world size of 1024×512. The benchmark validates this:

| World Size | Max Acceptable Tick |
|------------|-------------------|
| 512×512 | 4 ms |
| 1024×512 | 8 ms |
| 1024×1024 | 16 ms |
| 2048×1024 | 32 ms (30 FPS OK) |
