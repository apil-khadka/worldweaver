// Package benchmarks provides reproducible simulation performance benchmarks.
//
// # Usage
//
//	go test -bench=. -benchmem -count=3 ./benchmarks/...
//
// # Configuration
//
// Each benchmark uses a fixed seed (20260823) so results are comparable
// across runs.  Never pre-fill results — run on real hardware and report
// actual numbers.
//
// # Benchmark matrix
//
//	World Size  | Cells
//	512×512     | 262,144
//	1024×512    | 524,288
//	1024×1024   | 1,048,576
//
// Record: TPS, tick p95, active cells, RAM, connected clients.
// See docs/performance/benchmarks.md for the results table.
package benchmarks

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

func BenchmarkWorld512x512(b *testing.B)    { benchWorld(b, 512, 512) }
func BenchmarkWorld1024x512(b *testing.B)   { benchWorld(b, 1024, 512) }
func BenchmarkWorld1024x1024(b *testing.B)  { benchWorld(b, 1024, 1024) }
func BenchmarkWorld2048x1024(b *testing.B)  { benchWorld(b, 2048, 1024) }

func benchWorld(b *testing.B, width, height int) {
	b.Helper()
	w := world.New(width, height, 20260823)
	w.Generate()
	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		eng.TickOnce()
	}
	cells := width * height
	b.ReportMetric(float64(cells), "cells/op")
	b.ReportMetric(float64(b.Elapsed().Milliseconds())/float64(b.N), "ms/tick")
}
