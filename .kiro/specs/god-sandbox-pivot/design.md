# God Sandbox Pivot — Design

## 1. Diagnosis of the current problems

Three concrete defects motivate this pivot. Each has an identified cause.

### 1.1 Visual vibration

Two independent sources compound:

**Shader re-randomisation.** The fragment shader derives appearance from a hash
that is re-seeded from the clock:

```glsl
float flicker = hash(cellCoord + floor(u_time * 12.0));  // fire
float sparkle = hash(cellCoord + floor(u_time *  3.0));  // water
```

Every time the quantised clock advances, every affected cell jumps to an
uncorrelated new value. That is strobing, not animation.

*Fix:* drive animation from continuous functions of position and time, so
neighbouring cells and successive frames are correlated:

```glsl
float wave = sin(cellCoord.x * 0.4 + u_time * 2.0);
```

Where randomness per cell is wanted, hash on position only and use time to
*phase* it, never to re-seed it.

**Simulation churn.** Water spreads up to 4 cells per tick at 60 Hz and has no
rest state, so a large body permanently swaps cells. Every swap marks the chunk
dirty, so the client receives a stream of updates for water that is visually
static. The result is a body of water that boils.

*Fix:* a settling rule (§3).

### 1.2 Never-ending rain

The weather cycle is not mass-conserving. Precipitation can introduce water that
evaporation never removed, so total water grows without bound. Observed directly:
a world left running reached **504,872 of 524,288 cells occupied (96%)** — it had
flooded solid. A freshly generated world sits at 276,000.

*Fix:* conserve mass per transition and add a global ceiling (§4).

### 1.3 Two renderers, neither finished

Effort was split between a flat WebGL2 renderer and an isometric one. The
isometric path additionally had a broken camera mapping and produced an empty
screen. Committing to one renderer is the only way either becomes good.

*Fix:* delete the isometric and Pixi renderers.

## 2. World shape

### 2.1 Dimensions

Terraria's smallest world is 4200x1200, roughly 3.5:1. WorldWeaver is currently
1024x512 (2:1).

| Option | Cells | Note |
|--------|-------|------|
| 1024x512 (today) | 524k | too square, no room for strata |
| 2048x768 | 1.57M | 2.7:1, 3x current cost |
| 3072x768 | 2.36M | 4:1, close to Terraria's proportions |
| 4096x1024 | 4.19M | 4:1, 8x current cost |

Existing benchmarks already cover 2048x1024 (2.1M). **Target 3072x768**, gated on
the benchmark holding 60 TPS with chunk sleeping active. Chunk sleeping is what
makes this affordable: most of a wide world is inert bedrock at any moment.

### 2.2 Strata

Generation moves from "surface layers over uniform rock" to explicit depth bands,
each with its own material mix and cave density. Band boundaries follow the
terrain height so they undulate rather than sitting at fixed rows.

```
       ┌─────────────────────────────────────┐
Sky    │ empty, cloud                        │ 15%
       ├─────────────────────────────────────┤
Surface│ soil / sand / plant, lakes          │ 20%
       ├─────────────────────────────────────┤
Under- │ soil + rock, small caves, aquifers  │ 25%
ground ├─────────────────────────────────────┤
Cavern │ rock, large caves, oil, ice         │ 30%
       ├─────────────────────────────────────┤
Under- │ lava, ash, rock                     │ 10%
world  └─────────────────────────────────────┘
```

### 2.3 Biomes

Horizontal bands across the surface, each setting substrate, vegetation density
and baseline temperature/moisture:

| Biome | Substrate | Vegetation | Climate |
|-------|-----------|------------|---------|
| Desert | sand | sparse scrub | hot, dry |
| Forest | soil | dense trees | temperate, moist |
| Wetland | soil | reeds, standing water | warm, saturated |
| Tundra | soil + ice | sparse, low | cold |
| Volcanic | rock | none | very hot, lava |

Biome boundaries blend over a transition zone so the seam is not a hard line.

## 3. Settling

Add a per-cell `FlagSettled` bit alongside the existing `FlagMoved`.

A mobile cell (sand, water, oil) is marked settled when it attempts to move and
every candidate destination is blocked. The settled bit is cleared when any
neighbour changes, which the existing chunk `ChangedThisTick` machinery already
tracks.

