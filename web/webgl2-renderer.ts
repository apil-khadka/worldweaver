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

// ── Utilities ──────────────────────────────────────────────────────────────

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

uint sampleMat(vec2 worldPos) {
  vec2 uv = worldPos / u_worldSize;
  if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) return 0u;
  return texture(u_material, uv).r;
}

void main() {
  // Map fragment to world coordinate
  vec2 worldPos = u_camera + v_uv * u_viewport;
  vec2 cellCoord = floor(worldPos);
  vec2 cellUV = (cellCoord + 0.5) / u_worldSize;

  // Bounds check
  if (cellUV.x < 0.0 || cellUV.x > 1.0 || cellUV.y < 0.0 || cellUV.y > 1.0) {
    fragColor = vec4(0.05, 0.05, 0.063, 1.0);
    return;
  }

  // Sample material ID
  uint matID = texture(u_material, cellUV).r;

  // Base colour from palette
  vec4 col = texture(u_palette, vec2((float(matID) + 0.5) / 256.0, 0.5));

  // ── Per-cell color variation ─────────────────────────────────────────────
  float noise = hash(cellCoord) * 0.08 - 0.04;
  col.rgb += noise;

  // ── Depth shading (lower = darker, simulates underground) ────────────────
  float depth = cellCoord.y / u_worldSize.y;
  float depthDarken = mix(1.0, 0.6, depth);
  col.rgb *= depthDarken;

  // ── Water shimmer ────────────────────────────────────────────────────────
  if (matID == 4u) {
    float wave1 = sin(cellCoord.x * 0.3 + u_time * 2.0) * 0.05;
    float wave2 = sin(cellCoord.y * 0.5 + u_time * 1.3) * 0.03;
    float shimmer = wave1 + wave2;
    float sparkle = hash(cellCoord + floor(u_time * 3.0)) * 0.06;
    col.rgb += vec3(shimmer * 0.15 + sparkle, shimmer * 0.3 + sparkle, shimmer * 0.6 + sparkle * 1.5);
  }

  // ── Fire glow (brighten adjacent pixels) ─────────────────────────────────
  if (matID == 6u) {
    // Self-flicker
    float flicker = hash(cellCoord + floor(u_time * 10.0));
    col.rgb = mix(col.rgb, vec3(1.0, 0.5, 0.0), flicker * 0.4);
  } else {
    // Check neighbours for fire glow bleed
    float glow = 0.0;
    for (int dy = -1; dy <= 1; dy++) {
      for (int dx = -1; dx <= 1; dx++) {
        if (dx == 0 && dy == 0) continue;
        vec2 np = cellCoord + vec2(float(dx), float(dy));
        uint nm = sampleMat(np);
        if (nm == 6u) {
          float dist = length(vec2(float(dx), float(dy)));
          glow += 0.12 / dist;
        }
      }
    }
    glow = min(glow, 0.4);
    col.rgb += vec3(glow * 0.8, glow * 0.3, glow * 0.05);
  }

  col.rgb = clamp(col.rgb, 0.0, 1.0);
  fragColor = col;
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

  /** Zoom scale — 0.5, 1, 2, or 4 */
  zoom = 1;

  /** Discrete zoom levels */
  private static readonly ZOOM_LEVELS = [0.5, 1, 2, 4];

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

  /** Uniform locations (cached after first use) */
  private locs: Record<string, WebGLUniformLocation | null> = {};

  constructor(private readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext("webgl2", { antialias: false, alpha: false });
    if (!gl) throw new Error("WebGL2 not available");
    this.gl = gl;

    this.program = createProgram(gl, VERT_SRC, FRAG_SRC);
    this.vao = this.setupQuad();
    this.matTex = this.createEmptyMaterialTexture(1, 1);
    this.paletteTex = this.createPaletteTexture();

    // Cache uniform locations
    const uniforms = [
      "u_material", "u_palette", "u_worldSize",
      "u_camera", "u_viewport", "u_time",
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
  }

  /** Step zoom in or out. Returns new zoom level. */
  stepZoom(direction: 1 | -1): number {
    const levels = WebGL2WorldRenderer.ZOOM_LEVELS;
    const curIdx = levels.indexOf(this.zoom);
    const newIdx = Math.max(0, Math.min(levels.length - 1, curIdx + direction));
    this.zoom = levels[newIdx];
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
