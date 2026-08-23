# Atomic Elements — Requirements

## Intent

WorldWeaver's material system is 23 hardcoded IDs whose every interaction is an
`if` statement scattered across fifteen simulation files. There is no reaction
table: fire spreading to oil, lava quenching into rock, and plasma melting stone
are each a separate `switch` arm in a different file. Adding one material means
editing every file that tests for the class it belongs to, with no compile-time
guarantee the set is complete. The maintenance cost is O(materials²) and it is
already the reason the palette has stalled at eleven placeable entries.

This spec replaces that with a **data-driven element registry**: one table of
element definitions carrying real physical properties, and one table of reactions
carrying real chemical equations. Behaviour becomes data, so adding an element is
a row rather than a refactor. The registry is the single source of truth and is
served to the client, ending the current three-way duplication between the Go
constants, the HTML palette buttons, and the WebGL colour texture.

The elements are grounded in actual chemistry — sodium exploding in water,
thermite reducing iron oxide, magnesium burning in carbon dioxide — because real
reactions are more surprising than invented ones and give the world a coherent
logic a player can reason about rather than memorise. This is also what makes the
sandbox teach something, which is the difference between a tech demo and a
product worth judging.

Two adjacent defects are in scope because they make the sandbox unreadable and
unrewarding respectively: progression is broken (max level is reachable in about
two seconds) and material selection is a horizontal strip that cannot hold more
than a dozen entries.

## Measured starting point

Levelling, measured from the shipped code at brush radius 8 (197 cells/application,
8 applications/second):

| Power | Score per application | Score per second | Time to max level (10,000) |
|-------|---:|---:|---:|
| Rain | 591 | 4,728 | **2.1 s** |
| Growth | 1,182 | 9,456 | **1.1 s** |
| Heat | 197 | 1,576 | 6.3 s |

At the level-1 radius cap of 24 (1,793 cells) Rain scores 43,032/second and max
level arrives in **0.23 seconds**. Score is awarded per cell affected, so it grows
as O(r²) while the level thresholds grow ~5× per step.

Ecosystem state of the shipped `world.snapshot`, counted over all 655,360 cells:

| Material | Count |
|----------|------:|
| Herbivore (14) | 0 |
| Predator (15) | 0 |
| Sheep (21) | 0 |
| Grass (22) | 305 |

Every animal was extinct with food still present. Grazer population zero is an
absorbing state: reproduction requires a living parent, so no world could recover.

## REQ-AE-001: Element definitions SHALL live in one data table

Every element SHALL be described by a single record carrying its identity
(numeric ID, display name, chemical symbol, atomic number where applicable),
its category, and its physical properties: state at room temperature, density,
melting point, boiling point, ignition temperature, flammability, thermal
conductivity, and reactivity. Simulation code SHALL read behaviour from this
record rather than testing material IDs.

**Acceptance:** A new element can be added by appending one record, with no edit
to any `switch` on material ID, and it falls, melts, boils, and burns according
to its declared properties.

## REQ-AE-002: Reactions SHALL be declarative and chemically grounded

Pairwise interactions SHALL be expressed as reaction records naming the two
reactants, the products, the temperature threshold, the required catalyst if any,
the enthalpy sign, and the heat released or absorbed. Each record SHALL carry the
real chemical equation it models as documentation.

**Acceptance:** `2Na + 2H₂O → 2NaOH + H₂` is a row in the reaction table, not an
`if` statement; sodium placed in water produces lye and hydrogen and raises local
temperature. Removing the row removes the behaviour with no other code change.

## REQ-AE-003: Exothermic reactions SHALL be able to cascade

A reaction that releases heat SHALL raise the temperature of the cells around it,
so that a reaction which needs a temperature threshold can be triggered by a
neighbouring reaction reaching it. Chains SHALL be bounded so that a single
ignition cannot heat the whole world.

**Acceptance:** Igniting one thermite cell propagates through a connected thermite
mass without any explicit propagation code, and a world left running after the
mass is consumed returns to ambient temperature.

## REQ-AE-004: The registry SHALL be served to the client

The element table SHALL be exposed over HTTP so the client builds its palette,
its colour texture, and its element browser from the server's definitions. The
material list SHALL NOT be duplicated in TypeScript or HTML.

**Acceptance:** Adding an element server-side makes it appear in the client's
element drawer with the right colour and grouping after a page reload, with no
frontend edit.

## REQ-AE-005: Element selection SHALL be a searchable categorised drawer

Material selection SHALL be a panel that holds at least 60 entries grouped by
category, with text search over name and symbol, and SHALL show the selected
element's properties and its known reactions. It SHALL NOT overlay the region of
the canvas the player is working in, and SHALL be dismissable by keyboard.

**Acceptance:** With 60 elements registered, a player can find "Magnesium" by
typing "mg", read its melting point, see that it reacts with water and with
carbon dioxide, select it, and place it — without the canvas being obscured.

## REQ-AE-006: Progression SHALL be paced and resistant to farming

Score SHALL be awarded per action rather than per cell affected, so brush radius
does not multiply reward. Level thresholds SHALL grow super-linearly. Repeatedly
applying the same power to the same location SHALL yield diminishing returns.

**Acceptance:** Holding one power on one spot for a minute does not exceed level
3. Reaching level 2 takes under two minutes of ordinary play, and level 10 takes
hours rather than seconds.

## REQ-AE-007: The food chain SHALL have an extinction floor

When a tier of the food chain reaches zero population it SHALL be recolonised at
a slow rate, because reproduction cannot restart from zero. Recolonisation SHALL
NOT occur while the tier is merely scarce, so predator-prey population cycles are
preserved. A tier SHALL NOT be reintroduced when the tier below it cannot support
it.

**Acceptance:** A world whose grazers are extinct regains grazers; a world with a
living grazer population is not topped up; predators are not seeded into a world
with too few grazers to feed them.

## REQ-AE-008: Elements SHALL be verified by test, not by inspection

Every reaction in the table SHALL be covered by a test asserting the products,
and the registry SHALL be validated for internal consistency: no duplicate IDs,
no reaction naming an unregistered element, no melting point above its boiling
point.

**Acceptance:** `go test ./...` fails when a reaction references an element that
does not exist, and each famous reaction (alkali metal in water, thermite,
oxyhydrogen, neutralisation) has a named test.

## Out Of Scope

- Molecular or stoichiometric accuracy — reactions are 1:1 cell transformations,
  not balanced by mole ratio.
- Electricity and circuit propagation, despite conductivity being recorded.
- Pressure and gas diffusion modelling beyond the existing rise/disperse rules.
- Nuclear fission chain reactions, despite uranium being registered.
- Retrofitting the 23 existing materials into the registry in one step; they are
  migrated incrementally behind the registry's fallback path.

## References
- `.kiro/specs/material-system/requirements.md` — the system this supersedes
- `.kiro/specs/simulation-core/design.md` — tick order the reaction pass joins
- `.kiro/steering/tech.md` — architecture invariants and performance budget
- `internal/world/materials.go` — current hardcoded constants
- `internal/simulation/cell.go` — the dispatch switch being replaced
- `docs/decisions/ADR-011-data-driven-elements.md`
