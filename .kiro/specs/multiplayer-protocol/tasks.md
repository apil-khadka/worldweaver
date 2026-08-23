# Multiplayer Protocol — Tasks

## Phase 1 — Core Networking

- [x] Set up chi router with /ws, /health, /api/metrics, /api/login, /api/worlds, /api/scores endpoints
- [x] Implement WebSocket upgrade handler using coder/websocket
- [x] Create Hub struct with register/unregister/broadcast channels
- [x] Implement Hub.Run() goroutine with client management
- [x] Create Client struct with buffered send channel
- [x] Implement client read pump (reads messages, validates, enqueues actions)
- [x] Implement client write pump (drains send channel to WebSocket)
- [x] Define JSON message types (welcome, world_snapshot, chunk_update, player_state, world_metrics, error, etc.)
- [x] Implement WELCOME message with playerID + world dimensions
- [x] Implement FULL_SNAPSHOT encoder (base64-encoded material array)
- [x] Implement CHUNK_UPDATE encoder (array of dirty chunks, base64 data per chunk)
- [x] Implement PLAYER_JOIN / PLAYER_LEAVE broadcast
- [x] Implement PLAYER_STATE broadcast (influence, level, score)
- [x] Implement ERROR message encoder
- [x] Implement POWER_USE decoder with validation (bounds, budget, power ID)
- [x] Implement CURSOR_MOVE broadcast (multiplayer cursor presence)
- [x] Implement PING/PONG for RTT measurement
- [x] Add connection rate limiting (20/min per IP)
- [x] Implement input validation (bounds, power ID, influence budget)
- [x] Add per-player message rate limiting (chat: 5/10s)
- [ ] Write integration test: 10 concurrent clients, verify no data race (partially covered by TestE2EConcurrentLoad)

## Phase 2 — Multi-World & Social

- [x] Implement multi-world routing (world manager, client selects on connect via ?world= query param)
- [x] Add REST API for world creation/deletion with auth
- [x] Implement cursor broadcast (send other players' cursors in real-time)
- [x] Implement multiplayer chat (text messages with world position)
- [x] Implement location pings (Shift+Click ping visible to all)
- [x] Implement emote system (6 emotes, broadcast to nearby players)
- [x] Implement power combo detection (2+ players, different powers, within 32 cells, 500ms window)
- [x] Implement cooperative goals (4 goal types, rotate every 5 min, BFS-based detection)
- [ ] Implement world migration (move client between worlds without full reconnect)
- [ ] Add spectator mode (read-only connection, no influence budget)

## Phase 3 — Binary Protocol (Future)

- [ ] Migrate from JSON+base64 to raw binary WebSocket frames for chunk updates
- [ ] Implement binary opcode-based protocol (per ADR-006)
- [ ] Add RLE compression for chunk data
- [ ] Implement interest-managed viewport-based chunk filtering
