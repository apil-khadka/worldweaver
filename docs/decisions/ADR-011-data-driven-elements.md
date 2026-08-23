# ADR-011: Data-Driven Elements and Declarative Reactions

**Status:** Accepted
**Date:** 2026-08-23
**Supersedes:** part of ADR-008 (modular simulation), which assumed one simulation
module per material

## Context

Material behaviour is procedural: a dispatch switch in `internal/simulation/cell.go`
routes each cell to a per-material handler, and interactions live as `if` chains and
nested switches inside those handlers. Classification is a second hardcoded layer —
eight predicate functions (`IsSolid`, `IsFlammable`, `IsLiquid`, `IsGas`,
`IsTransient`, `IsEmissive`, `IsHazard`, `IsCreature`) plus `Density()`, each its own
switch over the same 23 IDs.

Adding a material means editing every file that tests for a class it belongs to,
across fifteen files, with no compile-time signal that the set is complete. A
material omitted from `Density()` silently gets the default of 50 and sinks like
every other omission. The palette is duplicated five ways: the Go constants, the
HTML buttons with inline hex, and three renderer colour tables.

The cost is O(materials²) in maintenance and it has already bitten: the placeable
palette has stalled at eleven entries while 23 materials exist, and eight defined
materials have no way to be placed at all.

We want roughly 40 more elements with real chemical reactions between them. Under the
current structure that is on the order of 65 new interactions hand-written into
fifteen files.

## Decision

Element properties move into a single data table of `Element` records carrying
identity, category, phase, and real physical values — density, melting point,
boiling point, ignition temperature, flammability, conductivity, reactivity — plus
the element each one melts, boils, and burns into.

Pairwise interactions move into a single table of `Reaction` records naming the
reactants, the products, a temperature threshold, an optional adjacent catalyst, and
a heat delta. Each record carries the real chemical equation it models as
documentation. The pair is order-insensitive.

The tables are indexed at startup into a flat `[256][256]` array of reaction indices,
so a reaction check is one array read rather than a table scan.

The registry is served over HTTP and becomes the single source of truth for the
client's palette, colour texture, and element browser.

Physical values are taken from the CRC Handbook of Chemistry and Physics and the NIST
Chemistry WebBook rather than chosen for feel.

## Alternatives Considered

- **Keep the procedural system and hand-write 65 interactions.** Rejected: it is the
  status quo extrapolated, and the status quo has already stalled the palette. No
  mechanism would prevent the same stall at 60 materials.

- **A scripting layer (Lua or an embedded expression language) for reactions.**
  Rejected: it buys generality we do not need — every reaction we identified is a
  pairwise transform with a threshold — at the cost of a sandbox, a new dependency,
  and per-tick interpreter overhead inside a 16 ms budget.

- **An interface per material (`type Material interface { Simulate(...) }`).**
  Rejected: it makes the dispatch polymorphic but leaves interactions bilateral, so
  each material still needs to know about the others. It also costs an interface
  dispatch per cell per tick, and the profile is already dominated by cell iteration.

- **Widen `Temperature` to `int32` to hold thermite's range.** Rejected in favour of
  changing the unit from tenths of a degree to whole degrees, which keeps `int16` and
  costs a decimal place nothing reads. Widening would add 2 MB on a 2M-cell world
  against the memory budget in `tech.md`.

- **Invented reactions tuned purely for play.** Rejected: real chemistry produces
  more surprising results than invention — magnesium burning *in* carbon dioxide is a
  case no designer would think to add — and it gives the sandbox a logic a player can
  reason about instead of memorise.

## Rationale

The decisive property is that adding an element becomes a row rather than a refactor,
and that the set of interactions becomes *enumerable*. Enumerability is what lets the
client show a player what an element does, which is impossible while that knowledge
exists only as control flow.

Declaring the data also makes it testable in a way procedural rules are not: the
registry can be validated for internal consistency (no duplicate IDs, no melting
point above a boiling point, no reaction naming an unregistered element) and every
reaction can be asserted against its declared products. A table with 65 rows and a
validator is safer to edit than 65 `if` statements across fifteen files.

The performance objection is answered by the index and by the reactivity gate: cells
whose element cannot react skip the neighbour walk entirely, and rock, sand and soil
are the overwhelming majority of a typical world.

This narrows ADR-008: simulation stays modular by *concern* (falling, flow, thermal,
life), but no longer gains a module per material.

## Consequences

**Positive.** Adding an element is one record and one colour. Interactions are
enumerable, so the client can explain them. Physical values being real means
behaviour like tungsten surviving lava, or gallium melting in a warm room, falls out
without tuning. The three-to-five-way palette duplication collapses to one served
table.

**Negative.** The temperature unit change is breaking: existing snapshots store
tenths of a degree and are indistinguishable from whole degrees. This forces a
snapshot version bump with a migration read path, and every temperature literal in
the simulation has to be rewritten. That migration is also the opportunity to fix
version 1 dropping `Energy` and `Thirst`, which currently loses creature state across
a restart.

**Neutral.** The 256-ID ceiling from `uint8` is unchanged and remains generous at 63
used. The WebGL palette texture is already 256 wide and needs no resize. The wire
protocol sends raw cell bytes with no length assumption and is unaffected.

**Transitional.** The 23 legacy materials are not migrated in one step. The registry
carries them where the data is known and the hardcoded switches remain as the
fallback path, so the two coexist until the legacy handlers are retired.

## References
- `.kiro/specs/atomic-elements/requirements.md`
- `.kiro/specs/atomic-elements/design.md`
- `docs/decisions/ADR-008-modular-simulation.md` — narrowed by this decision
- `internal/world/materials.go` — the hardcoded constants and predicates
- `internal/simulation/cell.go` — the dispatch switch
- CRC Handbook of Chemistry and Physics — physical property values
- NIST Chemistry WebBook — standard enthalpies of formation
