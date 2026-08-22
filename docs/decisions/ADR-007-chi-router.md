# ADR-007: go-chi/chi HTTP Router

**Status:** Accepted  
**Date:** 2026-08-22

## Context
WorldWeaver needs HTTP routing for static file serving, WebSocket upgrades, and API endpoints.

## Decision
Use go-chi/chi v5 as the HTTP router.

## Alternatives Considered
- net/http stdlib only (lacks middleware composition)
- Gin (heavyweight, non-stdlib Handler interface)
- Echo (similar to Gin)
- Fiber (non-net/http compatible)

## Rationale
Chi is idiomatic Go (implements http.Handler), has excellent middleware composition, is well-maintained, and adds no unnecessary abstractions over net/http. Minimal learning curve.

## Consequences
- Middleware chain for logging, recovery, request IDs
- Routes compose cleanly with chi.Router
- Easy to test with httptest

## References
- User requirement (steering message)
- go-chi/chi: https://github.com/go-chi/chi
