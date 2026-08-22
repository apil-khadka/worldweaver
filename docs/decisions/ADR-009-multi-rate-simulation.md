# ADR-009: Multi-Rate Simulation Scheduler

**Status:** Accepted  
**Date:** 2026-08-22

## Context
Not every simulation system needs to run at 60 Hz. Plant growth at 60 Hz wastes CPU. World stability at 60 Hz wastes CPU.

## Decision
Different systems run at independent rates: material motion 60Hz, fire 30Hz, plants 5Hz, stability 2Hz, persistence 0.2Hz.

## Alternatives Considered
- All systems at 60 Hz (wastes compute on slow-changing systems)
- Adaptive rate (complex, harder to benchmark)

## Rationale
Dramatically reduces CPU load for slow-changing systems while maintaining responsive physics for fast-moving materials (sand, water). Each rate is configurable.

## Consequences
- Scheduler complexity (minor)
- Systems must handle being called at their own rate
- Benchmarks should report per-system cost

## References
- Architecture Addendum § 22
