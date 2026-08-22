# WorldWeaver — Technical Steering

## Language & Runtime
- **Server:** Go 1.22+ (single authoritative process)
- **HTTP Router:** go-chi/chi v5 (idiomatic, stdlib-compatible)
- **WebSocket:** coder/websocket (nhooyr fork, modern Go API)
- **Client:** TypeScript (strict mode) + Vite
- **Desktop/Mobile:** Tauri v2 (Rust shell)

## Architecture Invariants
- Server is the SINGLE source of truth for all world state
- Clients NEVER simulate world physics
- Fixed 60 TPS simulation timestep, independent of render FPS
- Network update rate: 20 Hz (configurable)
- Simulation MUST NOT import WebSocket, HTTP, or browser code
- Renderer MUST NOT import simulation rules
- Protocol MUST NOT dictate simulation timing

## Renderer Stack
- **Primary:** WebGL2 (R8UI material texture + GLSL fragment shader)
- **Optional:** WebGPU (WGSL, selected only when adapter succeeds)
- **Fallback:** Canvas2D (ImageData, debug/reference only)
- Runtime selection: WebGPU → WebGL2 → Canvas2D

## Protocol
- Production: BinaryEncoderV1 (compact, version-tagged)
- Development: DebugJSONEncoder (human-readable)
- UpdateEncoder interface allows future evolution without simulation changes

## Modular Systems
- Multi-rate scheduler: material 60Hz, fire 30Hz, plants 5Hz, stability 2Hz
- Material Registry (data-driven, not switch-statement-heavy)
- Active chunk scheduling (sleep idle regions)

## Performance Requirements
- All claims must be backed by go test -bench output
- Use pprof for CPU/heap profiling
- Never pre-fill benchmark tables
- Target: 1M cells at 60 TPS on modern hardware

## Testing Requirements
- Simulation testable without WebSocket
- Protocol testable without renderer
- Renderer testable without live server
- Acceptance tests for: sand falls, water flows, fire burns, rock is immovable

## Dependencies
- Prefer well-maintained, pinned-version dependencies
- go-chi/chi v5.3.2, coder/websocket v1.8.15
- TypeScript 5.5.4, Vite 5.4.2
