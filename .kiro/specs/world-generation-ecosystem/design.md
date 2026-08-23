# World Generation & Ecosystem — Design

## 1. Why the current generator does not scale

`internal/world/generator.go` builds the world by placing named landmarks at
fractions of the width:

```go
volcanoX   := int(0.35 * float64(W))   // exactly one volcano
forestLeft := int(0.48 * float64(W))   // exactly one forest
forestRight:= int(0.72 * float64(W))
craterW    := 16                       // absolute cell count
```

Two consequences:

**Feature counts are fixed.** One volcano and one lake occupy the same absolute
area whatever the world size, so at 2048 wide they are half as prominent as they
were at 1024, and at 4096 they would be a quarter.

**Below the surface there is no generation at all.** The fill loop assigns a few
cells of soil or sand near the surface and then rock all the way down:

```go
for y := surfaceY; y < H; y++ {
    depth := y - surfaceY
    switch {
    case depth < 9:  MatSoil
    case depth < 18: MatSoil or MatRock
    default:         MatRock      // ← everything below ~18 cells
    }
}
```

That `default` branch is 40% of the world. Measured: rows 460–768 are 98–100%
rock. There are no caves, no aquifers, no ore variety and no underworld.

## 2. Generation rewritten as a pipeline

Replace one monolithic function with ordered, independently testable stages. Each
stage reads the world and writes to it; each can be benchmarked alone.

```
seed
 └─> 1. heightmap        fBm noise, biome-modulated amplitude
     2. strata fill      five depth bands, band-specific mixes
     3. cave carve       tunnel worms + noise threshold, per-band density
     4. aquifers         sealed water pockets in the Underground band
     5. underworld       lava bodies and ash in the deepest band
     6. surface water    lakes seeded from basins in the heightmap
     7. biome dressing   substrate, vegetation and climate per biome
     8. ecosystem seed   creatures by habitable area
     9. settle           run N ticks so loose material comes to rest
```

Stage 9 matters: generation places material that is not in equilibrium. Running
the simulation briefly before accepting players means the world the first player
sees is already settled, rather than visibly collapsing.

### 2.1 Density instead of counts

Every feature stage takes a rate and derives a count from world size:

```go
// Features per 1000 columns of world width.
type Density struct {
    Volcanoes    float64 // 0.7
    Lakes        float64 // 1.4
    CaveEntrances float64 // 4.0
    Aquifers     float64 // 2.5
}

func countFor(rate float64, width int) int {
    return int(rate * float64(width) / 1000.0)
}
```

At 2048 wide that gives 1 volcano, 2 lakes, 8 cave entrances, 5 aquifers; at 4096
it gives twice as many. This satisfies REQ-GEN-001 and is directly testable by
generating at several widths and comparing proportions.

### 2.2 Strata

Bands are expressed as fractions of the distance between the surface and the
world floor, so they follow the terrain rather than cutting across it.

| Band | Depth range below surface | Composition |
|------|---------------------------|-------------|
| Surface | 0 – 8% | biome substrate, vegetation |
| Underground | 8 – 35% | soil and rock mixed, small caves, aquifers |
| Cavern | 35 – 85% | rock, large caverns, oil and ice pockets |
| Underworld | 85 – 100% | rock, lava bodies, ash |

Sky is simply everything above the surface.

### 2.3 Cave carving

Two complementary techniques, because each alone has a characteristic failure:

**Tunnel worms** produce connected, walkable passages but look mechanical on
their own. A worm starts at a cave entrance on the surface, then steps with a
slowly turning heading, carving a circle of varying radius:

```
position = entrance
heading  = downward + jitter
repeat length times:
    carve circle(position, radius)
    heading += smallRandomTurn        // correlated, not per-step random
    radius   = base + noise(position)
    position += heading
```

Correlating the turn is what makes it read as a tunnel rather than a random walk.

**Noise threshold** produces organic chambers but leaves them disconnected. Open
a cell where 3D-ish noise exceeds a cutoff that varies by band, so the Cavern
band gets larger voids than the Underground band.

Worms guarantee connectivity to the surface, which REQ-GEN-003 asserts by flood
fill; the noise pass supplies the irregular chambers the worms pass through.

### 2.4 Aquifers

A sealed pocket of water in the Underground band, with a rock shell so it does
not immediately drain. These exist to reward the god tools: erasing the floor of
an aquifer releases water into the caves below, and the existing liquid
simulation handles the rest. No special-case code — the payoff is emergent.

## 3. Ecosystem

### 3.1 What exists now

`creatures.go` implements a two-species Lotka–Volterra model. Herbivores eat
plants and reproduce; predators sense herbivores within five cells and hunt them.
Energy is an `int16` in the range 3–8, stored in the temperature field.

