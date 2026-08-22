# ADR-002: WebSocket Transport

**Status:** Accepted  
**Date:** 2026-08-22

## Context
The server must stream world state updates to browsers in real-time.

## Decision
Use WebSockets for all client-server communication.

## Alternatives Considered
- WebRTC data channels (complex, peer-oriented)
- Server-Sent Events (unidirectional)
- HTTP polling (high latency)

## Rationale
WebSockets provide native browser support, persistent bidirectional connection, simple Go server implementation (coder/websocket), and are appropriate for real-time game state at 20-30 Hz update rates.

## Consequences
- Single persistent connection per client
- Requires WSS for production
- Strong ecosystem support in both Go and browsers

## References
- WorldWeaver_Full_Project_Documentation.md § 22
