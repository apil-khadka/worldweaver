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

	// Total bytes ever sent to clients. This is a monotonic counter, not a rate;
	// call RecordOutbound to feed it and OutboundRate for bytes per second.
	OutboundTotal atomic.Int64

	// Tick latency tracking (protected by mu)
	mu           sync.Mutex
	tickDurations []time.Duration // ring buffer of last N tick durations
	ringPos      int

	// Wall-clock TPS tracking. TPS is ticks observed per second of real time,
	// which is not the same as 1/tickDuration (that would report how fast the
	// loop *could* run, not how fast it actually advances).
	windowStart  time.Time
	windowTicks  int
	tpsEstimate  float64

	// Outbound bandwidth, measured over a rolling window for the same reason.
	bytesWindowStart time.Time
	bytesInWindow    int64
	bpsEstimate      float64
}

// RecordOutbound accounts for bytes written to clients and refreshes the rolling
// bandwidth estimate roughly once per second.
func (m *Metrics) RecordOutbound(n int64) {
	m.OutboundTotal.Add(n)

	m.mu.Lock()
	m.bytesInWindow += n
	if elapsed := time.Since(m.bytesWindowStart); elapsed >= time.Second {
		m.bpsEstimate = float64(m.bytesInWindow) / elapsed.Seconds()
		m.bytesInWindow = 0
		m.bytesWindowStart = time.Now()
	}
	m.mu.Unlock()
}

// OutboundRate returns current outbound throughput in bytes per second.
func (m *Metrics) OutboundRate() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(m.bpsEstimate)
}

const tickRingSize = 120 // ~2 seconds at 60 TPS

// New returns an initialized Metrics collector.
func New() *Metrics {
	now := time.Now()
	return &Metrics{
		tickDurations:    make([]time.Duration, tickRingSize),
		windowStart:      now,
		bytesWindowStart: now,
	}
}

// RecordTick records the wall-clock duration of a single simulation tick.
func (m *Metrics) RecordTick(d time.Duration) {
	m.TotalTicks.Add(1)
	m.mu.Lock()
	m.tickDurations[m.ringPos%tickRingSize] = d
	m.ringPos++

	// Recompute wall-clock TPS roughly once per second.
	m.windowTicks++
	if elapsed := time.Since(m.windowStart); elapsed >= time.Second {
		m.tpsEstimate = float64(m.windowTicks) / elapsed.Seconds()
		m.windowTicks = 0
		m.windowStart = time.Now()
	}
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

// TPS returns the measured simulation ticks per second in wall-clock time.
func (m *Metrics) TPS() float64 {
	m.mu.Lock()
	tps := m.tpsEstimate
	// Before the first full window closes, derive an interim estimate so the
	// UI is not stuck at zero for the first second.
	if tps == 0 && m.windowTicks > 0 {
		if elapsed := time.Since(m.windowStart).Seconds(); elapsed > 0 {
			tps = float64(m.windowTicks) / elapsed
		}
	}
	m.mu.Unlock()
	return tps
}

// Snapshot returns a point-in-time copy of all metrics suitable for JSON
// serialization or live UI rendering.
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		TotalTicks:   m.TotalTicks.Load(),
		ActiveCells:  m.ActiveCells.Load(),
		ActiveChunks: m.ActiveChunks.Load(),
		PlayerCount:  int(m.PlayerCount.Load()),
		OutboundBPS:  m.OutboundRate(),
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
