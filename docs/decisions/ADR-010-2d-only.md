# ADR-010: A single 2D renderer

**Status:** Accepted
**Date:** 2026-08-23
**Supersedes:** the "optional accelerated backend" framing in ADR-003 and ADR-005

## Context

The client accumulated four renderers: Canvas2D, WebGL2, an isometric 2.5D
renderer, and a PixiJS renderer. Only one can be on screen at a time, so each
addition split attention rather than adding capability.

The state at the time of this decision:

- **PixiJS** was the default. Its texture upload path did not work against the
  PixiJS 8 `TextureSource` API, so the canvas stayed blank.
- **Isometric 2.5D** compiled and its shaders were valid, but its camera mapping
  converted pan offsets to grid coordinates incorrectly and it selected no
  visible cells, so it also rendered nothing.
- **WebGL2** worked, but its fragment shader sampled the material texture with an
  un-flipped Y axis, so the world appeared upside down.
- **Canvas2D** worked and was the only correct renderer, at a fraction of the
  visual quality.

Separately, the isometric direction was aimed at a look comparable to Terraria or
Age of Empires. Those games rely on large hand-authored sprite sets. WorldWeaver
renders a cellular grid and has no art assets, so isometric projection alone was
never going to close that gap; it produced coloured blocks at an angle.

## Decision

Ship one renderer: **WebGL2, top-down side-view 2D**. Canvas2D is retained only
as a compatibility fallback for hardware without WebGL2.

Delete the isometric renderer, the PixiJS renderer, the view toggle, and the
`pixi.js` dependency.

Visual quality is pursued through the fragment shader instead of through
projection or sprite art: procedural material grain, geological strata, surface
and crevice lighting, depth-shaded water, sky gradients, and emissive light bleed.

## Alternatives considered

**Keep PixiJS and fix the texture upload.** Rejected. PixiJS provides scene
graph, batching and context management, none of which a single full-screen quad
with one shader needs. It cost roughly 130 kB of bundle for abstraction the
project does not use.

**Keep the isometric renderer behind a toggle.** Rejected. Two renderers sharing
one canvas cannot coexist cleanly: a canvas returns the same context on every
`getContext` call, so the previous renderer's animation loop keeps drawing into
the context the new one owns. Working around that meant a page reload per toggle,
for a view that looked worse.

**Commit to isometric and buy or commission tiles.** Rejected on time, and it
would make the renderer dependent on assets rather than on the simulation.

## Consequences

Positive:

- One rendering path to reason about, profile and improve
- Bundle dropped from 81.2 kB to 68.0 kB and lost a large transitive dependency
- Shader work benefits every player instead of one of four code paths
- The remaining renderer is verified: its shaders compile and link in a real
  WebGL2 context, and all uniforms resolve

Negative:

- No 2.5D depth illusion. Terrain depth is conveyed by shading, strata and
  material layering rather than by geometry.
- Canvas2D is now the only fallback, so its quality gap is more visible on old
  hardware.

## Evidence

- Shader compilation and uniform resolution verified in headless Chromium with
  software WebGL2
- Rendering verified by screenshot: sky gradient, layered bedrock, lake with an
  animated surface line, beach sand, cave interiors
- Bundle size measured before and after removal
