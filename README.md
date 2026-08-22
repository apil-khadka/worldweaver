<div align="center">

# 🌍 WorldWeaver

### *One World. Many Forces. Emergent Reality.*

[![CI](https://github.com/apil-khadka/worldweaver/actions/workflows/ci.yml/badge.svg)](https://github.com/apil-khadka/worldweaver/actions/workflows/ci.yml)
[![Deploy](https://github.com/apil-khadka/worldweaver/actions/workflows/deploy.yml/badge.svg)](https://github.com/apil-khadka/worldweaver/actions/workflows/deploy.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)](https://go.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.5-3178C6?logo=typescript)](https://typescriptlang.org)

</div>

---

## What Is WorldWeaver?

WorldWeaver is a **real-time multiplayer emergent world simulation** where players influence a shared persistent world through environmental forces — Rain, Heat, Wind, and Growth. The Go server is fully authoritative, running the simulation at 60 TPS. Browser clients connect via WebSocket and only render — they never simulate physics. When multiple players wield forces simultaneously, their interactions create emergent phenomena: rainstorms extinguish fires, heat evaporates water into steam, wind carries seeds to grow forests.

---

## 🎮 Live Demo

> **[▶ Play WorldWeaver Now](https://worldweaver.apilkhadka.com.np/)**
>
> Open in two browser tabs to experience multiplayer. No signup required.

| Service | URL |
|---------|-----|
| Frontend (Game) | https://worldweaver.apilkhadka.com.np/ |
| Backend (API) | https://worldweaverapi.apilkhadka.com.np/ |
| Repository | https://github.com/apil-khadka/worldweaver |

---

## ✨ Key Features

- **Server-Authoritative Physics** — One Go process simulates the entire world at 60 TPS. Clients are pure renderers. No desync, no cheating.
- **Multiplayer Environmental Forces** — Players wield Rain, Heat, Wind, and Growth powers with a shared influence economy that creates cooperation and conflict.
- **Emergent Material Interactions** — Water flows downhill, sand falls, fire spreads to plants, heat evaporates water to steam. No scripted events — all behavior emerges from simple rules.
- **Multi-Rate Simulation** — Materials at 60 Hz, fire at 30 Hz, plants at 5 Hz, stability at 2 Hz. Each system runs at its natural frequency.
- **Persistent World** — Binary snapshot system saves and restores the full world state. The world survives server restarts.
- **Adaptive Renderer** — Runtime selects WebGPU → WebGL2 → Canvas2D based on hardware. WebGL2 uploads material data as an R8UI texture with GLSL palette lookup.
- **Zero External APIs** — Pure computation. No databases, no third-party services, no API keys, no recurring costs.
- **World Stability Score** — Collaborative objective: the world has a measurable stability metric that all players influence together.

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Go Authoritative Server                       │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  Simulation  │  │  Procedural  │  │     World Persistence    │  │
│  │  Engine      │  │  Generation  │  │     (Binary Snapshots)   │  │
│  │  60 TPS      │  │  Terrain +   │  │                          │  │
│  │              │  │  Climate     │  │                          │  │
│  └──────┬───────┘  └──────────────┘  └──────────────────────────┘  │
│         │                                                           │
│  ┌──────▼───────────────────────────────────────────────────────┐   │
│  │              WebSocket Hub (chi + coder/websocket)            │   │
│  │              Binary Protocol @ 20 Hz broadcast               │   │
│  └──────────────────────┬───────────────────────────────────────┘   │
└─────────────────────────┼───────────────────────────────────────────┘
                          │ WebSocket (Binary Frames)
          ┌───────────────┼───────────────┐
          │               │               │
          ▼               ▼               ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  Browser 1  │  │  Browser 2  │  │  Browser N  │
│  WebGL2     │  │  Canvas2D   │  │  WebGPU     │
│  Renderer   │  │  Renderer   │  │  Renderer   │
└─────────────┘  └─────────────┘  └─────────────┘
     Render Only — No Physics — Input Only
```

---

## 🔧 How Kiro Was Used

WorldWeaver was built entirely using **Kiro's spec-driven development workflow**. Every feature followed the same rigorous pipeline: **Requirements → Design → Tasks → Implementation**, with automated hooks enforcing quality at every step.

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
| [`material-system`](.kiro/specs/material-system/) | Material registry, interaction rules (water flows, sand falls, fire spreads) |
| [`multiplayer-protocol`](.kiro/specs/multiplayer-protocol/) | WebSocket lifecycle, binary encoding format, 20 Hz broadcast, delta updates |
| [`player-powers`](.kiro/specs/player-powers/) | Influence economy, power validation, radius/intensity limits, server enforcement |
| [`world-persistence`](.kiro/specs/world-persistence/) | Binary snapshot format, auto-save interval, restore on startup |
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

This makes Kiro's contribution visible in the Git history and on GitHub's contributor graph.

### Why This Matters

Kiro wasn't used as a code autocomplete. It was the **engineering process itself**:
- Steering files prevented scope creep (no NPCs, no crafting, no accounts)
- Specs ensured every subsystem was designed before being coded
- Hooks caught regressions immediately — not in CI minutes later
- The structure steering enforced clean package boundaries that made the codebase maintainable at scale

---

## 🛠️ Tech Stack

| Layer | Technology | Why |
|-------|------------|-----|
| Server | Go 1.27 | Single-binary, goroutine-per-client, zero GC pressure with value types |
| Router | go-chi/chi v5 | Idiomatic, stdlib-compatible, middleware composable |
| WebSocket | coder/websocket | Modern Go API, nhooyr fork with fixes |
| Client | TypeScript 5.5 + Vite | Strict types, fast HMR, tree-shaking |
| Renderer | WebGL2 / WebGPU / Canvas2D | R8UI texture + GLSL palette shader for 60fps material rendering |
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

# Integration/acceptance tests
go test ./tests/...

# Race condition detection
go test -race ./...

# Benchmarks (simulation performance)
go test -bench=. -benchmem ./benchmarks/...
```

### What Tests Cover

| Suite | Validates |
|-------|-----------|
| `internal/simulation/*_test.go` | Material physics — sand falls, water flows, fire spreads |
| `tests/simulation_test.go` | Acceptance criteria — emergent behaviors work correctly |
| `tests/integration_test.go` | Full server lifecycle — WebSocket connect, receive world state |
| `benchmarks/simulation_bench_test.go` | TPS at scale — 512×512, 1024×512, 1024×1024 worlds |

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
| `MaxPlayers` | 16 | Maximum concurrent clients |
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
│   ├── world/               # State arrays, chunks (64×64), material fields
│   ├── simulation/          # Fixed-timestep engine, cell dispatch, multi-rate scheduler
│   ├── systems/             # Material registry (data-driven definitions)
│   ├── generation/          # Procedural terrain, biomes, climate zones
│   ├── game/                # Player management, influence economy, stability score
│   ├── protocol/            # BinaryV1 encoder + DebugJSON (version-tagged wire format)
│   ├── network/             # chi router, WebSocket hub, delta broadcast
│   ├── persistence/         # Binary snapshot save/restore
│   ├── metrics/             # Runtime telemetry (TPS, player count, memory)
│   └── config/              # Central configuration + validation
├── web/                     # TypeScript client (Vite)
│   ├── src/render/          # WebGL2 / WebGPU / Canvas2D renderer backends
│   ├── src/core/            # Protocol constants, shared types
│   ├── src/config/          # Client configuration
│   ├── src/network/         # WebSocket client
│   └── src/platform/        # Browser/Tauri platform abstraction
├── tests/                   # Integration + acceptance tests
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
