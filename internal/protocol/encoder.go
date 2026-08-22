// Package protocol defines the WorldWeaver wire format.
//
// # Architecture
//
// Protocol is strictly separated from Transport (internal/transport/).
// Protocol owns message schemas and encoding.
// Transport owns connections, client lifecycle, and delivery.
//
// # Versioning
//
// Every snapshot and delta includes a protocol version byte so that
// browser, Tauri desktop, and Tauri mobile clients running different
// builds can be detected and handled gracefully.
//
// # Encoding implementations
//
//   - BinaryEncoderV1  — compact binary, suitable for production
//   - DebugJSONEncoder — human-readable, suitable for development
//
// The active encoder is chosen at startup via config and injected
// into the transport layer.  Changing encoding does not require
// modifying simulation or game code (ADR-008).
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/worldweaver/worldweaver/internal/world"
)

// ProtocolVersion is the current wire-format version.
// Increment when the binary layout changes in a breaking way.
const ProtocolVersion uint8 = 1

// MessageType discriminates top-level messages.
type MessageType uint8

const (
	MsgTypeHello         MessageType = 0x01
	MsgTypeWelcome       MessageType = 0x02
	MsgTypeWorldSnapshot MessageType = 0x10
	MsgTypeChunkUpdate   MessageType = 0x11
	MsgTypePlayerState   MessageType = 0x20
	MsgTypeWorldMetrics  MessageType = 0x21
	MsgTypePowerInput    MessageType = 0x30
	MsgTypeViewport      MessageType = 0x31
	MsgTypeError         MessageType = 0x40
	MsgTypePing          MessageType = 0x50
	MsgTypePong          MessageType = 0x51
)

// SnapshotRequest carries the parameters needed to encode a full snapshot.
type SnapshotRequest struct {
	W     *world.World
	X, Y  int
	Width int
	Height int
	Tick  uint64
}

// ChunksRequest carries the dirty chunk list for delta encoding.
type ChunksRequest struct {
	W      *world.World
	Chunks []world.Chunk
	Tick   uint64
}

// UpdateEncoder is the interface all encoding implementations must satisfy.
//
// A change from BinaryEncoderV1 → BinaryEncoderV2 must not require
// changes in simulation, game, or transport packages (ADR-008 / section 61).
type UpdateEncoder interface {
	// EncodeSnapshot produces a full viewport snapshot payload.
	EncodeSnapshot(req SnapshotRequest) ([]byte, error)
	// EncodeChunks produces a dirty-chunk delta payload.
	EncodeChunks(req ChunksRequest) ([]byte, error)
	// Name returns an identifier used in metrics and logs.
	Name() string
}

// ─── BinaryEncoderV1 ────────────────────────────────────────────────────────

// BinaryEncoderV1 encodes world updates as compact binary messages.
//
// # Snapshot layout
//
//	byte  0       MsgTypeWorldSnapshot
//	byte  1       ProtocolVersion
//	bytes 2-9     tick       uint64 LE
//	bytes 10-13   x          int32  LE
//	bytes 14-17   y          int32  LE
//	bytes 18-21   w          uint32 LE
//	bytes 22-25   h          uint32 LE
//	bytes 26…     material   []uint8  (w*h bytes)
//
// # ChunkUpdate layout
//
//	byte  0       MsgTypeChunkUpdate
//	byte  1       ProtocolVersion
//	bytes 2-9     tick       uint64 LE
//	bytes 10-11   numChunks  uint16 LE
//	for each chunk:
//	  bytes 0-3   cx         int32  LE
//	  bytes 4-7   cy         int32  LE
//	  bytes 8-11  dataLen    uint32 LE
//	  bytes 12…   data       []uint8
type BinaryEncoderV1 struct{}

func (BinaryEncoderV1) Name() string { return "BinaryV1" }

func (BinaryEncoderV1) EncodeSnapshot(req SnapshotRequest) ([]byte, error) {
	w := req.W
	size := req.Width * req.Height
	buf := make([]byte, 26+size)
	le := binary.LittleEndian

	buf[0] = byte(MsgTypeWorldSnapshot)
	buf[1] = ProtocolVersion
	le.PutUint64(buf[2:], req.Tick)
	le.PutUint32(buf[10:], uint32(req.X))
	le.PutUint32(buf[14:], uint32(req.Y))
	le.PutUint32(buf[18:], uint32(req.Width))
	le.PutUint32(buf[22:], uint32(req.Height))

	out := buf[26:]
	for row := range req.Height {
		for col := range req.Width {
			wx := req.X + col
			wy := req.Y + row
			i := w.Index(wx, wy)
			if i >= 0 {
				out[row*req.Width+col] = w.Material[i]
			}
		}
	}
	return buf, nil
}

