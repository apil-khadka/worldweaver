# World Generation & Ecosystem — Requirements

## Intent

Turn the world from a wide strip of bedrock with a few hand-placed landmarks into
a layered, populated world worth exploring, and give it a food web that sustains
itself the way the water cycle now does.

## Measured starting point

A freshly generated 2048x768 world:

| Material | Share |
|----------|-------|
| empty | 45.97% |
| rock | 51.85% |
| water | 0.78% |
| soil | 0.62% |
| plant | 0.38% |
| sand | 0.35% |
| lava | 0.04% |
| smoke | 0.01% |
| herbivore | 6 cells |

By depth band:

| Rows | Dominant | Share |
|------|----------|-------|
| 0–153 | empty | 99.3% |
| 153–307 | empty | 89.4% |
| 307–460 | rock | 51.6% |
| 460–614 | rock | 98.2% |
| 614–768 | rock | 100.0% |

The lower 40% of the world contains essentially nothing. Feature counts are fixed
rather than proportional, so widening the world diluted them.

## REQ-GEN-001: Density-Driven Generation
Every generated feature SHALL be placed at a rate per unit width or area, not as
a fixed count. Doubling world width SHALL roughly double the number of lakes,
volcanoes, cave entrances and creature groups.

**Acceptance:** Generating at 1024, 2048 and 4096 wide yields feature counts and
material shares within 15% of each other proportionally.

## REQ-GEN-002: Five Vertical Strata
The world SHALL be generated in five depth bands, each with a distinct material
mix and cave character: Sky, Surface, Underground, Cavern, Underworld. Band
boundaries SHALL follow the terrain profile rather than sitting at fixed rows.

**Acceptance:** No band is more than 85% a single material, and each band's
dominant material differs from at least two others.

## REQ-GEN-003: Cave Systems
The Underground and Cavern bands SHALL contain connected cave systems, not
isolated pockets. Caves SHALL be reachable from the surface.

**Acceptance:** At least 12% of cells below the surface band are open, and a
flood fill from a surface opening reaches at least 40% of that open volume.

## REQ-GEN-004: Underworld
The deepest band SHALL be volcanically active: lava bodies, ash, and heat that
propagates upward into the Cavern band.

**Acceptance:** Lava is at least 3% of the deepest band; temperature there
averages above the plant heat-death threshold.

## REQ-GEN-005: Aquifers
The Underground band SHALL contain trapped water bodies which, once breached,
drain under gravity into caves below.

**Acceptance:** Erasing the floor beneath an aquifer causes water to flow
downward and settle in the cave below.

## REQ-GEN-006: Horizontal Biomes
The surface SHALL be divided into contiguous biomes — Desert, Forest, Wetland,
Tundra, Volcanic — with blended transitions. Biome *count* SHALL scale with world
width so a wider world holds more biomes rather than wider ones.

**Acceptance:** At least four distinct biome signatures appear across the width;
no transition between biomes is a single-column step change.

## REQ-GEN-007: Terraria-Like Proportions
The default world SHALL be at least 3:1 wider than tall.

**Acceptance:** Default dimensions satisfy width/height >= 3.0 while holding
60 TPS in the benchmark suite.

## REQ-ECO-001: Multi-Level Food Web
The ecosystem SHALL model at least four trophic roles: producer (plants),
grazer, predator, and decomposer.

**Acceptance:** Each role is present in a generated world and its population
responds to changes in the level below it.

## REQ-ECO-002: Conserved Nutrient Cycle
Biomass SHALL be conserved. Plants draw nutrients from soil; grazers gain energy
from plants; predators gain energy from grazers; death returns nutrients to soil.
No stage SHALL create biomass from nothing.

This mirrors the water cycle: the previous weather implementation created water
at the precipitation step and flooded the world to 96% occupancy.

**Acceptance:** Over 100,000 ticks with no player input, total biomass stays
within ±20% of its starting value and shows no monotonic trend.

## REQ-ECO-003: Carrying Capacity
Populations SHALL be limited by available food rather than by a hard cap.
Overgrazing SHALL reduce plant cover and cause grazer die-off, which SHALL allow
plants to recover.

**Acceptance:** Starting with excess grazers produces a damped oscillation in
both populations rather than extinction or unbounded growth.

## REQ-ECO-004: Water Dependence
Creatures SHALL require water and SHALL move toward it when thirsty, dying if
they cannot reach it.

**Acceptance:** Creatures placed far from water migrate toward the nearest water
body; creatures sealed away from water die out.

## REQ-ECO-005: Climate Tolerance
Each species SHALL have a viable temperature range and SHALL fail to establish
outside it, so biomes carry characteristic life.

**Acceptance:** Species populations differ measurably between tundra, temperate
and desert biomes in the same world.

## REQ-ECO-006: Density-Based Seeding
Initial creature populations SHALL scale with habitable area, not be a fixed
count.

**Acceptance:** Creature count at 4096 wide is roughly double that at 2048 wide.

## REQ-ECO-007: Observable Ecosystem State
The server SHALL expose population and biomass figures per trophic level so the
ecosystem is legible to players and to tests.

**Acceptance:** `/api/metrics` reports counts per species and total biomass.

## Out of scope

- Named creature species with sprites or animation
- Genetics, evolution or mutation
- Creature pathfinding beyond local gradient following

## References
- `.kiro/specs/god-sandbox-pivot/` — the 2D god-sandbox direction
- `internal/simulation/creatures.go` — current two-species model
- `internal/world/generator.go` — current fixed-feature generator
- `docs/performance/benchmark-results.md` — sizing headroom
