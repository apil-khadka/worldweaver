// Package simulation advances the world state on a fixed timestep.
// It is the only package allowed to mutate the world.
package simulation

import (
	"log"
	"sync"
	"time"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/world"
)

const (
	TargetTPS    = 60
	TickInterval = time.Second / TargetTPS
)

// Engine owns the simulation loop.
type Engine struct {
	world   *world.World
	metrics *metrics.Metrics

	// Queued player actions; consumed at the start of each tick.
	actionMu sync.Mutex
	actions  []PlayerAction

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewEngine constructs an Engine bound to the given world and metrics.
func NewEngine(w *world.World, m *metrics.Metrics) *Engine {
	return &Engine{
		world:   w,
		metrics: m,
		stopCh:  make(chan struct{}),
	}
}

// Start launches the simulation loop in a background goroutine.
func (e *Engine) Start() {
	e.wg.Add(1)
	go e.loop()
	log.Println("Simulation engine started")
}

// Stop signals the simulation loop to exit and waits for it to finish.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	log.Println("Simulation engine stopped")
}

// EnqueueAction safely adds a player action to be processed on the next tick.
// Wakes the target chunk and its neighbors to ensure the action takes effect.
func (e *Engine) EnqueueAction(a PlayerAction) {
	e.actionMu.Lock()
	e.actions = append(e.actions, a)
	e.actionMu.Unlock()

	// Wake the chunk(s) affected by this action so they are simulated.
	w := e.world
	w.WakeChunkAt(a.X, a.Y)
	// Also wake chunks covered by the action radius.
	for _, dy := range []int{-a.Radius, 0, a.Radius} {
		for _, dx := range []int{-a.Radius, 0, a.Radius} {
			w.WakeChunkAt(a.X+dx, a.Y+dy)
		}
	}
}

func (e *Engine) loop() {
	defer e.wg.Done()
	ticker := time.NewTicker(TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	start := time.Now()

	// Drain action queue
	e.actionMu.Lock()
	actions := e.actions
	e.actions = nil
	e.actionMu.Unlock()

	w := e.world
	w.ClearMoveFlags()

	// Apply player actions (these already woke chunks in EnqueueAction)
	for _, a := range actions {
		applyAction(w, a)
	}

	// Advance simulation chunk-by-chunk, skipping sleeping chunks.
	// Within each active chunk: bottom-to-top, alternating horizontal direction.
	leftToRight := w.Tick%2 == 0
	chunkSize := w.ChunkSize

	for cy := w.ChunkH - 1; cy >= 0; cy-- {
		for cx := range w.ChunkW {
			idx := cy*w.ChunkW + cx
			chunk := &w.Chunks[idx]

			// Skip sleeping chunks — the core optimization.
			if chunk.Sleeping {
				continue
			}

			// Determine cell bounds for this chunk (clamped to world edges).
			startX := cx * chunkSize
			startY := cy * chunkSize
			endX := startX + chunkSize
			endY := startY + chunkSize
			if endX > w.Width {
				endX = w.Width
			}
			if endY > w.Height {
				endY = w.Height
			}

			// Simulate cells within this chunk: bottom-to-top.
			for y := endY - 1; y >= startY; y-- {
				if leftToRight {
					for x := startX; x < endX; x++ {
						simulateCell(w, x, y)
					}
				} else {
					for x := endX - 1; x >= startX; x-- {
						simulateCell(w, x, y)
					}
				}
			}

			// If this chunk had changes, wake its neighbors so they
			// react to cross-boundary effects (falling sand, spreading fire).
			if chunk.ChangedThisTick {
				w.WakeNeighbors(cx, cy)
			}
		}
	}

	// Environmental pass — only on active chunks.
	updateEnvironmentChunked(w)

	// Update sleep states at end of tick.
	w.UpdateSleepStates()

	w.Tick++
	elapsed := time.Since(start)
	e.metrics.RecordTick(elapsed)
}
