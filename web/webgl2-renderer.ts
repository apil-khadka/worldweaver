/**
 * webgl2-renderer.ts — WebGL2 accelerated renderer adapter
 *
 * Wraps the WebGL2Renderer from src/render/webgl2/ and exposes the same
 * public interface as the Canvas2D WorldRenderer (renderer.ts), so all
 * existing consumers (network, input, effects, minimap, prediction) work
 * without changes.
 *
 * Single draw call per frame instead of per-pixel CPU iteration.
 *
 * Visual enhancements:
 *  - Depth shading (cells lower in world are darker)
 *  - Per-cell color variation via coordinate hash
 *  - Fire glow bleeding to adjacent pixels
 *  - Animated water shimmer via time uniform
 */

import type { FullSnapshot, ChunkUpdate, IWorldRenderer } from "./renderer.js";

// ── Shader sources ─────────────────────────────────────────────────────────

const VERT_SRC = /* glsl */ `#version 300 es
precision highp float;
in vec2 a_pos;
out vec2 v_uv;
void main() {
  gl_Position = vec4(a_pos, 0.0, 1.0);
  v_uv = a_pos * 0.5 + 0.5;
}`;

const FRAG_SRC = /* glsl */ `#version 300 es
precision highp float;
precision highp usampler2D;

in vec2 v_uv;
out vec4 fragColor;

uniform usampler2D u_material;   // R8UI texture, worldW × worldH
uniform sampler2D  u_palette;    // 256 × 1 RGBA lookup
uniform vec2       u_worldSize;  // (worldW, worldH)
uniform vec2       u_camera;     // world-coordinate top-left
uniform vec2       u_viewport;   // canvas size in pixels
uniform float      u_time;       // seconds

// 1 when the visible world contains fire, lava or embers. The light-bleed pass
// costs eight texture samples on every fragment, so it is skipped entirely when
// nothing is burning — which is the common case. The branch is on a uniform, so
// it is coherent across the draw call and costs nothing.
uniform int        u_hasEmissive;

// ── Material classes ───────────────────────────────────────────────────────

const uint MAT_EMPTY = 0u;
const uint MAT_ROCK  = 1u;
const uint MAT_SOIL  = 2u;
const uint MAT_SAND  = 3u;
const uint MAT_WATER = 4u;
const uint MAT_PLANT = 5u;
const uint MAT_FIRE  = 6u;
const uint MAT_VAPOR = 7u;
const uint MAT_SMOKE = 8u;
const uint MAT_LAVA  = 9u;
const uint MAT_ICE   = 10u;
const uint MAT_ASH   = 11u;
const uint MAT_OIL   = 12u;
const uint MAT_EMBER = 13u;
const uint MAT_CLOUD = 16u;
const uint MAT_VOID  = 17u;
const uint MAT_RAD   = 18u;
const uint MAT_PLASMA = 19u;
const uint MAT_CARRION = 20u;

// Opaque terrain: blocks light and forms the visible landscape silhouette.
bool isTerrain(uint m) {
  return m == MAT_ROCK || m == MAT_SOIL || m == MAT_SAND
      || m == MAT_PLANT || m == MAT_ICE || m == MAT_ASH || m == MAT_OIL;
}

// Anything that light passes through, for surface/edge detection.
bool isOpen(uint m) {
  return m == MAT_EMPTY || m == MAT_VAPOR || m == MAT_SMOKE || m == MAT_CLOUD;
}

// ── Utilities ──────────────────────────────────────────────────────────────

const float TAU = 6.2831853;

// Deterministic per-cell value. Callers must hash on position only and use time
// to shift phase; hashing a quantised clock re-rolls every cell each time the
// clock ticks over, which shows up as strobing rather than animation.
float hash(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

// Value noise, used for cloud/haze banding rather than per-cell speckle.
float valueNoise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  float a = hash(i);
  float b = hash(i + vec2(1.0, 0.0));
  float c = hash(i + vec2(0.0, 1.0));
  float d = hash(i + vec2(1.0, 1.0));
  return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

uint sampleMat(vec2 worldPos) {
  if (worldPos.x < 0.0 || worldPos.x >= u_worldSize.x
   || worldPos.y < 0.0 || worldPos.y >= u_worldSize.y) return MAT_EMPTY;
  vec2 uv = (floor(worldPos) + 0.5) / u_worldSize;
  return texture(u_material, uv).r;
}

// Per-material surface roughness — how strongly each material breaks up into
// lighter and darker grains. This is what separates "flat colour per cell" from
// something that reads as rock, soil or foliage.
float roughnessFor(uint m) {
  if (m == MAT_ROCK)  return 0.13;
  if (m == MAT_SOIL)  return 0.10;
  if (m == MAT_SAND)  return 0.07;
  if (m == MAT_PLANT) return 0.16;
  if (m == MAT_ASH)   return 0.10;
  if (m == MAT_ICE)   return 0.05;
  if (m == MAT_WATER) return 0.03;
  return 0.05;
}

// Sky colour for open cells above the landscape.
vec3 skyColour(float heightFrac, float x) {
  vec3 high = vec3(0.29, 0.47, 0.72); // deep blue overhead
  vec3 low  = vec3(0.62, 0.72, 0.80); // pale haze near the horizon
  // heightFrac is 1 at the top of the map and 0 at the bottom.
  vec3 sky = mix(low, high, pow(clamp(heightFrac, 0.0, 1.0), 1.4));

  // Very soft banding so large sky areas are not perfectly flat.
  sky += (valueNoise(vec2(x * 0.01, heightFrac * 4.0)) - 0.5) * 0.02;
  return sky;
}

// Background behind a see-through cell: either open sky or cave interior.
// An empty cell with terrain overhead is underground, otherwise it is sky.
vec3 backgroundAt(vec2 cellCoord, float depth) {
  // Five samples on a wider stride are enough to tell sky from cave interior,
  // and this runs for roughly half the screen, so the sample count matters.
  float cover = 0.0;
  for (int i = 1; i <= 5; i++) {
    if (isTerrain(sampleMat(cellCoord + vec2(0.0, -float(i) * 7.0)))) {
      cover = 1.0 - float(i - 1) / 5.0;
      break;
    }
  }
  vec3 sky  = skyColour(1.0 - depth, cellCoord.x);
  vec3 cave = mix(vec3(0.07, 0.06, 0.07), vec3(0.02, 0.02, 0.03), depth);
  return mix(sky, cave, cover);
}

void main() {
  // Map fragment to world coordinate. WebGL's v_uv.y is 0 at the bottom of the
  // screen while world row 0 is the top of the map, so Y is flipped here.
  vec2 flippedUV = vec2(v_uv.x, 1.0 - v_uv.y);
  vec2 worldPos = u_camera + flippedUV * u_viewport;
  vec2 cellCoord = floor(worldPos);

  // Outside the world entirely.
  if (cellCoord.x < 0.0 || cellCoord.x >= u_worldSize.x
   || cellCoord.y < 0.0 || cellCoord.y >= u_worldSize.y) {
    fragColor = vec4(0.05, 0.05, 0.063, 1.0);
    return;
  }

  vec2 cellUV = (cellCoord + 0.5) / u_worldSize;
  uint matID = texture(u_material, cellUV).r;

  // Depth through the world, 0 at the top row and 1 at the deepest row.
  float depth = cellCoord.y / u_worldSize.y;

  vec3 col;

  if (matID == MAT_EMPTY) {
    col = backgroundAt(cellCoord, depth);
  } else if (matID == MAT_VAPOR || matID == MAT_SMOKE || matID == MAT_CLOUD) {
    // Gases are translucent. There is no separate sky layer in this view, so
    // composite them over the background this cell would otherwise show.
    vec4 base = texture(u_palette, vec2((float(matID) + 0.5) / 256.0, 0.5));
    float density = base.a * (0.65 + valueNoise(cellCoord * 0.22 + u_time * 0.15) * 0.5);
    col = mix(backgroundAt(cellCoord, depth), base.rgb, clamp(density, 0.0, 1.0));
  } else {
    vec4 base = texture(u_palette, vec2((float(matID) + 0.5) / 256.0, 0.5));
    col = base.rgb;

    // ── Material grain ────────────────────────────────────────────────────
    // Two octaves: fine per-cell grain plus coarser clumping, so large areas of
    // one material develop visible structure instead of banding.
    float rough = roughnessFor(matID);
    float grain = hash(cellCoord) - 0.5;
    float clump = valueNoise(cellCoord * 0.18) - 0.5;
    col *= 1.0 + (grain * 0.6 + clump * 0.9) * rough;

    // ── Rock strata ───────────────────────────────────────────────────────
    // Bedrock is roughly half of the map, so without internal structure it
    // reads as one flat slab. Warped horizontal banding turns it into layered
    // geology, with occasional lighter mineral seams.
    if (matID == MAT_ROCK) {
      float warp = valueNoise(vec2(cellCoord.x * 0.010, cellCoord.y * 0.05));
      float strata = sin(cellCoord.y * 0.30 + warp * 7.0) * 0.5 + 0.5;
      col *= 0.90 + strata * 0.22;

      float vein = valueNoise(vec2(cellCoord.x * 0.035, cellCoord.y * 0.028));
      if (vein > 0.74) col *= vec3(1.10, 1.03, 0.93);
    }

    // ── Surface lighting ──────────────────────────────────────────────────
    // Terrain lit from above: the topmost cells of a landmass catch light and
    // the cells just beneath fall into shadow. This is what gives the world a
    // readable silhouette rather than a flat mass of colour.
    if (isTerrain(matID)) {
      bool openAbove = isOpen(sampleMat(cellCoord + vec2(0.0, -1.0)));
      if (openAbove) {
        col *= 1.28; // sun-facing crest
      } else if (isOpen(sampleMat(cellCoord + vec2(0.0, -2.0)))) {
        col *= 1.10; // just below the crest
      }

      // Ambient occlusion: crevices with many solid neighbours darken.
      float occl = 0.0;
      for (int dy = -1; dy <= 1; dy++) {
        for (int dx = -1; dx <= 1; dx++) {
          if (dx == 0 && dy == 0) continue;
          if (isTerrain(sampleMat(cellCoord + vec2(float(dx), float(dy))))) occl += 1.0;
        }
      }
      col *= mix(1.05, 0.86, occl / 8.0);

      // Directional side shading for a sense of relief.
      if (isOpen(sampleMat(cellCoord + vec2(-1.0, 0.0)))) col *= 1.08;
      if (isOpen(sampleMat(cellCoord + vec2( 1.0, 0.0)))) col *= 0.94;
    }

    // ── Water ─────────────────────────────────────────────────────────────
    if (matID == MAT_WATER) {
      // Count the water stacked overhead (-y is up). The more there is, the
      // further below the surface this cell sits, so the darker it reads.
      float waterAbove = 0.0;
      for (int i = 1; i <= 8; i++) {
        if (sampleMat(cellCoord + vec2(0.0, -float(i))) == MAT_WATER) waterAbove += 1.0;
        else break;
      }
      col *= mix(1.15, 0.72, waterAbove / 8.0);

      // Bright, animated surface line where water meets air.
      if (isOpen(sampleMat(cellCoord + vec2(0.0, -1.0)))) {
        float ripple = sin(cellCoord.x * 0.45 + u_time * 2.2) * 0.5 + 0.5;
        col += vec3(0.10, 0.16, 0.20) * (0.45 + ripple * 0.55);
      }

      // Glints fade in and out on a continuous wave. Re-rolling a hash against
      // a quantised clock made every cell jump to an unrelated value several
      // times a second, which read as vibration rather than sparkle.
      float glintPhase = hash(cellCoord) * TAU;
      float glint = sin(u_time * 2.5 + glintPhase);
      if (glint > 0.99) {
        col += vec3(0.30, 0.34, 0.38) * (glint - 0.99) * 100.0;
      }
    }

    // ── Plants ────────────────────────────────────────────────────────────
    if (matID == MAT_PLANT) {
      // Vary hue per clump so vegetation is not one flat green.
      float tint = valueNoise(cellCoord * 0.09);
      col.r *= 0.85 + tint * 0.35;
      col.b *= 0.80 + tint * 0.30;
      if (isOpen(sampleMat(cellCoord + vec2(0.0, -1.0)))) col += vec3(0.02, 0.09, 0.01);
    }

    // ── Emissive materials ────────────────────────────────────────────────
    if (matID == MAT_FIRE || matID == MAT_EMBER) {
      // Each cell keeps a fixed phase offset so neighbouring flames are not in
      // lockstep, while brightness varies smoothly in time. Two incommensurate
      // frequencies keep it from looking metronomic.
      float phase = hash(cellCoord) * TAU;
      float f1 = sin(u_time *  7.0 + phase) * 0.5 + 0.5;
      float f2 = sin(u_time * 11.3 + phase * 1.7) * 0.5 + 0.5;
      float flicker = f1 * 0.7 + f2 * 0.3;
      col = mix(col, vec3(1.0, 0.86, 0.35), flicker * 0.5);
    }
    if (matID == MAT_LAVA) {
      float pulse = sin(u_time * 2.2 + cellCoord.x * 0.25 + cellCoord.y * 0.2) * 0.5 + 0.5;
      col = mix(col, vec3(1.0, 0.72, 0.16), pulse * 0.4);
    }
    if (matID == MAT_ICE) {
      col += vec3(0.0, 0.02, 0.05) * (hash(cellCoord * 0.5) + 0.4);
    }

    // ── Exotic materials ──────────────────────────────────────────────────
    if (matID == MAT_PLASMA) {
      // Fast, bright churn so plasma reads as violent rather than merely hot.
      float phase = hash(cellCoord) * TAU;
      float a = sin(u_time * 17.0 + phase) * 0.5 + 0.5;
      float b = sin(u_time * 23.0 + phase * 2.1) * 0.5 + 0.5;
      col = mix(col, vec3(1.0, 0.97, 1.0), (a * 0.6 + b * 0.4) * 0.7);
    }
    if (matID == MAT_RAD) {
      // Slow pulse, suggesting something invisible doing damage.
      float phase = hash(cellCoord) * TAU;
      float pulse = sin(u_time * 3.5 + phase) * 0.5 + 0.5;
      col = mix(col, vec3(0.75, 1.0, 0.45), pulse * 0.45);
    }
    if (matID == MAT_VOID) {
      // Absence of light: crush toward black and add a faint violet rim so the
      // hole is still legible against a dark cave.
      col *= 0.35;
      float phase = hash(cellCoord) * TAU;
      col += vec3(0.10, 0.02, 0.16) * (sin(u_time * 2.0 + phase) * 0.5 + 0.5);
    }
    if (matID == MAT_CARRION) {
      col *= 0.9 + hash(cellCoord) * 0.15;
    }

    // Underground materials sit in progressively dimmer light.
    if (isTerrain(matID)) {
      col *= mix(1.0, 0.55, smoothstep(0.15, 1.0, depth));
    }
  }

  // ── Light bleed from emissive neighbours ─────────────────────────────────
  // Fire and lava spill warm light onto everything around them, which sells the
  // simulation far more than colouring the burning cell alone. Kept to a 3x3
  // neighbourhood so the per-fragment sample count stays modest.
  if (u_hasEmissive == 1 && matID != MAT_FIRE && matID != MAT_LAVA
      && matID != MAT_EMBER && matID != MAT_PLASMA) {
    float glow = 0.0;
    for (int dy = -1; dy <= 1; dy++) {
      for (int dx = -1; dx <= 1; dx++) {
        if (dx == 0 && dy == 0) continue;
        uint nm = sampleMat(cellCoord + vec2(float(dx), float(dy)));
        if (nm == MAT_FIRE || nm == MAT_LAVA || nm == MAT_EMBER) {
          glow += 0.16 / length(vec2(float(dx), float(dy)));
        } else if (nm == MAT_PLASMA) {
          // Plasma throws far more light than fire.
          glow += 0.30 / length(vec2(float(dx), float(dy)));
        }
      }
    }
    glow = min(glow, 0.6);
    col += vec3(glow, glow * 0.45, glow * 0.12);
  }

  // ── Vignette ────────────────────────────────────────────────────────────
  vec2 vc = v_uv - 0.5;
  col *= 1.0 - dot(vc, vc) * 0.28;

  // The view is a single opaque layer; translucency was already composited
  // against the sky or cave background above.
  fragColor = vec4(clamp(col, 0.0, 1.0), 1.0);
}`;