```
attempt move
  ├── moved            → clear settled on self and neighbours
  └── all blocked      → set settled; skip this cell next tick
```

Settled cells are skipped by the dispatcher, so a full basin costs nothing, stops
marking its chunk dirty, and the chunk sleeps. This directly satisfies REQ-GSP-005
and removes the churn half of the vibration problem.

Water additionally needs a lateral rest condition: spread only when the
destination column is strictly lower, so a level surface stops instead of
oscillating left and right. The existing alternating scan direction hides this bug
by making it symmetric rather than eliminating it.

## 4. Water budget

Each weather transition becomes explicitly conserving:

```
evaporate:  water → vapour     (one cell consumed, one produced)
condense:   vapour → cloud
precipitate: cloud → water
```

A world-level counter tracks total water-equivalent cells (water + vapour +
cloud). A configured ceiling, expressed as a fraction of world cells, feeds back
into the rates:

```
overBudget = totalWater / ceiling

evaporationRate  *= clamp(overBudget, 1.0, 3.0)
precipitationRate/= clamp(overBudget, 1.0, 4.0)
```

Below budget the cycle runs normally; above it, evaporation accelerates and rain
throttles. The system becomes self-limiting instead of divergent.

The Rain power spawns water and so must draw against the same budget: when the
world is at its ceiling, Rain redistributes rather than adds. This also gives
World Stability a real physical meaning rather than a cosmetic number.

## 5. God tools

### 5.1 Protocol

Extend the existing power message with a tool discriminator rather than inventing
a parallel channel, so validation, rate limiting and influence accounting are
shared:

```json
{ "type": "power", "tool": "place", "material": 4,
  "x": 512, "y": 300, "radius": 8, "intensity": 1.0 }
```

`tool` is one of `force` (default, existing behaviour), `place`, `erase`,
`raise`, `lower`.

### 5.2 Server handling

All tools resolve to the same path: validate → charge influence → enqueue a
`PlayerAction` → apply inside the tick. Nothing bypasses the simulation, so
server authority is preserved (REQ-GSP-010).

Validation additions:
- `material` must be a registered ID, and must be placeable (not a creature)
- `radius` clamped to a per-tool maximum
- placement cost scales with area, so a large brush drains influence quickly

### 5.3 Costs

Direct placement is deliberately more expensive than forces, keeping the
elemental powers the efficient tool at scale (REQ-GSP-009):

| Tool | Cost model |
|------|-----------|
| force | existing per-activation cost |
| place | area x per-material multiplier |
| erase | area x 0.5 |
| raise/lower | area x 1.5 |

## 6. Camera

Replace "cover the canvas in both axes" with "fit height, pan width":

```
fitZoom = canvasHeight / worldHeight
zoom    = clamp(zoom, fitZoom, fitZoom * 16)
viewX   = clamp(viewX, 0, worldWidth - visibleWidth)
viewY   = clamp(viewY, 0, worldHeight - visibleHeight)
```

At minimum zoom the whole height is visible and the world extends off both sides,
which is the Terraria reading of the space. Horizontal panning is the primary
navigation, so camera acceleration should feel good on the X axis in particular.

## 7. Sequencing against the deadline

Submission is due 2026-08-23 23:59 UTC. At time of writing roughly **11 hours**
remain. The full pivot does not fit, so it is split:

**Tonight — behaviour fixes and subtraction.** Low risk, high visible payoff:

1. Delete isometric + Pixi renderers and the toggle
2. Fix shader animation (removes visible vibration)
3. Add settling (removes churn, water stops boiling)
4. Conserve the water cycle and cap it (fixes endless rain)
5. God tools: place / erase / brush size
6. Camera: fit height, pan width
7. Widen the world one step, to 2048x768, if benchmarks allow

**After the deadline — the larger reshaping.** Needs benchmarking and tuning:

8. 3072x768 with full five-band strata
9. Five horizontal biomes with blended transitions
10. Interest-managed streaming so a wide world stays cheap per client
11. Binary protocol to carry the increased chunk volume

Ordering rationale: items 1-6 make the existing world look and behave correctly,
which is what a judge sees. Items 8-11 make it bigger, which matters far less if
the smaller version already reads as intentional and stable.
