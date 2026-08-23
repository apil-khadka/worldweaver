// Package world owns the world state: material arrays, environmental fields,
// and chunk bookkeeping. It does NOT contain simulation logic.
package world

import (
	"math/rand"
	"sync/atomic"
)

// World holds the full authoritative state of the simulated environment.
type World struct {
	Width  int
	Height int
	Seed   int64

	// Material grid stored as a flat array: index = y*Width + x
	Material []uint8

	// Environmental fields (same layout as Material)
	Temperature []int16  // stored as fixed-point tenths of degrees Celsius
	Moisture    []uint8  // 0–255
	Lifetime    []uint16 // ticks remaining for transient materials (fire, smoke, vapor)
	Flags       []uint8  // per-cell bit flags

	// Creature state. Energy previously shared the Temperature array, which meant
	// the Heat power wrote straight into a creature's energy and made anything it
	// touched effectively immortal. It also left no way to give creatures a
	// temperature response, since the field was already spoken for.
	Energy []uint8 // 0–255 food reserve; 0 means starved
	Thirst []uint8 // 0–255, rises until the creature drinks

	// Chunks
	Chunks     []Chunk
	ChunkW     int // width in chunks
	ChunkH     int // height in chunks
	ChunkSize  int // cells per chunk edge (e.g. 32 or 64)

	// Simulation tick counter.
	// Atomic because it is read from HTTP handler goroutines (the WebSocket
	// welcome message) while the simulation loop increments it. Everything
	// else that reads world state does so from the post-tick hook, on the
	// simulation goroutine; the tick counter is the one deliberate exception
	// to keep connection setup lock-free.
	Tick atomic.Uint64

	rng *rand.Rand
}

// New allocates and zeroes a World of the given dimensions.
func New(width, height int, seed int64) *World {
	size := width * height
	w := &World{
		Width:       width,
		Height:      height,
		Seed:        seed,
		Material:    make([]uint8, size),
		Temperature: make([]int16, size),
		Moisture:    make([]uint8, size),
		Lifetime:    make([]uint16, size),
		Flags:       make([]uint8, size),
		Energy:      make([]uint8, size),
		Thirst:      make([]uint8, size),
		ChunkSize:   64,
		rng:         rand.New(rand.NewSource(seed)),
	}
	w.initChunks()
	return w
}

// Index converts (x, y) world coordinates to a flat array index.
// Returns -1 for out-of-bounds coordinates.
func (w *World) Index(x, y int) int {
	if x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return -1
	}
	return y*w.Width + x
}

// Generate builds a fresh initial world terrain using the stored seed.
func (w *World) Generate() {
	gen := newGenerator(w, w.rng)
	gen.generate()
}

// RNG exposes the world-level random source for deterministic use.
func (w *World) RNG() *rand.Rand { return w.rng }