// ── Palette ────────────────────────────────────────────────────────────────

function buildPaletteData(): Uint8Array {
  const data = new Uint8Array(256 * 4);
  const defs: [number, number, number, number, number][] = [
    [ 0,  13,  13,  16, 255],  // Empty
    [ 1,  80,  75,  70, 255],  // Rock
    [ 2, 101,  67,  33, 255],  // Soil
    [ 3, 210, 180, 100, 255],  // Sand
    [ 4,  30, 100, 200, 220],  // Water
    [ 5,  50, 140,  50, 255],  // Plant
    [ 6, 240, 100,  20, 255],  // Fire
    [ 7, 180, 210, 240, 120],  // Vapor
    [ 8,  80,  80,  80, 160],  // Smoke
    [ 9, 255,  80,   0, 255],  // Lava
    [10, 180, 230, 255, 255],  // Ice
    [11,  50,  45,  40, 200],  // Ash
    [12,  80,  60,  20, 255],  // Oil
    [13, 200,  50,  10, 180],  // Ember
    [14, 100, 200,  60, 255],  // Herbivore
    [15, 220,  50,  50, 255],  // Predator
    [16, 200, 200, 220, 180],  // Cloud
    [17,  18,   6,  30, 255],  // Void — near-black with a violet cast
    [18, 130, 245,  90, 210],  // Radiation — sickly green
    [19, 190, 130, 255, 255],  // Plasma — violet-white
    [20,  92,  58,  52, 255],  // Carrion — dull red-brown
    [21, 238, 232, 220, 255],  // Sheep — off-white fleece
    [22,  92, 168,  62, 255],  // Grass — lighter and yellower than woody plants
  ];
  for (const [id, r, g, b, a] of defs) {
    data[id * 4]     = r;
    data[id * 4 + 1] = g;
    data[id * 4 + 2] = b;
    data[id * 4 + 3] = a;
  }
  return data;
}

