# ADR-003: WebGL2 Primary Renderer with Canvas2D Fallback

**Status:** Accepted  
**Date:** 2026-08-22

## Context
The client renders a large material grid (up to 1M+ cells). Per-cell fillRect calls are too slow at scale.

## Decision
WebGL2 is the primary accelerated renderer. Material data is uploaded as an R8UI texture; a GLSL fragment shader performs palette lookup. Canvas2D serves as reference/fallback. WebGPU is optional.

## Alternatives Considered
- Canvas2D only (too slow for large worlds)
- Three.js (unnecessary abstraction)
- WebGPU only (insufficient browser support)

## Rationale
WebGL2 provides GPU-accelerated rendering without WebGPU compatibility risk. Canvas2D ensures correctness verification. texSubImage2D enables efficient dirty-chunk updates.

## Consequences
- Maintains the IWorldRenderer abstraction for backend switching
- Shader code must be maintained alongside WGSL for WebGPU
- Canvas2D remains useful for debugging

## References
- WorldWeaver_Full_Project_Documentation.md § 30
- Architecture Addendum § 4-8
