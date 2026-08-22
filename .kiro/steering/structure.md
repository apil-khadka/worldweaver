# WorldWeaver — Structure Steering

## Go Package Boundaries

| Package | Owns | Must NOT import |
|---------|------|------------------|
| internal/world | State arrays, chunks, fields | simulation, network, game |
| internal/simulation | Engine, tick loop, cell dispatch | network, transport, config |
| internal/systems | Material registry, definitions | network, transport |
| internal/generation | Terrain, biome, climate | network, game |
| internal/game | Player, influence, powers, stability | network, world (write only via simulation) |
| internal/protocol | Wire format, encoders | transport, simulation |
| internal/network | chi router, WebSocket hub, broadcast | simulation internals |
| internal/transport | (planned) interest management | simulation, game |
| internal/persistence | Snapshot save/load | network, game |
| internal/metrics | Telemetry collection | everything except standard lib |
| internal/config | Central configuration, validation | everything except standard lib |

## TypeScript Module Structure

| Module | Owns | Must NOT import |
|--------|------|------------------|
| src/render/ | IWorldRenderer + backends | simulation logic |
| src/core/ | Protocol constants, shared types | renderer internals |
| src/config/ | Client configuration | server code |
| src/platform/ | BrowserPlatform, TauriPlatform | renderer internals |
| src/design/ | CSS tokens, component styles | network, render |
| src/network/ | WebSocket client | renderer internals |

## Naming Conventions
- Go: standard gofmt, PascalCase exports, camelCase unexported
- TypeScript: PascalCase for types/interfaces/classes, camelCase for functions/variables
- CSS: BEM-like with `ww-` prefix (e.g. `.ww-btn--primary`)
- Files: snake_case for Go, PascalCase for TS classes, kebab-case for CSS

## Test Locations
- Go unit tests: `*_test.go` alongside package
- Integration tests: `tests/`
- Benchmarks: `benchmarks/`
- Frontend: (future) `web/__tests__/`

## Protocol Ownership
- `internal/protocol/` owns ALL wire format definitions
- Message type IDs defined ONCE in protocol/
- Client constants.ts must mirror protocol/ exactly