// ── WebGL2 helper functions ────────────────────────────────────────────────

function compileShader(gl: WebGL2RenderingContext, type: number, src: string): WebGLShader {
  const s = gl.createShader(type)!;
  gl.shaderSource(s, src);
  gl.compileShader(s);
  if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(s);
    gl.deleteShader(s);
    throw new Error("Shader compile error: " + log);
  }
  return s;
}

function createProgram(gl: WebGL2RenderingContext, vert: string, frag: string): WebGLProgram {
  const p = gl.createProgram()!;
  const vs = compileShader(gl, gl.VERTEX_SHADER, vert);
  const fs = compileShader(gl, gl.FRAGMENT_SHADER, frag);
  gl.attachShader(p, vs);
  gl.attachShader(p, fs);
  gl.linkProgram(p);
  if (!gl.getProgramParameter(p, gl.LINK_STATUS)) {
    throw new Error("WebGL2 program link error: " + gl.getProgramInfoLog(p));
  }
  return p;
}

// ── WebGL2WorldRenderer adapter ────────────────────────────────────────────

/**
 * Drop-in replacement for WorldRenderer that uses WebGL2 for rendering.
 * Same public API: viewX, viewY, initWorld(), applySnapshot(), etc.
 */
export class WebGL2WorldRenderer implements IWorldRenderer {
  private gl: WebGL2RenderingContext;
  private program: WebGLProgram;
  private matTex: WebGLTexture;
  private paletteTex: WebGLTexture;
  private vao: WebGLVertexArrayObject;
  private animFrame: number = 0;

