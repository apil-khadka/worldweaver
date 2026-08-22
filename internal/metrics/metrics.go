// Package metrics collects runtime telemetry from the simulation engine and
// network layer.  All counters are safe to read and write concurrently.
//
// Metrics are exposed:
//   - in the live UI footer (TPS, active cells, RTT)
//   - via the JSON /api/metrics endpoint (for tooling and dashboards)
//   - via /debug/pprof/ (Go profiling, development only)
package metrics

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds all runtime observability data for WorldWeaver.
type Metrics struct {
	// Simulation counters (updated every tick)
	TotalTicks   atomic.Uint64
	ActiveCells  atomic.Int64
	ActiveChunks atomic.Int64

	// Players (updated by the network hub)
	PlayerCount atomic.Int32

	// Network throughput (bytes per second, updated periodically)
	OutboundBPS atomic.Int64

	// Tick latency tracking (protected by mu)
	mu           sync.Mutex
	tickDurations []time.Duration // ring buffer of last N tick durations
	ringPos      int
}

const tickRingSize = 120 // ~2 seconds at 60 TPS

// New returns an initialized Metrics collector.
func New() *Metrics {
	return &Metrics{
		tickDurations: make([]time.Duration, tickRingSize),
	}
}

// RecordTick records the wall-clock duration of a single simulation tick.
func (m *Metrics) RecordTick(d time.Duration) {
	m.TotalTicks.Add(1)
	m.mu.Lock()
	m.tickDurations[m.ringPos%tickRingSize] = d
	m.ringPos++
	m.mu.Unlock()
}

// TickP95 returns the 95th-percentile tick duration from the ring buffer.
func (m *Metrics) TickP95() time.Duration {
	m.mu.Lock()
	buf := make([]time.Duration, tickRingSize)
	copy(buf, m.tickDurations)
	m.mu.Unlock()

	// Simple selection sort on a small slice is fine.
	sorted := make([]int64, 0, tickRingSize)
	for _, d := range buf {
		if d > 0 {
			sorted = append(sorted, int64(d))
		}
	}
	if len(sorted) == 0 {
		return 0
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := int(math.Ceil(float64(len(sorted))*0.95)) - 1
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return time.Duration(sorted[idx])
}

// TPS returns the current simulation ticks per second based on the ring buffer.
func (m *Metrics) TPS() float64 {
	m.mu.Lock()
	count := 0
	var total time.Duration
	for _, d := range m.tickDurations {
		if d > 0 {
			count++
			total += d
		}
	}
	m.mu.Unlock()
	if count == 0 || total == 0 {
		return 0
	}
	return float64(count) / total.Seconds()
}

// Snapshot returns a point-in-time copy of all metrics suitable for JSON
// serialization or live UI rendering.
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		TotalTicks:   m.TotalTicks.Load(),
		ActiveCells:  m.ActiveCells.Load(),
		ActiveChunks: m.ActiveChunks.Load(),
		PlayerCount:  int(m.PlayerCount.Load()),
		OutboundBPS:  m.OutboundBPS.Load(),
		TPS:          m.TPS(),
		TickP95Ms:    float64(m.TickP95()) / float64(time.Millisecond),
	}
}

// Snapshot is a serializable point-in-time view of Metrics.
type Snapshot struct {
	TotalTicks   uint64  `json:"totalTicks"`
	ActiveCells  int64   `json:"activeCells"`
	ActiveChunks int64   `json:"activeChunks"`
	PlayerCount  int     `json:"playerCount"`
	OutboundBPS  int64   `json:"outboundBPS"`
	TPS          float64 `json:"tps"`
	TickP95Ms    float64 `json:"tickP95Ms"`
}
