# God Sandbox Pivot — Requirements

## Intent

Reposition WorldWeaver as a **wide, layered 2D god sandbox**: the world shape and
scale of Terraria, the material physics of The Powder Toy, and god-mode authority
over everything. The player is never an avatar in the world — they act on it.

The 2.5D/isometric direction is abandoned. It split effort across renderers and
could not reach a convincing look without art assets that do not exist.

## REQ-GSP-001: Single 2D Renderer
The client SHALL ship exactly one renderer: the top-down/side-view WebGL2
renderer. The isometric renderer, the PixiJS renderer and the view toggle SHALL
be removed, along with the `pixi.js` dependency.

**Acceptance:** `web/` contains no isometric or Pixi renderer; `?view=` is gone;
`npm ls pixi.js` reports it absent; bundle size drops.

## REQ-GSP-002: Terraria-Shaped World
The world SHALL be markedly wider than tall, with distinct vertical strata rather
than a uniform bedrock mass.

Strata, top to bottom:

| Band | Share of height | Dominant materials |
|------|-----------------|--------------------|
| Sky | ~15% | empty, cloud |
| Surface | ~20% | soil, sand, plant, water |
| Underground | ~25% | soil, rock, water pockets, caves |
| Cavern | ~30% | rock, oil, ice, large caves |
| Underworld | ~10% | lava, ash, rock |

**Acceptance:** A generated world's material histogram differs measurably per
band; no band is more than 85% a single material.

## REQ-GSP-003: Horizontal Biomes
The surface SHALL be divided into contiguous horizontal biomes, each visually and
materially distinct: Desert, Forest, Wetland, Tundra, Volcanic.

**Acceptance:** Sampling surface material by column yields at least four distinct
biome signatures across the width.

## REQ-GSP-004: Camera Fits Height, Pans Width
Because the world is wide, the camera SHALL fit the world vertically and pan
horizontally. Zooming out further than "world height fills the canvas" SHALL NOT
be permitted, so no empty margin is ever visible.

**Acceptance:** At minimum zoom the full world height is visible and horizontal
panning traverses the full width without exposing background.

## REQ-GSP-005: Materials Come To Rest
Materials SHALL settle. A body of water or a pile of sand with no free surface to
move into SHALL stop changing, allowing its chunks to sleep.

This is a correctness requirement, not only a performance one: perpetual cell
churn is what produces the shimmering that makes the world look unstable.

**Acceptance:** A sealed basin of water reaches a state where, over 300 ticks,
fewer than 0.1% of its cells change, and the containing chunks report sleeping.

## REQ-GSP-006: Stable Visuals
Rendering SHALL NOT re-randomise per-cell appearance every frame. Animated
materials SHALL use continuous functions of time so motion reads as flowing
rather than vibrating.

**Acceptance:** With the simulation paused, consecutive frames of the same region
are visually near-identical; no per-cell strobing.

## REQ-GSP-007: Conserved Water Cycle
The weather cycle SHALL conserve water mass. Evaporation removes water and
creates vapour; condensation converts vapour to cloud; precipitation converts
cloud back to water. No stage SHALL create water from nothing.

Total world water SHALL be bounded. When it exceeds a configured ceiling,
evaporation increases and precipitation reduces.

**Acceptance:** Over 100,000 ticks with no player input, total water cell count
stays within ±15% of its starting value and never trends monotonically upward.

## REQ-GSP-008: God Mode Tools
The player SHALL be able to act directly on the world:

- **Sculpt** — raise and lower terrain
- **Place** — paint any material
- **Erase** — clear cells to empty
- **Forces** — the existing Rain, Heat, Wind, Growth, Life powers
- **Brush size** — adjustable radius, shown on the cursor

All actions remain server-validated and server-applied.

**Acceptance:** Each tool produces the expected world change; an out-of-bounds or
over-budget request is rejected with an error and leaves the world untouched.

## REQ-GSP-009: Influence Still Constrains
God tools SHALL consume influence. Direct material placement SHALL cost more than
elemental forces, so forces remain the efficient way to shape the world at scale.

**Acceptance:** Continuous placement drains influence to zero and further actions
are refused until it regenerates.

## REQ-GSP-010: No Regression In Multiplayer
All changes SHALL preserve server authority, the shared world, cursor presence
and the existing protocol.

**Acceptance:** The existing end-to-end suite passes unchanged.

## Out Of Scope

- Player avatars, inventory, combat, crafting
- Tile-based sprite art or auto-tiling assets
- 3D or isometric projection of any kind

## References
- WorldWeaver_Master_Plan.md § 7 (dual engine), § 12 (chunking), § 32 (priority)
- Existing specs: simulation-core, player-powers, multiplayer-protocol
