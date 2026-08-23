package network

import (
	"encoding/base64"
	"encoding/json"

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/world"
)

// BroadcastChunkUpdates sends all dirty chunks to all connected clients.
// Must run on the simulation goroutine (via the engine's post-tick hook):
// it reads world state, which belongs to the tick.
//
// Design invariant: this function must never block the simulation goroutine.
// All writes go into per-client send queues that are drained asynchronously.
// Client sends happen under the hub read lock so a client unregistering
// cannot race with delivery.
func (h *Hub) BroadcastChunkUpdates(w *world.World) {
	// Collect dirty chunks before touching the client map; this reads the
	// world and is serialized with the tick by the caller.
	updates := buildChunkUpdates(w)
	if len(updates) == 0 {
		w.ClearDirty()
		return
	}

	msg, _ := json.Marshal(ChunkUpdateMsg{
		Type:   MsgChunkUpdate,
		Tick:   w.Tick.Load(),
		Chunks: updates,
	})

	var totalBytes int64
	h.mu.RLock()
	for c := range h.clients {
		c.sendRaw(msg)
		totalBytes += int64(len(msg))
	}
	h.mu.RUnlock()

	// Record outbound bytes for metrics
	if h.metrics != nil {
		h.metrics.RecordOutbound(totalBytes)
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
	x := int(cx)
	y := int(cy)
	ww := int(cw)
	wh := int(ch)

	// The client may not have reported its viewport yet (the HELLO/VIEWPORT
	// message is processed by the read pump, which starts after this call).
	// In that case send the whole world so the client always has data to
	// render instead of an empty snapshot.
	if ww <= 0 || wh <= 0 {
		x, y = 0, 0
		ww, wh = w.Width, w.Height
	}

	// Clamp viewport to world bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= w.Width || y >= w.Height {
		x, y = 0, 0
		ww, wh = w.Width, w.Height
	}
	if x+ww > w.Width {
		ww = w.Width - x
	}
	if y+wh > w.Height {
		wh = w.Height - y
	}
	if ww <= 0 || wh <= 0 {
		return
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
		"tick": w.Tick.Load(),
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
			Tick: w.Tick.Load(),
			Data: data,
		})
	}
	return entries
}
