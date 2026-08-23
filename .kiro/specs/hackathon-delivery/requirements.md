# Hackathon Delivery — Requirements

## REQ-HAK-001: Compilable Codebase
The project SHALL compile cleanly with `go build ./...` and `npm run build` (web/).

**Acceptance:** Zero compilation errors on both Go server and TypeScript client.

## REQ-HAK-002: Runnable Demo
The app SHALL run locally with `go run cmd/server/main.go` and serve a playable game in the browser at localhost:8080.

**Acceptance:** Fresh checkout → build → run → open browser → see simulation running and can apply powers.

## REQ-HAK-003: Passing Tests
At minimum, the e2e and integration tests SHALL pass. Simulation unit tests should pass after import fixes.

**Acceptance:** `go test ./tests/ -count=1` passes for e2e_test.go and integration_test.go.

## REQ-HAK-004: Public Deployment
The application SHALL be deployed to a publicly accessible URL where judges can play without local setup.

**Acceptance:** HTTPS URL loads lobby, connects WebSocket, shows live simulation.

## REQ-HAK-005: Public Repository
The GitHub repository SHALL be public with `.kiro/` directory visible (not gitignored).

**Acceptance:** Anyone can `git clone` and see specs, steering, hooks in `.kiro/`.

## REQ-HAK-006: Professional README
README.md SHALL contain: project description, live demo link, architecture overview, setup instructions, how Kiro was used, testing instructions, and attribution.

**Acceptance:** A developer unfamiliar with the project can understand what it is and run it locally within 5 minutes of reading the README.

## REQ-HAK-007: Demo Video
A video (≤3 minutes) SHALL demonstrate: live multiplayer gameplay, server authority, Kiro workflow (specs/steering/hooks), and real metrics.

**Acceptance:** Publicly accessible video link (YouTube unlisted or similar) showing working application.

## REQ-HAK-008: Submission Form
The Google Form SHALL be submitted before August 23, 2026 23:59 UTC with all required fields filled.

**Acceptance:** Form confirmation received.

## References
- HACKATHON_PREP_PLAN.md
- WorldWeaver_Master_Plan.md § 34 (Hackathon Submission Checklist)
- https://codingagents.fyi/hackathon/kiro/