  /** Current viewport origin (same as Canvas2D renderer) */
  viewX = 0;
  viewY = 0;

  /** World dimensions */
  worldW = 0;
  worldH = 0;

  /** Chunk size in cells */
  chunkSize = 64;

  /** Zoom scale in screen pixels per world cell. */
  zoom = 1;

  /**
   * Smallest zoom that still covers the canvas with world content.
   *
   * Zooming out past this would leave empty margins around the map, so it acts
   * as the lower bound and as the initial zoom level.
   */
  private fitZoom = 1;

  /** How far the player may zoom in relative to the fitted level. */
  private static readonly MAX_ZOOM_FACTOR = 16;

  /** How many world cells are visible horizontally */
  get visibleW(): number {
    return Math.ceil(this.canvas.width / this.zoom);
  }
  /** How many world cells are visible vertically */
  get visibleH(): number {
    return Math.ceil(this.canvas.height / this.zoom);
  }

  /** Local material cache for minimap/prediction compatibility */
  private materialCache: Uint8Array | null = null;

  /**
   * Whether anything in the world is currently emitting light.
   *
   * Drives the shader's light-bleed pass, which is skipped when false. Updated
   * from outside on a slow interval rather than per frame.
   */
  private hasEmissive = false;

  /** Reports whether fire, lava or embers are present, enabling light bleed. */
  setHasEmissive(present: boolean): void {
    this.hasEmissive = present;
  }

