# ADR-008: Modular Simulation Systems

**Status:** Accepted  
**Date:** 2026-08-22

## Context
A monolithic simulation loop becomes brittle as material count grows. Different systems evolve at different rates.

## Decision
Split simulation into modular systems: MaterialSystem, LiquidSystem, ThermalSystem, FireSystem, LifeSystem, ChunkScheduler.

## Alternatives Considered
- Single monolithic loop (simpler initially, scales poorly)
- Entity-Component-System (overkill for cellular simulation)

## Rationale
Enables multi-rate scheduling, independent testing, feature flags per system, material registry approach. Adding materials doesn't require modifying unrelated systems.

## Consequences
- Systems must declare their tick rate
- Scheduler manages when each system runs
- Inter-system communication via world state (no direct coupling)

## References
- Architecture Addendum § 17, 22
