# World Generation & Ecosystem — Tasks

Ordered so each stage is independently verifiable. Phase 1 is the largest gain
per unit effort: it fills the 40% of the world that currently holds nothing.

## Phase 1 — Depth: make the existing world worth digging into

- [ ] Extract generation into ordered stages in `internal/world/generation/`,
      one file per stage, each callable and testable alone
- [ ] Add a `Density` struct expressing features per 1000 columns, and derive all
      feature counts from world width
- [ ] Test: generate at 1024, 2048 and 4096 wide; assert proportional feature
      counts and material shares within 15%
- [ ] Define the five bands as fractions of surface-to-floor depth so they follow
      the terrain profile
- [ ] Implement band-specific material mixes (Surface, Underground, Cavern, Underworld)
- [ ] Test: no band exceeds 85% a single material
- [ ] Implement tunnel-worm carving with correlated heading changes, seeded from
      surface cave entrances
- [ ] Implement noise-threshold chamber carving with per-band cutoffs
- [ ] Test: at least 12% of sub-surface cells are open
- [ ] Test: flood fill from a surface opening reaches at least 40% of open volume
- [ ] Generate the underworld: lava bodies, ash, upward heat propagation
- [ ] Test: lava is at least 3% of the deepest band
- [ ] Generate aquifers as rock-shelled water pockets in the Underground band
- [ ] Test: breaching an aquifer floor drains water into the cave below
- [ ] Add a post-generation settle pass so the world is at rest before players join
- [ ] Update `TestGeneratedWorldHasVisibleVariety` to run at the configured
      default size rather than a hardcoded 1024x512

## Phase 2 — Life: an ecosystem that sustains itself

- [ ] Add a `Nutrient []uint8` field to the world, same layout as Moisture
- [ ] Seed soil nutrient during generation, richer in Wetland and Forest biomes
- [ ] Make plant growth consume soil nutrient, and fail where nutrient is absent
- [ ] Add `MatCarrion` with a decay lifetime
- [ ] Creature death produces carrion instead of clearing the cell
- [ ] Carrion decay returns nutrient to the soil beneath it
- [ ] Track world-level total biomass across all stores
- [ ] Test: 100,000 ticks with no input keeps biomass within ±20% and shows no
      monotonic trend — the check that caught the water cycle creating water
- [ ] Move creature energy out of the temperature field into its own array
- [ ] Widen the energy range from 3–8 to 0–255 and retune eat, decay and
      reproduction thresholds against it
- [ ] Add a thirst counter; creatures seek water by following the moisture gradient
- [ ] Test: creatures sealed away from water die; creatures near water survive
- [ ] Give each species a viable temperature range; reproduction fails outside it
- [ ] Test: species populations differ measurably between tundra, temperate and
      desert regions of one world
- [ ] Seed creatures by habitable area rather than a fixed count
- [ ] Test: creature count at 4096 wide is roughly double that at 2048 wide
- [ ] Test: seeding excess grazers yields a damped oscillation, not extinction or
      runaway growth
- [ ] Report per-species counts and total biomass on `/api/metrics`
- [ ] Surface population figures in the client HUD

## Phase 3 — Length: Terraria proportions

Gated on the binary protocol, which is what the extra width actually costs.

- [ ] Replace JSON+base64 chunk updates with a binary frame format
- [ ] Implement interest-managed streaming keyed on each client's viewport
- [ ] Measure initial load time and steady-state bandwidth at 3072x896
- [ ] Raise the default world to 3072x896 (3.43:1) once load is acceptable
- [ ] Scale biome *count* with width so a wider world holds more biomes
- [ ] Blend biome transitions over a zone rather than a single column
- [ ] Re-run the benchmark suite and update `docs/performance/benchmark-results.md`

## Housekeeping carried over

- [ ] `docs/performance/benchmark-results.md` still lists older figures and claims
      a 100-tick warm-up the benchmark code does not perform
- [ ] Confirm evaporation actually fires in a generated world; the cycle is proven
      bounded but may be dormant, since it needs cells above 15 °C

## Measured baseline this spec is written against

2048x768, freshly generated:

```
empty  45.97%   rock 51.85%   water 0.78%   soil 0.62%
plant   0.38%   sand  0.35%   lava  0.04%   smoke 0.01%
herbivore 6 cells, predator 0

rows   0–153  empty 99.3%
rows 153–307  empty 89.4%
rows 307–460  rock  51.6%
rows 460–614  rock  98.2%
rows 614–768  rock 100.0%
```
