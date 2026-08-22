# ADR-005: WebGPU as Optional Renderer Backend

**Status:** Accepted  
**Date:** 2026-08-22

## Context
WebGPU offers modern explicit GPU resource management but lacks universal browser support.

## Decision
WebGPU renderer is optional. Selected only when navigator.gpu exists and adapter initialisation succeeds. Falls back to WebGL2.

## Alternatives Considered
- WebGPU required (excludes Safari, older browsers)
- WebGPU ignored entirely

## Rationale
Preserves access to modern GPU APIs where available without breaking the majority of browsers. WGSL shaders maintained separately from GLSL. WebGPU visualisation only — no simulation.

## Consequences
- Two shader languages (GLSL + WGSL) to maintain
- Must test fallback path regularly
- WebGPU never runs simulation (server-authoritative invariant)

## References
- Architecture Addendum § 5, 9