func (BinaryEncoderV1) EncodeChunks(req ChunksRequest) ([]byte, error) {
	w := req.W
	cs := w.ChunkSize

	// Pre-calculate total size
	totalData := 0
	var dirty []world.Chunk
	for _, ch := range req.Chunks {
		if !ch.Dirty {
			continue
		}
		cx0 := ch.CellX(cs)
		cy0 := ch.CellY(cs)
		cxEnd := cx0 + cs
		if cxEnd > w.Width { cxEnd = w.Width }
		cyEnd := cy0 + cs
		if cyEnd > w.Height { cyEnd = w.Height }
		totalData += (cxEnd - cx0) * (cyEnd - cy0)
		dirty = append(dirty, ch)
	}
	if len(dirty) == 0 {
		return nil, nil
	}

	// header(10) + numChunks(2) + per-chunk-header(12)*N + data
	buf := make([]byte, 0, 12+len(dirty)*12+totalData)
	le := binary.LittleEndian
	var hdr [12]byte
	hdr[0] = byte(MsgTypeChunkUpdate)
	hdr[1] = ProtocolVersion
	le.PutUint64(hdr[2:], req.Tick)
	buf = append(buf, hdr[:10]...)
	var nc [2]byte
	le.PutUint16(nc[:], uint16(len(dirty)))
	buf = append(buf, nc[:]...)

	for _, ch := range dirty {
		cx0 := ch.CellX(cs)
		cy0 := ch.CellY(cs)
		cxEnd := cx0 + cs
		if cxEnd > w.Width { cxEnd = w.Width }
		cyEnd := cy0 + cs
		if cyEnd > w.Height { cyEnd = w.Height }
		dataLen := (cxEnd - cx0) * (cyEnd - cy0)

		var chdr [12]byte
		le.PutUint32(chdr[0:], uint32(ch.X))
		le.PutUint32(chdr[4:], uint32(ch.Y))
		le.PutUint32(chdr[8:], uint32(dataLen))
		buf = append(buf, chdr[:]...)

		data := make([]byte, dataLen)
		idx := 0
		for y := cy0; y < cyEnd; y++ {
			for x := cx0; x < cxEnd; x++ {
				data[idx] = w.Material[y*w.Width+x]
				idx++
			}
		}
		buf = append(buf, data...)
	}
	return buf, nil
}

// ─── DebugJSONEncoder ────────────────────────────────────────────────────────

// DebugJSONEncoder produces human-readable JSON messages for development and
// browser console inspection.  It is NOT suitable for production use due to
// ~5–10× higher bandwidth than BinaryEncoderV1.
type DebugJSONEncoder struct{}

func (DebugJSONEncoder) Name() string { return "DebugJSON" }

func (DebugJSONEncoder) EncodeSnapshot(req SnapshotRequest) ([]byte, error) {
	w := req.W
	data := make([]byte, req.Width*req.Height)
	for row := range req.Height {
		for col := range req.Width {
			i := w.Index(req.X+col, req.Y+row)
			if i >= 0 {
				data[row*req.Width+col] = w.Material[i]
			}
		}
	}
	return json.Marshal(map[string]any{
		"type": "world_snapshot",
		"ver":  ProtocolVersion,
		"tick": req.Tick,
		"x":    req.X,
		"y":    req.Y,
		"w":    req.Width,
		"h":    req.Height,
		"data": data,
	})
}

func (DebugJSONEncoder) EncodeChunks(req ChunksRequest) ([]byte, error) {
	w := req.W
	cs := w.ChunkSize
	type entry struct {
		CX   int    `json:"cx"`
		CY   int    `json:"cy"`
		Tick uint64 `json:"tick"`
		Data []byte `json:"data"`
	}
	var entries []entry
	for _, ch := range req.Chunks {
		if !ch.Dirty {
			continue
		}
		cx0 := ch.CellX(cs)
		cy0 := ch.CellY(cs)
		cxEnd := cx0 + cs; if cxEnd > w.Width { cxEnd = w.Width }
		cyEnd := cy0 + cs; if cyEnd > w.Height { cyEnd = w.Height }
		data := make([]byte, 0, (cxEnd-cx0)*(cyEnd-cy0))
		for y := cy0; y < cyEnd; y++ {
			for x := cx0; x < cxEnd; x++ {
				data = append(data, w.Material[y*w.Width+x])
			}
		}
		entries = append(entries, entry{ch.X, ch.Y, req.Tick, data})
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return json.Marshal(map[string]any{
		"type":   "chunk_update",
		"ver":    ProtocolVersion,
		"tick":   req.Tick,
		"chunks": entries,
	})
}

// NewEncoder returns the encoder matching the given name.
func NewEncoder(name string) (UpdateEncoder, error) {
	switch name {
	case "binary", "BinaryV1":
		return BinaryEncoderV1{}, nil
	case "json", "DebugJSON":
		return DebugJSONEncoder{}, nil
	default:
		return nil, fmt.Errorf("unknown encoder %q (valid: binary, json)", name)
	}
}
