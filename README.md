<div align="center">

# 🌍 WorldWeaver

### *One World. Many Forces. Emergent Reality.*

[![CI](https://github.com/apil-khadka/worldweaver/actions/workflows/ci.yml/badge.svg)](https://github.com/apil-khadka/worldweaver/actions/workflows/ci.yml)
[![Deploy](https://github.com/apil-khadka/worldweaver/actions/workflows/deploy.yml/badge.svg)](https://github.com/apil-khadka/worldweaver/actions/workflows/deploy.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)](https://go.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.5-3178C6?logo=typescript)](https://typescriptlang.org)
[![Tauri](https://img.shields.io/badge/Tauri-2.x-FFC131?logo=tauri)](https://tauri.app)

</div>

---

## What Is WorldWeaver?

WorldWeaver is a **real-time multiplayer emergent world simulation** where players shape a living ecosystem through elemental forces — Rain, Heat, Wind, Growth, and Life. A fully authoritative Go server runs physics at 60 TPS while browser clients render a gorgeous isometric 2.5D world via WebGL2. When multiple players wield forces simultaneously, reality emerges: rainstorms extinguish fires, heat evaporates oceans into clouds that drift and rain elsewhere, rivers carve canyons through hydraulic erosion, herbivores graze on forests while predators stalk them in a Lotka-Volterra population cycle.

**This isn't a game with scripted events. It's a world with physics. Everything emerges.**

---

## 🎮 Live Demo

> **[▶ Play WorldWeaver Now](https://worldweaver.apilkhadka.com.np/)**
>
> Open in two browser tabs to experience multiplayer. No signup required — just pick a nickname and start shaping the world.

| Service | URL |
|---------|-----|
| Frontend (Game) | https://worldweaver.apilkhadka.com.np/ |
| Backend (API) | https://worldweaverapi.apilkhadka.com.np/ |
| Repository | https://github.com/apil-khadka/worldweaver |

---

## ✨ Key Features

### 🧪 Simulation & Physics

- **15 Material Types** — Rock, Soil, Sand, Water, Plant, Fire, Smoke, Vapor, Ice, Oil, Lava, Cloud, Air, Herbivore, Predator — each with unique physics rules
- **Lotka-Volterra Ecosystem** — Herbivores eat plants and multiply; predators hunt herbivores. Population dynamics emerge naturally from simple interaction rules
- **Full Weather Cycle** — Water evaporates → rises as vapor → condenses into clouds → precipitates as rain → floods lowlands. All driven by temperature differentials
- **Hydraulic Erosion** — Rivers carve terrain over time, depositing sediment downstream. Mountains erode, valleys form, deltas build
- **Multi-Rate Simulation** — Materials at 60 Hz, fire at 30 Hz, plants at 5 Hz, creatures at 10 Hz, weather at 2 Hz. Each system runs at its natural frequency
- **Emergent Material Interactions** — Water flows downhill, sand falls, fire spreads to plants, lava melts ice, oil floats on water. No scripted events — all behavior emerges from simple rules

### 🎮 Player Experience

- **5 Elemental Powers** — Rain (spawn water), Heat (ignite/evaporate), Wind (push materials), Growth (accelerate plants), Life (spawn creatures)
- **Multiplayer Cursor Presence** — See other players' cursors and active powers in real-time
- **Client-Side Prediction** — Instant visual feedback on power use before server confirmation arrives
- **Scoring & Leaderboard** — Actions earn points; compete for ecosystem influence on the live leaderboard
- **Nickname-Based Login** — No accounts, no passwords. Pick a name and play
- **Multi-World Support** — Create, join, and list multiple independent worlds

### 🎨 Rendering & Effects

- **Isometric 2.5D Rendering** — WebGL2 isometric projection gives the flat grid visual depth and terrain height
- **Cosmetic Particle Effects** — Rain droplets, fire embers, smoke wisps, water splashes, floating leaves — all GPU-accelerated
- **Procedural Sound Effects** — Web Audio API generates real-time audio: crackling fire, flowing water, wind gusts, rain ambience
- **Adaptive Renderer** — Runtime selects WebGPU → WebGL2 → Canvas2D based on hardware capability

### 🏗️ Infrastructure

- **Server-Authoritative Physics** — One Go process owns all state. Clients are pure renderers. No desync, no cheating
- **Rate Limiting & Security** — Per-IP and per-player rate limits prevent abuse; input validation on all power actions
- **Persistent World** — Binary snapshot system saves and restores full world state. Worlds survive server restarts
- **Tauri Desktop/Mobile Packaging** — Ship as a native app on Windows, macOS, Linux, iOS, and Android via Tauri v2
- **Comprehensive E2E Test Suite** — 26 end-to-end tests covering the full multiplayer lifecycle
- **Zero External APIs** — Pure computation. No databases, no third-party services, no API keys, no recurring costs

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Go Authoritative Server                             │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │  Simulation  │  │   Weather    │  │  Ecosystem   │  │   Erosion    │   │
│  │  Engine      │  │   Cycle      │  │  (Lotka-     │  │  (Hydraulic  │   │
│  │  60 TPS      │  │  Evap/Cloud/ │  │   Volterra)  │  │   carving)   │   │
│  │  15 materials│  │  Rain/Flood  │  │  Herb+Pred   │  │              │   │
│  └──────┬───────┘  └──────────────┘  └──────────────┘  └──────────────┘   │
│         │                                                                   │
│  ┌──────┴───────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │  Multi-World │  │   Scoring    │  │   Player     │  │  Persistence │   │
│  │  Manager     │  │   & Leader-  │  │   Auth &     │  │  (Binary     │   │
│  │  Create/Join │  │   board      │  │   Rate Limit │  │   Snapshots) │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │          WebSocket Hub (chi + coder/websocket)                        │   │
│  │          Binary Protocol @ 20 Hz broadcast + cursor presence          │   │
│  └──────────────────────────┬───────────────────────────────────────────┘   │
└─────────────────────────────┼───────────────────────────────────────────────┘
                              │ WebSocket (Binary Frames)
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│   Browser/Tauri  │  │   Browser/Tauri  │  │   Browser/Tauri  │
│                  │  │                  │  │                  │
│ • Isometric 2.5D │  │ • WebGL2 palette │  │ • Particle FX    │
│ • Particle FX    │  │ • Client predict │  │ • Procedural SFX │
│ • Procedural SFX │  │ • Cursor presence│  │ • Lobby + scores │
│ • Leaderboard UI │  │ • Minimap        │  │ • Minimap        │
└──────────────────┘  └──────────────────┘  └──────────────────┘
         Render Only — No Physics — Input + Prediction Only
```

---

## 🔧 How Kiro Was Used

WorldWeaver was built entirely using **Kiro's spec-driven development workflow**. Every feature — from the initial simulation engine through the full ecosystem, weather cycle, erosion, multiplayer scoring, and Tauri packaging — followed the same rigorous pipeline: **Requirements → Design → Tasks → Implementation**, with automated hooks enforcing quality at every step.

### Steering Files (`.kiro/steering/`)

Three steering documents guided Kiro throughout development:

| File | Purpose |
|------|---------|
| [`product.md`](.kiro/steering/product.md) | Vision, product thesis, non-goals, principles ("The server owns the universe", "Players manipulate forces, not cells", "Emergence > feature count") |
| [`tech.md`](.kiro/steering/tech.md) | Architecture invariants (server-authoritative, 60 TPS fixed timestep, renderer isolation), dependency versions, performance requirements |
| [`structure.md`](.kiro/steering/structure.md) | Package boundaries with explicit import restrictions — simulation MUST NOT import network, renderer MUST NOT import simulation rules |

### Specs (`.kiro/specs/`) — 6 Full Specifications

Each spec contains three documents — `requirements.md`, `design.md`, `tasks.md` — forming a complete engineering blueprint:

| Spec | What It Defines |
|------|-----------------|
| [`simulation-core`](.kiro/specs/simulation-core/) | Fixed-timestep engine, multi-rate scheduler, cell dispatch loop |
| [`material-system`](.kiro/specs/material-system/) | Material registry, 15 material types, interaction rules (water flows, sand falls, fire spreads, lava melts ice, creatures hunt) |
| [`multiplayer-protocol`](.kiro/specs/multiplayer-protocol/) | WebSocket lifecycle, binary encoding, 20 Hz broadcast, cursor presence, multi-world routing |
| [`player-powers`](.kiro/specs/player-powers/) | 5 powers (Rain/Heat/Wind/Growth/Life), influence economy, radius/intensity limits, server enforcement |
| [`world-persistence`](.kiro/specs/world-persistence/) | Binary snapshot format, auto-save interval, restore on startup, multi-world persistence |
| [`performance-benchmark`](.kiro/specs/performance-benchmark/) | Reproducible benchmarks, world size scaling, TPS/memory targets |

### The Spec-Driven Workflow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Requirements   │────▶│     Design      │────▶│     Tasks       │────▶│ Implementation  │
│                 │     │                 │     │                 │     │                 │
│ • User stories  │     │ • Architecture  │     │ • Ordered steps │     │ • Code + tests  │
│ • Acceptance    │     │ • Interfaces    │     │ • Checkboxes    │     │ • Hook-verified │
│   criteria      │     │ • Data flow     │     │ • Dependencies  │     │ • CI-validated  │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Hooks (`.kiro/hooks/`) — 4 Automated Quality Gates

| Hook | Trigger | Action |
|------|---------|--------|
| [`post-save-go`](.kiro/hooks/post-save-go.yaml) | Any `.go` file saved | `gofmt -w` + `go vet ./...` |
| [`post-save-ts`](.kiro/hooks/post-save-ts.yaml) | Any `.ts` file saved | `npx tsc --noEmit` (strict typecheck) |
| [`post-task-test`](.kiro/hooks/post-task-test.yaml) | Simulation/protocol task completed | `go test ./... -timeout 60s` |
| [`benchmark-check`](.kiro/hooks/benchmark-check.yaml) | Performance task completed | `go test -bench=. -benchmem ./benchmarks/...` |

### Git Hook — AI Co-Author Attribution

A [`prepare-commit-msg`](.githooks/prepare-commit-msg) hook automatically appends co-author attribution to every commit:

```
Co-authored-by: Kiro (AI) <kiro-dev@amazon.com>
```

### What Kiro Delivered (by the numbers)

| Metric | Value |
|--------|-------|
| Materials simulated | 15 |
| Kiro specs authored | 6 (18 documents) |
| E2E tests | 26 |
| Hooks enforcing quality | 4 |
| Lines of Go (server) | ~4,000+ |
| Lines of TypeScript (client) | ~3,500+ |
| External API dependencies | 0 |

### Why This Matters

Kiro wasn't used as a code autocomplete. It was the **engineering process itself**:
- Steering files prevented scope creep and kept the vision coherent across dozens of features
- Specs ensured every subsystem — from basic sand physics to the full Lotka-Volterra ecosystem — was designed before being coded
- Hooks caught regressions immediately — not in CI minutes later
- The structure steering enforced clean package boundaries that made the codebase maintainable even as it grew from 5 materials to 15 with weather, erosion, and creatures

---

## 🛠️ Tech Stack

| Layer | Technology | Why |
|-------|------------|-----|
| Server | Go 1.27 | Single-binary, goroutine-per-client, zero GC pressure with value types |
| Router | go-chi/chi v5 | Idiomatic, stdlib-compatible, middleware composable |
| WebSocket | coder/websocket | Modern Go API, nhooyr fork with fixes |
| Client | TypeScript 5.5 + Vite | Strict types, fast HMR, tree-shaking |
| Renderer | WebGL2 (isometric 2.5D) | R8UI texture + GLSL palette shader for 60fps material rendering |
| Audio | Web Audio API | Procedural sound synthesis — no asset files needed |
| Particles | Canvas2D overlay | GPU-friendly cosmetic effects (rain, embers, smoke, leaves) |
| Desktop/Mobile | Tauri v2 (Rust) | Native packaging for all platforms from one codebase |
| Containers | Docker (multi-stage) | Minimal Alpine images, reproducible builds |
| CI/CD | GitHub Actions | Test + deploy on every push to main |
| Hosting | Dokploy (self-hosted) | Docker-native, webhook-triggered deploys |

---

## 🚀 Quick Start

### Prerequisites

- Go 1.27+ ([install](https://go.dev/dl/))
- Node.js 20+ ([install](https://nodejs.org/))

### Run Locally

```bash
# Clone
git clone https://github.com/apil-khadka/worldweaver.git
cd worldweaver

# Start the server (serves frontend at :8080)
go run ./cmd/server

# Open in browser
open http://localhost:8080
```

Open in **two tabs** to see multiplayer in action.

### Frontend Development (Hot Reload)

```bash
cd web
npm install
npm run dev    # Vite dev server with HMR
```

### Desktop App (Tauri)

```bash
# Prerequisites: Rust toolchain
cd src-tauri
cargo tauri dev     # Development mode
cargo tauri build   # Production binary
```

---

## 🐳 Docker Deployment

### Docker Compose (Full Stack)

```bash
docker compose up --build
```

This starts both services:
- Backend on internal port 8080
- Frontend on port 80 (proxies `/ws` to backend)

### Individual Services

```bash
# Backend only
docker build -f Dockerfile.backend -t worldweaver-backend .
docker run -p 8080:8080 -v world-data:/app/data worldweaver-backend

# Frontend only
docker build -f Dockerfile.frontend -t worldweaver-frontend .
docker run -p 80:80 -e BACKEND_URL=http://host.docker.internal:8080 worldweaver-frontend
```

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# E2E tests (full multiplayer lifecycle — 26 tests)
go test ./tests/... -v

# Integration tests
go test ./tests/integration_test.go

# Race condition detection
go test -race ./...

# Benchmarks (simulation performance)
go test -bench=. -benchmem ./benchmarks/...
```

### What Tests Cover

| Suite | Tests | Validates |
|-------|-------|-----------|
| `tests/e2e_test.go` | 26 | Full multiplayer lifecycle — login, world creation, power use, scoring, presence, multi-world |
| `tests/integration_test.go` | — | Server lifecycle — WebSocket connect, receive world state, reconnection |
| `tests/simulation_test.go` | — | Material physics — sand falls, water flows, fire spreads, erosion carves, creatures hunt |
| `benchmarks/simulation_bench_test.go` | — | TPS at scale — 512×512, 1024×512, 1024×1024 worlds |

---

## ⚙️ Configuration

### CLI Flags

```bash
go run ./cmd/server \
  -addr :8080 \
  -width 1024 \
  -height 512 \
  -seed 20260823 \
  -snapdir ./snapshots
```

### Server Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `addr` | `:8080` | Listen address |
| `width` | 1024 | World width (cells) |
| `height` | 512 | World height (cells) |
| `seed` | 20260823 | World generation seed |
| `snapdir` | `.` | Snapshot save directory |
| `TickRate` | 60 Hz | Simulation frequency |
| `NetworkRate` | 20 Hz | Client broadcast frequency |
| `MaxPlayers` | 16 | Maximum concurrent clients per world |
| `SnapshotInterval` | 5m | Auto-save frequency |

### Environment Variables (Docker)

| Variable | Service | Description |
|----------|---------|-------------|
| `BACKEND_URL` | Frontend | Backend URL for nginx proxy |
| `TZ` | Backend | Timezone for logging |

---

## 💰 API & Service Costs

**$0.00/month in external API costs.**

WorldWeaver is pure computation — a single Go binary that simulates physics. There are:
- ❌ No databases (state lives in memory + binary snapshots on disk)
- ❌ No third-party APIs (no OpenAI, no Firebase, no analytics services)
- ❌ No API keys required
- ❌ No recurring SaaS costs
- ❌ No cloud functions or serverless bills

The only cost is the server itself (a single VPS or container host).

---

## 📁 Project Structure

```
worldweaver/
├── cmd/server/              # Server entry point + CLI flag parsing
├── internal/
│   ├── world/               # State arrays, chunks (64×64), 15 material types
│   ├── simulation/          # Fixed-timestep engine, multi-rate scheduler
│   │   ├── engine.go        # Core tick loop, cell dispatch
│   │   ├── water.go         # Fluid dynamics, pressure, flow
│   │   ├── fire.go          # Combustion, spread, heat transfer
│   │   ├── lava.go          # Molten rock, melting, cooling
│   │   ├── weather.go       # Evaporation, clouds, precipitation, flooding
│   │   ├── erosion.go       # Hydraulic carving, sediment transport
│   │   ├── creatures.go     # Lotka-Volterra: herbivores, predators
│   │   ├── plants.go        # Growth, seeding, decay
│   │   └── ...              # Sand, soil, ice, oil, smoke, vapor
│   ├── game/
│   │   ├── worlds.go        # Multi-world manager (create/join/list)
│   │   ├── auth.go          # Nickname-based player login
│   │   ├── scoring.go       # Points, leaderboard, achievements
│   │   ├── player.go        # Player state, powers, influence
│   │   ├── powers.go        # 5 elemental powers + validation
│   │   ├── stability.go     # World stability metric
│   │   └── influence.go     # Influence economy
│   ├── network/
│   │   ├── hub.go           # WebSocket hub, multi-world routing
│   │   ├── websocket.go     # Connection lifecycle, upgrade
│   │   ├── ratelimit.go     # Per-IP + per-player rate limiting
│   │   ├── protocol.go      # Binary message encoding
│   │   ├── broadcast.go     # 20 Hz delta broadcast
│   │   └── client.go        # Client session management
│   ├── systems/             # Material registry (data-driven definitions)
│   ├── generation/          # Procedural terrain, biomes, climate zones
│   ├── persistence/         # Binary snapshot save/restore
│   ├── protocol/            # BinaryV1 encoder + DebugJSON
│   ├── metrics/             # Runtime telemetry (TPS, player count, memory)
│   └── config/              # Central configuration + validation
├── web/                     # TypeScript client
│   ├── main.ts              # Entry point, game loop
│   ├── isometric-renderer.ts # 2.5D WebGL2 isometric projection
│   ├── webgl2-renderer.ts   # Flat WebGL2 renderer (palette shader)
│   ├── renderer.ts          # Renderer selection + fallback
│   ├── particles.ts         # Cosmetic particle system (rain, embers, smoke, leaves)
│   ├── effects.ts           # Visual effect triggers
│   ├── audio.ts             # Procedural sound via Web Audio API
│   ├── network.ts           # WebSocket client + reconnection
│   ├── prediction.ts        # Client-side prediction for instant feedback
│   ├── lobby.ts             # World list, create/join, login
│   ├── scoring.ts           # Leaderboard UI + score display
│   ├── minimap.ts           # World minimap overlay
│   ├── input.ts             # Keyboard/mouse/touch input handling
│   ├── ui.ts                # HUD, power selector, status bar
│   └── src/                 # Shared types, platform adapters, design tokens
├── src-tauri/               # Tauri v2 desktop/mobile packaging
│   ├── tauri.conf.json      # App config, window settings
│   ├── Cargo.toml           # Rust dependencies
│   └── src/                 # Rust entry point
├── tests/                   # Integration + E2E tests (26 tests)
├── benchmarks/              # Simulation performance benchmarks
├── docs/
│   ├── decisions/           # ADR-001 through ADR-009
│   ├── architecture/        # System overview
│   ├── protocol/            # Binary protocol spec
│   └── performance/         # Benchmark methodology
├── .kiro/
│   ├── steering/            # product.md, tech.md, structure.md
│   ├── specs/               # 6 feature specifications (req/design/tasks)
│   └── hooks/               # 4 automated quality gates
├── .github/workflows/       # CI (test + lint) + Deploy (Dokploy webhook)
├── Dockerfile.backend       # Go multi-stage build (Alpine)
├── Dockerfile.frontend      # Vite build + nginx (Debian)
├── docker-compose.yml       # Full-stack local deployment
└── nginx.conf               # Frontend reverse proxy + WebSocket upgrade
```

---

## 📜 License

[MIT](LICENSE)

---

## 🙏 Attribution

Built with [**Kiro**](https://kiro.dev) for the [**Ready, Spec, Ship Hackathon 2026**](https://codingagents.fyi/hackathon/kiro/).

Kiro drove the entire engineering workflow — from product steering and specification authoring through implementation and automated quality enforcement. Every commit carries `Co-authored-by: Kiro (AI) <kiro-dev@amazon.com>`.
