package world

// ChunkState represents the activity state of a simulation chunk.
type ChunkState uint8

const (
	ChunkInactive ChunkState = iota
	ChunkActive
	ChunkDirty   // has pending network updates
)

// Chunk is a rectangular sub-region of the world used for:
//   - active-region optimization (skip idle chunks)
//   - dirty-region tracking for network updates
//   - future persistence partitioning
type Chunk struct {
	X, Y   int        // chunk grid coordinates (not cell coordinates)
	State  ChunkState
	Dirty  bool       // true if material changed this tick
	Active bool       // true if simulation should process this chunk
}

// CellX returns the first cell X coordinate of this chunk.
func (c *Chunk) CellX(chunkSize int) int { return c.X * chunkSize }

// CellY returns the first cell Y coordinate of this chunk.
func (c *Chunk) CellY(chunkSize int) int { return c.Y * chunkSize }

// initChunks creates the chunk grid for the world.
func (w *World) initChunks() {
	w.ChunkW = (w.Width + w.ChunkSize - 1) / w.ChunkSize
	w.ChunkH = (w.Height + w.ChunkSize - 1) / w.ChunkSize
	w.Chunks = make([]Chunk, w.ChunkW*w.ChunkH)
	for cy := range w.ChunkH {
		for cx := range w.ChunkW {
			idx := cy*w.ChunkW + cx
			w.Chunks[idx] = Chunk{X: cx, Y: cy, Active: true}
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
}

// ClearDirty resets all dirty flags after network broadcast.
func (w *World) ClearDirty() {
	for i := range w.Chunks {
		w.Chunks[i].Dirty = false
	}
}
