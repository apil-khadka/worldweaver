# Multiplayer Protocol — Design

## Hub Pattern

A central `Hub` struct manages all connected clients:

```go
type Hub struct {
    clients    map[uint32]*Client
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte
    nextID     uint32
}
```

The Hub runs in its own goroutine, selecting on register/unregister/broadcast channels. The simulation calls `hub.Broadcast(chunkData)` after each tick.

## Per-Client Send Queue

Each client has a buffered channel (capacity 64 messages). The write pump drains this channel to the WebSocket:

```go
type Client struct {
    ID       uint32
    conn     *websocket.Conn
    send     chan []byte  // buffered, cap=64
    hub      *Hub
}
```

**Drop-oldest policy:** When `send` is full, the oldest message is dequeued before enqueuing the new one. This ensures the client always gets the most recent state.

## HTTP Router

Using `chi` for HTTP routing:
- `GET /ws` — WebSocket upgrade endpoint
- `GET /health` — Health check (returns 200 + tick count)
- `GET /metrics` — Prometheus-compatible metrics

## Protocol Message Types

| Opcode | Direction | Name | Payload |
|--------|-----------|------|---------|
| 0x01 | S→C | WELCOME | version, player_id, world_w, world_h, tick |
| 0x02 | S→C | FULL_SNAPSHOT | material[], temp[], moisture[], lifetime[] |
| 0x03 | S→C | CHUNK_UPDATE | chunk_x, chunk_y, data[] |
| 0x04 | S→C | PLAYER_JOIN | player_id, name |
| 0x05 | S→C | PLAYER_LEAVE | player_id |
| 0x06 | S→C | PLAYER_STATE | player_id, cursor_x, cursor_y, active_power |
| 0x07 | S→C | ERROR | code, message |
| 0x10 | C→S | POWER_USE | power_id, x, y |
| 0x11 | C→S | CURSOR_MOVE | x, y |
| 0x12 | C→S | PING | timestamp |
| 0x13 | S→C | PONG | timestamp, server_tick |

## BinaryEncoderV1 Layout

All messages use little-endian encoding:
```
[1 byte opcode][2 byte payload_length][payload...]
```

For CHUNK_UPDATE:
```
[1 opcode=0x03][2 len][1 chunk_x][1 chunk_y][32*32 material_bytes]
```

For FULL_SNAPSHOT:
```
[1 opcode=0x02][4 len_u32][W*H material][W*H*2 temp][W*H moisture][W*H*2 lifetime]
```

## Connection Flow (WELCOME)

1. Client opens WebSocket to `/ws`
2. Server upgrades connection, assigns player ID
3. Server sends WELCOME (version=1, player_id, dimensions, current tick)
4. Server sends FULL_SNAPSHOT
5. Client begins rendering; server streams CHUNK_UPDATEs each tick
6. Client sends POWER_USE / CURSOR_MOVE as needed

## Thread Model

- **Hub goroutine:** manages client map, handles register/unregister
- **Per-client read pump:** reads from WebSocket, validates, enqueues to simulation action queue
- **Per-client write pump:** drains `send` channel to WebSocket
- **Simulation goroutine:** calls `hub.BroadcastChunks()` between ticks
