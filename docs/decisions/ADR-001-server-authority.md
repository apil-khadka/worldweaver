# ADR-001: Server-Authoritative Simulation

**Status:** Accepted  
**Date:** 2026-08-22

## Context
Rich simulations running on every client device create synchronisation problems, limit world complexity to the weakest device, and make cheat prevention difficult.

## Decision
The Go server is the single authoritative source of truth for all world simulation state. Clients send input and render state received from the server. Clients NEVER mutate world state directly.

## Alternatives Considered
- Client-side simulation with server reconciliation
- Deterministic lockstep
- Peer-to-peer simulation

## Rationale
Server authority eliminates divergence, enables large worlds regardless of client power, simplifies cheat prevention, and makes the architecture demonstrable for the hackathon.

## Consequences
- Higher server compute requirements
- Latency between input and visible result
- Network bandwidth for state distribution
- Thin clients can run on any device

## References
- WorldWeaver_Full_Project_Documentation.md § 20
