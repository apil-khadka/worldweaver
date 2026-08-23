# Atomic Elements — Tasks

Ordered so each phase compiles and tests green on its own. The registry precedes
reactions because reactions reference element IDs; the temperature unit change
precedes thermite because thermite overflows the old range.

## Phase 1 — Element registry

- [ ] Add `internal/world/element.go` with the `Element`, `Category` and `Phase`
      types from design §2
- [ ] Add `internal/world/elements_data.go` with the 40 element records, physical
      values sourced from the CRC Handbook and NIST WebBook
- [ ] Add registry lookup: `Lookup(id) (Element, bool)`, `All()`, `ByCategory()`
- [ ] Add `Validate()` rejecting duplicate IDs, melting point above boiling point,
      and `MeltsInto`/`BoilsInto`/`BurnsInto` naming an unregistered element
- [ ] Route the existing `Density()` and phase predicates through the registry,
      keeping the hardcoded switch as the fallback for the 23 legacy materials
- [ ] Test: registry validates clean; every element resolves; a deliberately
      broken record fails `Validate()`

## Phase 2 — Temperature unit and snapshot v2

- [ ] Change temperature semantics from tenths of a degree to whole degrees across
      the simulation, updating every literal threshold
- [ ] Bump the snapshot format to version 2, persisting `Energy` and `Thirst`
      which version 1 dropped
- [ ] Add a version 1 read path that divides temperature by ten on load and
      rewrites as version 2
- [ ] Test: a version 1 fixture loads with temperatures converted, not
      reinterpreted; a v2 round trip preserves creature energy and thirst

## Phase 3 — Reaction table

- [ ] Add `internal/simulation/reaction.go` with the `Reaction` type from design §3
- [ ] Add `internal/simulation/reactions_data.go` with the 65 reactions, each
      carrying its real chemical equation
- [ ] Build the `[256][256]` reaction index at startup from the table
- [ ] Add the reaction pass to the tick, skipping cells whose element has zero
      reactivity so rock and sand cost nothing
- [ ] Apply `HeatDelta` to surrounding cells so exothermic reactions cascade
- [ ] Bound the cascade so one ignition cannot heat the whole world
- [ ] Test: every reaction in the table produces its declared products
- [ ] Test: named cases for sodium in water, thermite, oxyhydrogen, magnesium in
      CO₂, and acid neutralisation
- [ ] Test: no reaction references an unregistered element
- [ ] Test: a thermite mass self-propagates, and the world returns to ambient
      temperature afterwards
- [ ] Benchmark: the reaction pass stays inside the 16 ms tick budget on a 2M-cell
      world

## Phase 4 — Element drawer

- [ ] Serve the registry at `GET /api/elements` as grouped JSON including colours
- [ ] Build the WebGL palette texture from the endpoint instead of the hardcoded
      `buildPaletteData()`, and delete the duplicate copies in the WebGPU and
      alternate WebGL2 renderers
- [ ] Replace `#material-palette` with a categorised drawer holding 60+ entries,
      anchored inside `#canvas-wrapper` so it does not obscure the working area
- [ ] Add text search over element name and symbol
- [ ] Show the selected element's properties and its reactions, read from the
      endpoint
- [ ] Keep the `data-material` and `.active` contracts `input.ts` depends on, or
      update both together
- [ ] Test: with 60 elements registered the drawer lists all of them, search by
      symbol finds them, and selection places the right material

## Phase 5 — Progression

- [ ] Replace per-cell scoring with per-action scoring plus a `sqrt(cells)` area
      term
- [ ] Replace the 5-level threshold list with `50 * 2.2^(level-2)` over 25 levels
- [ ] Add repetition damping, per-minute rate damping, and the movement bonus
- [ ] Gate element categories behind levels 6–25
- [ ] Test: a minute of holding one power on one spot does not exceed level 3
- [ ] Test: level 2 is reachable in under two minutes of varied play
- [ ] Test: brush radius does not multiply score by more than the area term allows

## Completed already

- [x] Extinction floor for the food chain — `internal/simulation/ecosystem_recovery.go`
      with four tests covering recolonisation, the no-top-up rule, the predator prey
      floor, and seeded energy (REQ-AE-007)
- [x] Brush ring overlay drawn at the true affected radius, tinted per tool
- [x] Creature markers and live population legend, so sub-pixel animals are visible
- [x] Screen-to-world conversion corrected for render scale, fixing placement offset
- [x] Continuous application while the pointer is held
- [x] Forces honour the brush radius instead of a pinned 24 cells
- [x] Shared goal banner wired to the `goal_update` the server already broadcast
- [x] Wheel and pinch zoom removed; explicit keys and buttons instead
- [x] Pan keys moved to `e.code` so A/D survive Caps Lock, and ignored while typing