  /** Uniform locations (cached after first use) */
  private locs: Record<string, WebGLUniformLocation | null> = {};

  constructor(private readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext("webgl2", { antialias: false, alpha: false });
    if (!gl) throw new Error("WebGL2 not available");
    this.gl = gl;
    // Material textures use one byte per cell. The default unpack alignment of
    // four makes WebGL calculate the wrong row stride for narrow/sub-region
    // uploads, which can reject a valid snapshot and leave chunk-shaped holes.
    gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);

    this.program = createProgram(gl, VERT_SRC, FRAG_SRC);
    this.vao = this.setupQuad();
    this.matTex = this.createEmptyMaterialTexture(1, 1);
    this.paletteTex = this.createPaletteTexture();

    // Cache uniform locations
    const uniforms = [
      "u_material", "u_palette", "u_worldSize",
      "u_camera", "u_viewport", "u_time", "u_hasEmissive",
    ];
    for (const u of uniforms) {
      this.locs[u] = gl.getUniformLocation(this.program, u);
    }

    this.startRenderLoop();
  }

  /** Called when the server sends world dimensions. */
  initWorld(w: number, h: number): void {
    this.worldW = w;
    this.worldH = h;
    this.materialCache = new Uint8Array(w * h);

    // Recreate material texture at correct size
    const gl = this.gl;
    gl.deleteTexture(this.matTex);
    this.matTex = this.createEmptyMaterialTexture(w, h);

    this.recomputeFitZoom();
    this.clampCamera();
  }

  /**
   * Recomputes the zoom needed to cover the canvas with world content.
   *
   * A 1024x512 world shown 1:1 on a wider canvas leaves the map floating in
   * empty space, so the view is scaled to cover whichever axis is tighter.
   */
  private recomputeFitZoom(): void {
    if (this.worldW === 0 || this.worldH === 0) return;
    if (this.canvas.width === 0 || this.canvas.height === 0) return;

    const previous = this.fitZoom;
    this.fitZoom = Math.max(
      this.canvas.width / this.worldW,
      this.canvas.height / this.worldH,
    );

    // Keep the player's relative zoom across resizes; snap to fit on first run.
    this.zoom = previous > 0 && this.zoom > previous
      ? Math.max(this.fitZoom, this.zoom * (this.fitZoom / previous))
      : this.fitZoom;
  }

  /** Keeps the viewport inside the world so no empty margin is shown. */
  private clampCamera(): void {
    const maxX = Math.max(0, this.worldW - this.visibleW);
    const maxY = Math.max(0, this.worldH - this.visibleH);
    this.viewX = Math.min(Math.max(0, this.viewX), maxX);
    this.viewY = Math.min(Math.max(0, this.viewY), maxY);
  }

  /** Step zoom in or out. Returns new zoom level. */
  stepZoom(direction: 1 | -1): number {
    const max = this.fitZoom * WebGL2WorldRenderer.MAX_ZOOM_FACTOR;
    const next = direction > 0 ? this.zoom * 2 : this.zoom / 2;
    // Never zoom out past the fitted level, which would expose empty margins.
    this.zoom = Math.min(max, Math.max(this.fitZoom, next));
    this.clampCamera();
    return this.zoom;
  }

  /** Apply a full viewport snapshot from the server. */
  applySnapshot(snap: FullSnapshot): void {
    if (!this.materialCache) return;

    // Update CPU-side cache (for minimap/prediction)
    for (let row = 0; row < snap.h; row++) {
      for (let col = 0; col < snap.w; col++) {
        const wx = snap.x + col;
        const wy = snap.y + row;
        if (wx >= 0 && wx < this.worldW && wy >= 0 && wy < this.worldH) {
          this.materialCache[wy * this.worldW + wx] = snap.data[row * snap.w + col];
        }
      }
    }

    // Upload to GPU texture
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    // Clamp dimensions to world bounds
    const uploadW = Math.min(snap.w, this.worldW - snap.x);
    const uploadH = Math.min(snap.h, this.worldH - snap.y);
    if (uploadW > 0 && uploadH > 0 && uploadW === snap.w && uploadH === snap.h) {
      gl.texSubImage2D(
        gl.TEXTURE_2D, 0,
        snap.x, snap.y,
        snap.w, snap.h,
        gl.RED_INTEGER, gl.UNSIGNED_BYTE,
        snap.data,
      );
    } else if (uploadW > 0 && uploadH > 0) {
      // Need to copy a sub-region
      const subData = new Uint8Array(uploadW * uploadH);
      for (let r = 0; r < uploadH; r++) {
        for (let c = 0; c < uploadW; c++) {
          subData[r * uploadW + c] = snap.data[r * snap.w + c];
        }
      }
      gl.texSubImage2D(
        gl.TEXTURE_2D, 0,
        snap.x, snap.y,
        uploadW, uploadH,
        gl.RED_INTEGER, gl.UNSIGNED_BYTE,
        subData,
      );
    }
  }

  /** Apply a list of dirty chunk updates from the server. */
  applyChunkUpdates(updates: ChunkUpdate[]): void {
    if (!this.materialCache) return;
    const gl = this.gl;
    const cs = this.chunkSize;

    gl.bindTexture(gl.TEXTURE_2D, this.matTex);

    for (const u of updates) {
      const cx0 = u.cx * cs;
      const cy0 = u.cy * cs;
      const cw = Math.min(cs, this.worldW - cx0);
      const ch = Math.min(cs, this.worldH - cy0);
      if (cw <= 0 || ch <= 0) continue;

      // Update CPU cache
      let idx = 0;
      for (let y = cy0; y < cy0 + ch; y++) {
        for (let x = cx0; x < cx0 + cw; x++) {
          this.materialCache[y * this.worldW + x] = u.data[idx++];
        }
      }

      // Upload chunk to GPU
      // If chunk size matches exactly, upload directly
      if (cw === cs && ch === cs) {
        gl.texSubImage2D(
          gl.TEXTURE_2D, 0,
          cx0, cy0, cw, ch,
          gl.RED_INTEGER, gl.UNSIGNED_BYTE,
          u.data,
        );
      } else {
        // Edge chunk: upload trimmed sub-region
        const trimmed = new Uint8Array(cw * ch);
        for (let r = 0; r < ch; r++) {
          for (let c = 0; c < cw; c++) {
            trimmed[r * cw + c] = u.data[r * cs + c];
          }
        }
        gl.texSubImage2D(
          gl.TEXTURE_2D, 0,
          cx0, cy0, cw, ch,
          gl.RED_INTEGER, gl.UNSIGNED_BYTE,
          trimmed,
        );
      }
    }
  }

  /** Resize — update viewport after canvas dimension change. */
  onResize(): void {
    this.gl.viewport(0, 0, this.canvas.width, this.canvas.height);
    // The canvas aspect changed, so the zoom that covers it changed too.
    this.recomputeFitZoom();
    this.clampCamera();
  }

  /** Expose material cache for minimap rendering and client-side prediction. */
  getMaterialCache(): Uint8Array | null {
    return this.materialCache;
  }

  /** Public draw trigger for client-side prediction (immediate feedback). */
  drawImmediate(): void {
    // The render loop handles drawing; prediction just needs to update
    // the material cache (which it does via getMaterialCache), and the
    // next frame picks it up automatically. But we should re-upload the
    // full texture if prediction modified the cache.
    if (!this.materialCache || this.worldW === 0) return;
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    gl.texSubImage2D(
      gl.TEXTURE_2D, 0,
      0, 0, this.worldW, this.worldH,
      gl.RED_INTEGER, gl.UNSIGNED_BYTE,
      this.materialCache,
    );
  }

  dispose(): void {
    cancelAnimationFrame(this.animFrame);
    const gl = this.gl;
    gl.deleteProgram(this.program);
    gl.deleteTexture(this.matTex);
    gl.deleteTexture(this.paletteTex);
  }

  // ── Render loop ──────────────────────────────────────────────────────────

  private startRenderLoop(): void {
    const loop = (time: number) => {
      this.render(time);
      this.animFrame = requestAnimationFrame(loop);
    };
    this.animFrame = requestAnimationFrame(loop);
  }

  private render(time: number): void {
    const gl = this.gl;
    const cw = this.canvas.width;
    const ch = this.canvas.height;

    if (cw === 0 || ch === 0 || this.worldW === 0) return;

    gl.viewport(0, 0, cw, ch);
    gl.clearColor(0.05, 0.05, 0.063, 1.0);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(this.program);

    // Bind material texture
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    gl.uniform1i(this.locs["u_material"]!, 0);

    // Bind palette texture
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, this.paletteTex);
    gl.uniform1i(this.locs["u_palette"]!, 1);

    // Set uniforms
    gl.uniform2f(this.locs["u_worldSize"]!, this.worldW, this.worldH);
    gl.uniform2f(this.locs["u_camera"]!, this.viewX, this.viewY);
    gl.uniform2f(this.locs["u_viewport"]!, this.visibleW, this.visibleH);
    gl.uniform1f(this.locs["u_time"]!, time * 0.001);
    gl.uniform1i(this.locs["u_hasEmissive"]!, this.hasEmissive ? 1 : 0);

    // Draw fullscreen quad
    gl.bindVertexArray(this.vao);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    gl.bindVertexArray(null);
  }

  // ── GPU resource setup ───────────────────────────────────────────────────

  private setupQuad(): WebGLVertexArrayObject {
    const gl = this.gl;
    const vao = gl.createVertexArray()!;
    gl.bindVertexArray(vao);

    const buf = gl.createBuffer()!;
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
      -1, -1,  1, -1,  -1, 1,  1, 1,
    ]), gl.STATIC_DRAW);

    const loc = gl.getAttribLocation(this.program, "a_pos");
    gl.enableVertexAttribArray(loc);
    gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);

    gl.bindVertexArray(null);
    return vao;
  }

  private createEmptyMaterialTexture(w: number, h: number): WebGLTexture {
    const gl = this.gl;
    const tex = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(
      gl.TEXTURE_2D, 0, gl.R8UI,
      w, h, 0,
      gl.RED_INTEGER, gl.UNSIGNED_BYTE,
      new Uint8Array(w * h),
    );
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return tex;
  }

  private createPaletteTexture(): WebGLTexture {
    const gl = this.gl;
    const tex = gl.createTexture()!;
    const data = buildPaletteData();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(
      gl.TEXTURE_2D, 0, gl.RGBA,
      256, 1, 0,
      gl.RGBA, gl.UNSIGNED_BYTE,
      data,
    );
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return tex;
  }
}

// ── Feature detection ──────────────────────────────────────────────────────

/** Returns true if WebGL2 is available in this browser. */
export function isWebGL2Available(): boolean {
  try {
    const probe = document.createElement("canvas");
    const ctx = probe.getContext("webgl2");
    return ctx !== null;
  } catch {
    return false;
  }
}