Limitations: energy is too coarse for gradual starvation, there is no water
requirement, no temperature tolerance, nothing consumes corpses, and initial
population is a fixed six individuals.

### 3.2 Trophic structure

| Role | Material | Eats | Returns on death |
|------|----------|------|------------------|
| Producer | Plant | soil nutrients | nutrients to soil |
| Grazer | Herbivore | Plant | carrion |
| Predator | Predator | Herbivore | carrion |
| Decomposer | Carrion | — | nutrients to soil |

Carrion is a new transient material: a dead creature becomes carrion, which
decays over time into soil nutrients rather than vanishing.

### 3.3 Conserved nutrients

The lesson from the water cycle is explicit here. Precipitation previously
created water while leaving the cloud in place, so total water grew without bound
and the world flooded to 96% occupancy. The nutrient cycle is defined to make
that class of bug structurally impossible:

```
soil nutrient ──> plant growth        (nutrient decremented)
plant ─────────> grazer energy        (plant cell consumed)
grazer ────────> predator energy      (grazer cell consumed)
any death ─────> carrion ──> soil nutrient
```

Every arrow moves a fixed quantity from one store to another. Nothing is created.
A world-level biomass total is tracked so REQ-ECO-002 can assert conservation
over 100,000 ticks, exactly as `TestWaterCycleIsBounded` now does for water.

Nutrient level reuses the existing per-cell `Moisture`-style field pattern: a new
`Nutrient []uint8` array on the world, same layout as the others.

### 3.4 Carrying capacity

No hard population cap. Instead:

- Plants require soil nutrient to grow, and consume it when they do
- Grazing removes plants, which removes future grazing capacity
- Starving grazers die, become carrion, and return nutrients to the soil
- Recovered soil lets plants regrow

The result should be a damped oscillation. REQ-ECO-003 asserts that seeding
excess grazers produces oscillation rather than extinction or runaway growth.
This is the property most likely to need tuning, so the test asserts the shape of
the trajectory rather than specific numbers — the approach that worked for
`TestVegetationSurvivesSimulation`, where sampling the curve distinguished a real
equilibrium (`1302 → 625 → 622 → 622 → 622`) from a slow collapse.

### 3.5 Energy, water and climate

**Energy** widens from 3–8 to 0–255 so starvation is gradual and tunable. It
currently shares the temperature field, which blocks giving creatures a real
temperature response; it moves to its own field.

**Thirst** is a second counter. A creature seeks water when thirsty by following
the local moisture gradient, which needs no pathfinding: moisture is already
elevated near water because `wetSoilAround` maintains it.

**Climate tolerance** gives each species a viable temperature range, so tundra
and desert carry visibly different life. Outside the range, creatures fail to
reproduce before they die, so populations thin out naturally rather than being
forbidden.

## 4. World size

Benchmarks on an Apple M5 Pro, against the 16.67 ms budget for 60 TPS:

| Size | Cells | ms/tick | Budget used |
|------|-------|---------|-------------|
| 1024x512 | 524k | 0.996 | 6% |
| 2048x768 | 1.57M | 2.355 | 14% |
| 2048x1024 | 2.1M | 2.987 | 18% |

Simulation cost is not the constraint. Extrapolating, a 3072x896 world (2.75M
cells, 3.43:1 — close to Terraria's 3.5:1) lands near 3.9 ms, about 23% of the
budget.

The real constraint is the initial snapshot. It is sent as JSON with base64
payload, sized to the client's viewport. At fit zoom on a 3072x896 world the
viewport covers the full height and roughly 1500 columns: about 1.3M cells, or
1.8 MB encoded. Workable on a local network, uncomfortable over the internet.

So the width increase is sequenced *after* the binary protocol, and the depth work
comes first: filling the dead 40% improves the world more than extending it
sideways, and costs nothing in bandwidth.

## 5. Sequencing

**First — make the existing world deep.** No bandwidth cost, largest gameplay
gain, and it is what makes the world feel like Terraria rather than a strip.

1. Generation pipeline refactor with density-driven placement
2. Five-band strata
3. Cave systems, verified connected by flood fill
4. Underworld with lava
5. Aquifers

**Second — make it alive.**

6. Nutrient field and conserved nutrient cycle
7. Carrion and decomposition
8. Energy to its own field, widened range
9. Thirst and water seeking
10. Climate tolerance per species
11. Density-based creature seeding
12. Population metrics on `/api/metrics`

**Third — make it long.** Gated on the binary protocol, because that is what the
extra width actually costs.

13. Binary chunk protocol
14. Interest-managed streaming keyed on viewport
15. Widen to 3072x896
16. Biome count scaling with width
