# Hackathon Delivery — Design

## Strategy

The codebase is ~95% complete. The primary work is fixing compilation issues, verifying everything runs end-to-end, deploying, and producing submission materials.

## Phase 1: Make It Work

### Compilation Fixes

1. **Fix go.mod**: Change `go 1.27.0` to a real Go version (1.22 or 1.23)
2. **Fix phantom import**: `tests/simulation_test.go`, `internal/generation/terrain/generator.go`, and `internal/generation/biome/seeder.go` import `github.com/worldweaver/worldweaver/internal/systems/materials` which doesn't exist. Material constants live in `internal/world`. Options:
   - (A) Create `internal/systems/materials` as a thin re-export package
   - (B) Update all imports to use `internal/world` directly ← preferred
3. **Verify `go build ./cmd/server/`** compiles
4. **Verify `cd web && npm run build`** compiles

### Functional Verification

- Start server locally, open browser, confirm:
  - Lobby loads
  - WebSocket connects (WELCOME message received)
  - World renders (materials visible)
  - Powers work (click applies rain/heat/wind/growth)
  - Second browser tab sees same world state

## Phase 2: Deploy & Document

### Deployment

Target: Docker deploy to VPS or PaaS (Fly.io / Railway / Render / existing VPS with Dokploy).

Architecture:
```
Docker image (multi-stage: Node build → Go build → minimal runtime)
  → Serves static web/ from Go's http.FileServer
  → Single process: HTTP + WebSocket + Simulation
```

Existing `Dockerfile.backend` and `Dockerfile.frontend` can be consolidated or used with `docker-compose.yml`.

### README Enhancement

Add sections:
- Live demo URL
- "How Kiro Was Used" (reference specs, steering, hooks, workflow)
- Quick start (local development)
- Deployment instructions
- Testing instructions
- Attribution / tech stack

## Phase 3: Demo Video

Structure (≤3 min):
- 0:00–0:30 — Hook: show living world, state the thesis
- 0:30–1:30 — Live demo: multiplayer powers, emergent interactions
- 1:30–2:20 — Kiro workflow: show specs→code, hooks, steering
- 2:20–2:50 — Architecture + real metrics (TPS, cells, p95)
- 2:50–3:00 — Closing

Use screen recording + voiceover. No fancy editing needed.

## Phase 4: Submit

- Fill Google Form with repo link, demo URL, video URL, project description
- Final smoke test all links
- Verify `.kiro/` visible in repo

## Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| Go won't compile | Focus on import fix first — the actual code is correct |
| Frontend build fails | dist/ already exists as fallback; can serve pre-built |
| Deployment issues | Docker is already scaffolded; fall back to screen recording of localhost |
| Time pressure | README + video are highest ROI for judging; prioritize over fixing stretch features |
