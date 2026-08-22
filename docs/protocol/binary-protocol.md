# WorldWeaver Binary Protocol Specification

**Version:** 1  
**Encoder:** BinaryEncoderV1 (`internal/protocol/encoder.go`)

## Overview

WorldWeaver uses a compact binary protocol for high-frequency world-state updates (chunk deltas at 20 Hz). A DebugJSONEncoder exists for development.

The `UpdateEncoder` interface decouples encoding from transport:
```go
type UpdateEncoder interface {
    EncodeSnapshot(req SnapshotRequest) ([]byte, error)
    EncodeChunks(req ChunksRequest) ([]byte, error)
    Name() string
}
```

## Message Types

| ID | Name | Direction | Description |
|----|------|-----------|-------------|
| 0x01 | HELLO | C→S | Client greeting with viewport size |
| 0x02 | WELCOME | S→C | Server responds with world metadata |
| 0x10 | WORLD_SNAPSHOT | S→C | Full viewport material dump |
| 0x11 | CHUNK_UPDATE | S→C | Dirty chunk deltas |
| 0x20 | PLAYER_STATE | S→C | Influence, power state |
| 0x21 | WORLD_METRICS | S→C | TPS, active cells, stability |
| 0x30 | POWER_INPUT | C→S | Player power application |
| 0x31 | VIEWPORT | C→S | Camera position update |
| 0x40 | ERROR | S→C | Rejected request explanation |
| 0x50 | PING | C→S | Latency measurement |
| 0x51 | PONG | S→C | Ping response |

## WORLD_SNAPSHOT Layout

```
Offset  Type     Field
0       uint8    MsgTypeWorldSnapshot (0x10)
1       uint8    ProtocolVersion
2–9     uint64   tick (LE)
10–13   int32    x (LE)
14–17   int32    y (LE)
18–21   uint32   width (LE)
22–25   uint32   height (LE)
26…     uint8[]  material data (width×height bytes)
```

## CHUNK_UPDATE Layout

```
Offset  Type     Field
0       uint8    MsgTypeChunkUpdate (0x11)
1       uint8    ProtocolVersion
2–9     uint64   tick (LE)
10–11   uint16   numChunks (LE)

For each chunk:
  0–3   int32    cx (chunk grid X, LE)
  4–7   int32    cy (chunk grid Y, LE)
  8–11  uint32   dataLen (LE)
  12…   uint8[]  material data
```

## Snapshot Persistence Format

Binary snapshots saved to disk follow this layout:

```
Magic:     4 bytes  "WWSN"
Version:   2 bytes  uint16 LE (current = 1)
Width:     4 bytes  int32 LE
Height:    4 bytes  int32 LE
Seed:      8 bytes  int64 LE
Tick:      8 bytes  uint64 LE
Reserved: 16 bytes  zeros

Material[]:    width×height bytes
Temperature[]: width×height×2 bytes (int16 LE)
Moisture[]:    width×height bytes
Lifetime[]:    width×height×2 bytes (uint16 LE)
```

## Protocol Evolution

- Protocol version byte in every message enables forward compatibility
- Changing encoder selection does not require simulation or game code changes (ADR-006)
- Future: RLE compression for chunk data, LOD representation

## References

- [ADR-006: Binary Protocol](../decisions/ADR-006-binary-protocol.md)
- Source: `internal/protocol/encoder.go`
