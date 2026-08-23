# Hackathon Delivery — Tasks

## Phase 1: Make It Work (CRITICAL PATH)

- [ ] Fix go.mod: change `go 1.27.0` to valid Go version (e.g., `go 1.22`)
- [ ] Fix import path: update `tests/simulation_test.go` to import `internal/world` instead of `internal/systems/materials`
- [ ] Fix import path: update `internal/generation/terrain/generator.go` to use `internal/world`
- [ ] Fix import path: update `internal/generation/biome/seeder.go` to use `internal/world`
- [ ] Verify: `go build ./cmd/server/` compiles successfully
- [ ] Verify: `go build ./...` compiles all packages
- [ ] Verify: `cd web && npm install && npm run build` succeeds
- [ ] Verify: `go run cmd/server/main.go` starts and listens on :8080
- [ ] Verify: browser at localhost:8080 shows lobby, connects, renders world
- [ ] Verify: power application works (Rain spawns water, Heat ignites)
- [ ] Verify: multiplayer works (two tabs see same world)
- [ ] Run tests: `go test ./tests/ -count=1 -v` passes

## Phase 2: Deploy & Document

- [ ] Build Docker image (or use existing Dockerfile.backend + web/dist)
- [ ] Deploy to publicly accessible URL (Fly.io / Railway / VPS)
- [ ] Verify deployed version works in fresh browser (lobby → connect → play)
- [ ] Verify WSS (WebSocket over HTTPS) works in production
- [ ] Enhance README.md with hackathon sections:
  - Live demo link
  - "How Kiro Was Used" section
  - Local setup instructions (go run + npm run dev)
  - Testing instructions
  - Architecture diagram
  - Attribution
- [ ] Make GitHub repo public (if not already)
- [ ] Verify `.kiro/` directory is visible in public repo (not gitignored)

## Phase 3: Demo Video

- [ ] Write demo video script (structure in design.md)
- [ ] Record screen: show lobby → connect → apply powers → see simulation
- [ ] Record screen: open second tab, show multiplayer sync
- [ ] Record screen: show .kiro/ specs, steering, hooks in editor
- [ ] Record screen: show real metrics in footer (TPS, cells, chunks, RTT)
- [ ] Add voiceover explaining thesis: "server computes reality, GPU visualizes"
- [ ] Export video ≤3 minutes
- [ ] Upload to YouTube (unlisted) or other public host
- [ ] Verify video link is accessible without login

## Phase 4: Submit

- [ ] Write project description for Google Form (2-3 paragraphs)
- [ ] Fill out submission form: https://forms.gle/xBLjk9nKMqbi2zie9
- [ ] Double-check all links work (repo, demo, video)
- [ ] Final smoke test of deployed app
- [ ] Confirm submission received

## Post-Submission (DO NOT block on these)

- [ ] Add CI badge to README once GitHub Actions pass
- [ ] Tag release v0.1.0
- [ ] Run and document benchmark results
