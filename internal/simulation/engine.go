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
	TargetTPS       = 60
	TickInterval    = time.Second / TargetTPS
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
func (e *Engine) EnqueueAction(a PlayerAction) {
	e.actionMu.Lock()
	e.actions = append(e.actions, a)
	e.actionMu.Unlock()
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

	// Apply player actions
	for _, a := range actions {
		applyAction(w, a)
	}

	// Advance simulation — bottom-to-top, alternating horizontal direction
	leftToRight := w.Tick%2 == 0
	for y := w.Height - 1; y >= 0; y-- {
		if leftToRight {
			for x := range w.Width {
				simulateCell(w, x, y)
			}
		} else {
			for x := w.Width - 1; x >= 0; x-- {
				simulateCell(w, x, y)
			}
		}
	}

	w.Tick++
	elapsed := time.Since(start)
	e.metrics.RecordTick(elapsed)
}
