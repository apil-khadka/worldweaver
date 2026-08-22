package network

import (
	"encoding/base64"
	"encoding/json"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/world"
)

// BroadcastChunkUpdates sends all dirty chunks to all connected clients.
// It is called by the simulation loop after each tick (or every N ticks).
//
// Design invariant: this function must never block the simulation goroutine.
// All writes go into per-client send queues that are drained asynchronously.
func (h *Hub) BroadcastChunkUpdates(w *world.World) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	if len(clients) == 0 {
		w.ClearDirty()
		return
	}

	// Collect dirty chunks
	updates := buildChunkUpdates(w)
	if len(updates) == 0 {
		return
	}

	msg, _ := json.Marshal(ChunkUpdateMsg{
		Type:   MsgChunkUpdate,
		Tick:   w.Tick,
		Chunks: updates,
	})

	var totalBytes int64
	for _, c := range clients {
		c.sendRaw(msg)
		totalBytes += int64(len(msg))
	}

	// Record outbound bytes for metrics
	if h.metrics != nil {
		h.metrics.OutboundBPS.Add(totalBytes)
	}

	w.ClearDirty()
}

// BroadcastMetrics sends a WorldMetricsMsg to all connected clients.
func (h *Hub) BroadcastMetrics(snap metrics.Snapshot, stability float32, tick uint64) {
	msg, _ := json.Marshal(WorldMetricsMsg{
		Type:         MsgWorldMetrics,
		Tick:         tick,
		TPS:          snap.TPS,
		TickP95Ms:    snap.TickP95Ms,
		ActiveCells:  snap.ActiveCells,
		ActiveChunks: snap.ActiveChunks,
		PlayerCount:  snap.PlayerCount,
		OutboundBPS:  snap.OutboundBPS,
		Stability:    stability,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.sendRaw(msg)
	}
}

// SendFullSnapshot sends a full world snapshot to a single client.
// Used when a client first connects or after a reconnect.
func SendFullSnapshot(c *Client, w *world.World) {
	cx, cy, cw, ch := c.Player.Camera()
	// Clamp viewport to world bounds
	x := int(cx)
	y := int(cy)
	ww := int(cw)
	wh := int(ch)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+ww > w.Width {
		ww = w.Width - x
	}
	if y+wh > w.Height {
		wh = w.Height - y
	}

	data := make([]byte, ww*wh)
	for row := range wh {
		for col := range ww {
			data[row*ww+col] = w.Material[(y+row)*w.Width+(x+col)]
		}
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	c.sendJSON(map[string]any{
		"type": MsgWorldSnapshot,
		"tick": w.Tick,
		"x":    x,
		"y":    y,
		"w":    ww,
		"h":    wh,
		"data": encoded,
	})
}

// buildChunkUpdates returns serialized data for each dirty chunk.
func buildChunkUpdates(w *world.World) []ChunkUpdateEntry {
	var entries []ChunkUpdateEntry
	cs := w.ChunkSize
	for i := range w.Chunks {
		if !w.Chunks[i].Dirty {
			continue
		}
		chunk := &w.Chunks[i]
		cx0 := chunk.CellX(cs)
		cy0 := chunk.CellY(cs)

		// Clamp chunk to world bounds
		cxEnd := cx0 + cs
		cyEnd := cy0 + cs
		if cxEnd > w.Width {
			cxEnd = w.Width
		}
		if cyEnd > w.Height {
			cyEnd = w.Height
		}

		data := make([]byte, (cxEnd-cx0)*(cyEnd-cy0))
		idx := 0
		for y := cy0; y < cyEnd; y++ {
			for x := cx0; x < cxEnd; x++ {
				data[idx] = w.Material[y*w.Width+x]
				idx++
			}
		}
		entries = append(entries, ChunkUpdateEntry{
			CX:   chunk.X,
			CY:   chunk.Y,
			Tick: w.Tick,
			Data: data,
		})
	}
	return entries
}
