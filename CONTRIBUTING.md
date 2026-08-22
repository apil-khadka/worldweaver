# Contributing to WorldWeaver

Thank you for your interest in WorldWeaver! This document explains how to set up the project, contribute changes, and follow our development conventions.

## Development Setup

### Requirements
- Go 1.22+
- Node.js 20+
- (Optional) Tauri CLI for desktop builds

### Quick Start

```bash
git clone https://github.com/apil-khadka/worldweaver
cd worldweaver

# Backend
go mod tidy
go build ./...
go test ./...

# Frontend
cd web
npm install
npm run typecheck
```

## Branch Conventions

| Prefix | Purpose |
|--------|----------|
| `feature/` | New functionality |
| `fix/` | Bug fixes |
| `perf/` | Performance improvements |
| `docs/` | Documentation changes |
| `refactor/` | Code restructuring (no behaviour change) |
| `test/` | Test additions or fixes |
| `ci/` | CI/CD changes |

Example: `feature/vapor-material`, `perf/chunk-sleeping`, `fix/water-lateral-spread`

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(simulation): add vapor-to-water condensation
fix(network): prevent broadcast blocking on slow client
perf(simulation): skip inactive chunks in tick loop
test(world): add boundary condition tests for 2048x1024
docs: update benchmark results table
```

Scope examples: `simulation`, `network`, `renderer`, `game`, `protocol`, `generation`, `persistence`, `config`

## Testing

```bash
# All tests
go test ./...

# Acceptance tests only
go test ./tests/...

# Race detector
go test -race ./...

# Benchmarks
go test -bench=. -benchmem ./benchmarks/...
```

All PRs must pass `go test ./...` and `go vet ./...` before merging.

## Architecture Principles

**Critical constraint:** Client changes must NOT introduce client-authoritative world simulation.

The server is the single source of truth. Clients render and send input. This is non-negotiable.

Other principles:
- Simulation must not import network/transport packages
- Renderer must not import simulation rules
- Protocol must not dictate simulation timing
- Performance claims require benchmark evidence
- New materials must have at least one meaningful interaction

See [ADRs](docs/decisions/) for architectural decisions.

## Pull Request Checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] Frontend typechecks (`cd web && npm run typecheck`)
- [ ] No client-authoritative simulation introduced
- [ ] Benchmarks run if performance-sensitive
- [ ] ADR created if a significant architectural decision was made
- [ ] Kiro spec tasks updated if applicable

## Code Style

- **Go:** Standard `gofmt` formatting. Run `gofmt -w .` before committing.
- **TypeScript:** Strict mode. No `any` unless absolutely necessary.
- **CSS:** Use design tokens (`--ww-*` custom properties). No arbitrary magic values.

## Adding Materials

1. Add ID constant to `internal/systems/materials/registry.go`
2. Add `Def` entry in the `init()` function
3. Add simulation behaviour in appropriate system file
4. Mirror the ID in `web/src/core/constants.ts`
5. Add palette entry in the renderer
6. Write at least one interaction test

## Questions?

Open a [Discussion](https://github.com/apil-khadka/worldweaver/discussions) or an [Issue](https://github.com/apil-khadka/worldweaver/issues).
