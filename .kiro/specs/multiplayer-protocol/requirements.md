# Multiplayer Protocol — Requirements

## REQ-NET-001: WebSocket Transport
The server SHALL accept client connections over WebSocket (RFC 6455) on a configurable HTTP port.

**Acceptance:** `wscat -c ws://localhost:PORT/ws` establishes a connection and receives a WELCOME message.

## REQ-NET-002: Server Authority
The server SHALL be the sole authority on world state. Clients send input intentions; the server validates and applies them. Clients never directly modify world state.

**Acceptance:** A crafted client message claiming to place material at an invalid location is rejected; world state unchanged.

## REQ-NET-003: Initial Snapshot
Upon successful connection, the server SHALL send the client a full world snapshot including dimensions, current tick, and all material + environment arrays.

**Acceptance:** New client renders full world within 500ms of connection.

## REQ-NET-004: Incremental Updates
After the initial snapshot, the server SHALL send only changed chunks each tick. Unchanged chunks are not transmitted.

**Acceptance:** Idle world with no player interaction produces zero chunk-update messages.

## REQ-NET-005: Reconnect Support
A disconnected client SHALL be able to reconnect and receive a fresh full snapshot without server restart.

**Acceptance:** Client disconnects, reconnects 5 seconds later, receives valid snapshot and resumes.

## REQ-NET-006: Slow-Client Isolation
A client that cannot keep up with updates SHALL have its send queue trimmed (drop oldest). It must not block other clients or the simulation.

**Acceptance:** Simulated slow client (1 KB/s) does not increase tick duration for other clients.

## REQ-NET-007: Input Validation
All player actions received SHALL be validated: bounds check on coordinates, valid material ID, influence budget check.

**Acceptance:** Out-of-bounds placement request returns error message; no panic.

## REQ-NET-008: Protocol Version Tag
Every WELCOME message SHALL include a protocol version number. Clients with mismatched versions are disconnected with a human-readable reason.

**Acceptance:** Client with version "0" receives disconnect with "protocol version mismatch" message.

## REQ-NET-009: Backpressure Signaling
When a client's send queue exceeds 75% capacity, the server SHALL log a warning. At 100%, oldest messages are dropped.

**Acceptance:** Load test fills queue; warning logged at threshold; no goroutine leak.

## REQ-NET-010: No Simulation Blocking
Network operations (accept, send, receive) SHALL never block the simulation goroutine. All network I/O happens in separate goroutines communicating via channels.

**Acceptance:** Killing all client connections mid-tick does not affect tick duration.

## References
- WorldWeaver_Full_Project_Documentation.md § 23 (Multiplayer Architecture)
- WorldWeaver_Full_Project_Documentation.md § 24 (Protocol Messages)
- WorldWeaver_Full_Project_Documentation.md § 25 (Binary Encoding)
- WorldWeaver_Full_Project_Documentation.md § 26 (Backpressure)
