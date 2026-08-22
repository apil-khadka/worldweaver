# Multiplayer Protocol — Tasks

- [x] Set up chi router with /ws, /health, /metrics endpoints
- [x] Implement WebSocket upgrade handler using gorilla/websocket
- [x] Create Hub struct with register/unregister/broadcast channels
- [x] Implement Hub.Run() goroutine with select loop
- [x] Create Client struct with buffered send channel (cap 64)
- [x] Implement client read pump (reads messages, validates, enqueues actions)
- [x] Implement client write pump (drains send channel to WebSocket)
- [x] Define protocol opcodes as constants
- [x] Implement WELCOME message encoder (opcode 0x01)
- [x] Implement FULL_SNAPSHOT encoder (opcode 0x02)
- [x] Implement CHUNK_UPDATE encoder (opcode 0x03)
- [x] Implement PLAYER_JOIN / PLAYER_LEAVE broadcast (opcodes 0x04, 0x05)
- [x] Implement PLAYER_STATE broadcast (opcode 0x06)
- [x] Implement ERROR message encoder (opcode 0x07)
- [x] Implement POWER_USE decoder with validation (opcode 0x10)
- [x] Implement CURSOR_MOVE decoder (opcode 0x11)
- [x] Implement PING/PONG (opcodes 0x12, 0x13)
- [x] Add drop-oldest policy to send queue when full
- [x] Add backpressure warning log at 75% queue capacity
- [x] Implement input validation (bounds, material ID, influence budget)
- [x] Add protocol version check on WELCOME; disconnect mismatched clients
- [x] Implement reconnect token (optional session resume)
- [x] Add rate limiting per client (max messages/sec)
- [ ] Write integration test: 10 concurrent clients, verify no data race

## Phase 2 — Multi-World & Scaling

- [x] Implement multi-world routing (hub-per-world, client selects on connect)
- [x] Add cursor broadcast optimization (spatial filtering, only send nearby cursors)
- [ ] Implement world migration (move client between worlds without full reconnect)
- [ ] Add spectator mode (read-only connection, no influence budget)
- [ ] Implement world persistence (snapshot to disk on graceful shutdown)
