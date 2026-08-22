# Changelog

All notable changes to WorldWeaver are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- **Simulation Engine** — 60 TPS fixed-timestep authoritative world simulation
  - Sand: falls down, diagonal slide
  - Water: falls, spreads laterally, wets soil, extinguishes fire
  - Fire: spreads to plants, finite lifetime, creates smoke
  - Plants: grow on moist soil, spread slowly
  - Environment: temperature decay, water evaporation
- **Material Registry** — Data-driven material definitions (14 materials: Empty through Ember)
- **World Generation** — Procedural terrain via simplex noise + biome seeder + climate initializer
- **Multiplayer Protocol** — chi HTTP router + WebSocket hub with per-client send queues
  - BinaryEncoderV1 for production (compact binary chunk updates)
  - DebugJSONEncoder for development (human-readable)
- **Renderer Abstraction** — IWorldRenderer interface with runtime backend selection
  - WebGL2Renderer: R8UI material texture + GLSL palette shader + animated water/fire
  - WebGPURenderer: WGSL shader pipeline (optional, modern browsers)
  - Canvas2DRenderer: ImageData-based fallback/debug renderer
- **Game Layer** — Player influence economy (Rain, Heat, Wind, Growth powers)
  - World Stability composite metric
  - Server-side power validation and rate limiting
- **Persistence** — Binary snapshot save/restore with atomic writes
- **Metrics** — TPS, tick P95, active cells/chunks, player count, outbound bandwidth
- **Configuration** — Centralized ServerConfig + Tuning with validation
- **Frontend Design System** — CSS custom properties (tokens.css), component styles
- **Platform Adapter** — BrowserPlatform + TauriPlatform stubs
- **Testing** — 9 acceptance tests + 4 benchmark configurations (512×512 through 2048×1024)
- **Kiro Integration** — 3 steering files, 6 specs, 4 hooks
- **ADRs** — 9 Architecture Decision Records (ADR-001 through ADR-009)
- **Docker** — Multi-stage Dockerfile (Go build + distroless runtime)
- **CI** — GitHub Actions workflow (Go test/vet, benchmarks, TypeScript typecheck)

### Architecture Decisions
- ADR-001: Server-authoritative simulation
- ADR-002: WebSocket transport
- ADR-003: WebGL2 primary renderer
- ADR-004: 64×64 chunked world
- ADR-005: WebGPU optional backend
- ADR-006: Binary protocol with JSON debug
- ADR-007: chi HTTP router
- ADR-008: Modular simulation systems
- ADR-009: Multi-rate simulation scheduler
