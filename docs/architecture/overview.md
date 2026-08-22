# WorldWeaver Architecture Overview

## Design Philosophy

> The server computes reality. The network transports reality. The GPU visualises reality.

WorldWeaver is a server-authoritative multiplayer simulation where:
- One Go process owns ALL world state
- Multiple thin clients render and send input
- No client ever simulates physics

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     SERVER (Go)                         │
│                                                         │
│  Config → Generation → World State ← Persistence        │
│                           │                              │
│                    Simulation Engine (60 TPS)            │
│                           │                              │
│              Game Rules + Metrics                        │
│                           │                              │
│              Protocol (BinaryV1 / JSON)                  │
│                           │                              │
│              chi HTTP + WebSocket Transport              │
└───────────────────────────┬─────────────────────────────┘
                            │
                       WSS / binary
                            │
┌───────────────────────────┼─────────────────────────────┐
│                     CLIENT (TypeScript)                  │
│                                                         │
│  WebSocket Transport → State Cache → Renderer           │
│                                                         │
│  Input Handler → Power Messages → Server                │
│                                                         │
│  Renderer: WebGL2 | WebGPU | Canvas2D                   │
│  Platform: Browser | Tauri Desktop | Tauri Mobile        │
└─────────────────────────────────────────────────────────┘
```

## Module Boundaries

### Server
| Package | Responsibility | Does NOT know about |
|---------|---------------|--------------------|
| `world` | State storage (arrays, chunks) | simulation rules, network |
| `simulation` | Tick loop, material behaviours | WebSocket, HTTP |
| `systems/materials` | Material registry & definitions | network, game |
| `generation` | Procedural terrain, climate | network, game |
| `game` | Influence, powers, stability | WebSocket, rendering |
| `protocol` | Wire format encoding | transport, simulation |
| `network` | chi router, WebSocket hub | simulation internals |
| `persistence` | Snapshot save/load | network, game |
| `metrics` | Telemetry collection | minimal deps |
| `config` | Configuration & validation | minimal deps |

### Client
| Module | Responsibility | Does NOT know about |
|--------|---------------|--------------------|
| `render/` | GPU rendering (IWorldRenderer) | simulation rules |
| `core/` | Protocol constants, shared types | renderer internals |
| `config/` | Client settings | server code |
| `platform/` | Browser/Tauri adapters | renderer internals |
| `design/` | CSS tokens, component styles | network, render |

## Data Flow

```
Player click → InputHandler → WebSocket msg → Server validation
    → EnqueueAction → Tick processes action → World mutates
    → Dirty chunks marked → Broadcast pipeline encodes
    → WebSocket send → Client state cache update
    → GPU texture sub-upload → Next frame renders
```

## Key Invariants

1. **One writer:** Only the simulation goroutine mutates world state
2. **No blocking:** Slow clients never stall the simulation loop
3. **Newest wins:** If a client's send queue fills, oldest updates are dropped
4. **Server authority:** Client input is a request, not a command
5. **Fixed timestep:** Simulation advances at 60 TPS regardless of client count or FPS

## References

- [ADR-001: Server Authority](../decisions/ADR-001-server-authority.md)
- [ADR-007: chi Router](../decisions/ADR-007-chi-router.md)
- [ADR-008: Modular Simulation](../decisions/ADR-008-modular-simulation.md)
