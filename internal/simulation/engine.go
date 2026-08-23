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

	// postTicks run at the end of every tick, on the simulation goroutine.
	// They are the sanctioned way to observe or broadcast world state:
	// anything that reads the world from another goroutine is a data race.
	postTickMu sync.Mutex
	postTicks  []func(w *world.World)

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

// AddPostTick registers fn to run at the end of every simulation tick, on
// the simulation goroutine. This is the only supported way to observe world
// state outside this package: the world belongs to the running loop, and any
// concurrent reader is a data race. Safe to call before or after Start.
func (e *Engine) AddPostTick(fn func(w *world.World)) {
	e.postTickMu.Lock()
	e.postTicks = append(e.postTicks, fn)
	e.postTickMu.Unlock()
}

// EnqueueAction safely adds a player action to be processed on the next tick.
// It is safe to call from any goroutine; the affected chunks are woken when
// the action is applied, on the simulation goroutine.
func (e *Engine) EnqueueAction(a PlayerAction) {
	e.actionMu.Lock()
	e.actions = append(e.actions, a)
	e.actionMu.Unlock()
}

// wakeActionArea wakes the chunks covered by an action so it takes effect
// even in a sleeping region. Runs on the simulation goroutine only.
func wakeActionArea(w *world.World, a PlayerAction) {
	w.WakeChunkAt(a.X, a.Y)
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

	// Apply player actions, waking their chunks first so sleeping regions
	// still react. Waking here keeps chunk state private to the tick.
	for _, a := range actions {
		wakeActionArea(w, a)
		applyAction(w, a)
	}

	// Advance simulation chunk-by-chunk, skipping sleeping chunks.
	// Within each active chunk: bottom-to-top, alternating horizontal direction.
	leftToRight := w.Tick.Load()%2 == 0
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

	// Weather cycle pass — evaporation & cloud formation on active chunks.
	updateWeatherCycle(w)

	// Reaction pass — the declarative element table. Runs every few ticks and
	// skips any cell whose element cannot react, so most of the grid costs
	// nothing. See ADR-011.
	simulateReactions(w)

	// Cooling — relaxes reaction heat back toward ambient. Deliberately outside the
	// chunk-sleep optimisation: a burnt-out crater is motionless, so its chunk
	// sleeps, and cooling that honoured sleep would leave it hot forever.
	relaxTemperatures(w)

	// Ecosystem floor — reintroduces a tier of the food chain that has gone
	// extinct. Grazer population zero is otherwise an absorbing state, so a
	// long-lived world drifted into having no animals and could never recover.
	recoverEcosystem(w)

	// Update sleep states at end of tick.
	w.UpdateSleepStates()

	// Publish cheap observability counters every tick.
	e.metrics.ActiveChunks.Store(int64(w.ActiveChunkCount()))

	// Counting non-empty cells is O(width*height), so sample it at ~2 Hz
	// instead of every tick to keep the simulation loop cheap.
	if w.Tick.Load()%30 == 0 {
		e.metrics.ActiveCells.Store(int64(countNonEmptyCells(w)))
	}

	w.Tick.Add(1)
	elapsed := time.Since(start)
	e.metrics.RecordTick(elapsed)

	// Hand the finished tick to observers (broadcast, metrics, persistence).
	// Running these here serializes every world reader with the simulation,
	// which is what makes the world safe to share at all.
	e.postTickMu.Lock()
	hooks := make([]func(w *world.World), len(e.postTicks))
	copy(hooks, e.postTicks)
	e.postTickMu.Unlock()
	for _, fn := range hooks {
		fn(w)
	}
}

// countNonEmptyCells reports how many cells currently hold material.
// Used for the live metrics HUD and the /api/metrics endpoint.
func countNonEmptyCells(w *world.World) int {
	n := 0
	for _, m := range w.Material {
		if m != world.MatEmpty {
			n++
		}
	}
	return n
}
