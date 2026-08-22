package world

// ChunkState represents the activity state of a simulation chunk.
type ChunkState uint8

const (
	ChunkInactive ChunkState = iota
	ChunkActive
	ChunkDirty // has pending network updates
)

// SleepThreshold is how many consecutive idle ticks before a chunk sleeps.
const SleepThreshold = 10

// Chunk is a rectangular sub-region of the world used for:
//   - active-region optimization (skip idle chunks)
//   - dirty-region tracking for network updates
//   - future persistence partitioning
type Chunk struct {
	X, Y   int        // chunk grid coordinates (not cell coordinates)
	State  ChunkState
	Dirty  bool       // true if material changed this tick
	Active bool       // true if simulation should process this chunk

	// Sleep tracking: counts ticks with no cell changes.
	// When IdleTicks >= SleepThreshold the chunk is sleeping.
	Sleeping  bool
	IdleTicks uint16
	// Set by the simulation when any cell in this chunk is modified.
	ChangedThisTick bool
}

// CellX returns the first cell X coordinate of this chunk.
func (c *Chunk) CellX(chunkSize int) int { return c.X * chunkSize }

// CellY returns the first cell Y coordinate of this chunk.
func (c *Chunk) CellY(chunkSize int) int { return c.Y * chunkSize }

// WakeUp forces this chunk out of sleep and resets its idle counter.
func (c *Chunk) WakeUp() {
	c.Sleeping = false
	c.IdleTicks = 0
}

// initChunks creates the chunk grid for the world.
func (w *World) initChunks() {
	w.ChunkW = (w.Width + w.ChunkSize - 1) / w.ChunkSize
	w.ChunkH = (w.Height + w.ChunkSize - 1) / w.ChunkSize
	w.Chunks = make([]Chunk, w.ChunkW*w.ChunkH)
	for cy := range w.ChunkH {
		for cx := range w.ChunkW {
			idx := cy*w.ChunkW + cx
			w.Chunks[idx] = Chunk{X: cx, Y: cy, Active: true, Sleeping: false}
		}
	}
}

// ChunkIndex returns the flat index of the chunk containing cell (x, y).
func (w *World) ChunkIndex(x, y int) int {
	cx := x / w.ChunkSize
	cy := y / w.ChunkSize
	return cy*w.ChunkW + cx
}

// MarkDirty marks the chunk containing cell (x, y) as dirty.
func (w *World) MarkDirty(x, y int) {
	idx := w.ChunkIndex(x, y)
	w.Chunks[idx].Dirty = true
	w.Chunks[idx].ChangedThisTick = true
}

// ClearDirty resets all dirty flags after network broadcast.
func (w *World) ClearDirty() {
	for i := range w.Chunks {
		w.Chunks[i].Dirty = false
	}
}

// UpdateSleepStates advances sleep tracking for all chunks at the end of a tick.
// Chunks with no changes accumulate idle ticks; those reaching the threshold sleep.
// Chunks that had changes reset their idle counter and wake up.
func (w *World) UpdateSleepStates() {
	for i := range w.Chunks {
		c := &w.Chunks[i]
		if c.ChangedThisTick {
			c.Sleeping = false
			c.IdleTicks = 0
		} else {
			if !c.Sleeping {
				c.IdleTicks++
				if c.IdleTicks >= SleepThreshold {
					c.Sleeping = true
				}
			}
		}
		c.ChangedThisTick = false
	}
}

// WakeChunk wakes the chunk at grid position (cx, cy) if it exists.
func (w *World) WakeChunk(cx, cy int) {
	if cx < 0 || cx >= w.ChunkW || cy < 0 || cy >= w.ChunkH {
		return
	}
	w.Chunks[cy*w.ChunkW+cx].WakeUp()
}

// WakeChunkAt wakes the chunk containing cell (x, y).
func (w *World) WakeChunkAt(x, y int) {
	if x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return
	}
	idx := w.ChunkIndex(x, y)
	w.Chunks[idx].WakeUp()
}

// WakeNeighbors wakes the 8 neighbors of chunk at grid (cx, cy).
func (w *World) WakeNeighbors(cx, cy int) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			w.WakeChunk(cx+dx, cy+dy)
		}
	}
}

// SleepingChunkCount returns how many chunks are currently sleeping (for metrics).
func (w *World) SleepingChunkCount() int {
	count := 0
	for i := range w.Chunks {
		if w.Chunks[i].Sleeping {
			count++
		}
	}
	return count
}

// ActiveChunkCount returns how many chunks are NOT sleeping.
func (w *World) ActiveChunkCount() int {
	return len(w.Chunks) - w.SleepingChunkCount()
}
