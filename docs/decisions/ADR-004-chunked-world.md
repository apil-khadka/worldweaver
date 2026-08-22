# ADR-004: 64×64 Chunked World Representation

**Status:** Accepted  
**Date:** 2026-08-22

## Context
Simulating all cells every tick wastes CPU on stable regions. Broadcasting the full world wastes bandwidth.

## Decision
Partition the world into 64×64 cell chunks. Track active/dirty status per chunk.

## Alternatives Considered
- Full-world scan every tick
- Per-cell interest management (too granular)
- 32×32 chunks (more overhead, less data per update)

## Rationale
64×64 balances chunk overhead vs data size. Allows sleeping inactive regions, dirty-region networking, viewport-only streaming. Benchmarkable.

## Consequences
- Chunk boundary effects must be handled (materials crossing boundaries)
- Memory overhead for chunk metadata is small (< 1KB per chunk)
- Active-region optimisation can dramatically reduce tick cost

## References
- WorldWeaver_Full_Project_Documentation.md § 17
