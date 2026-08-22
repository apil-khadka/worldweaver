/**
 * isometric-renderer.ts — WebGL2 Isometric 2.5D Renderer
 *
 * Renders the flat 2D material grid as isometric tiles with height variation.
 * Uses instanced rendering: one draw call for all visible tiles.
 *
 * Each material type has an intrinsic height, creating a 2.5D depth illusion.
 * The underlying simulation data remains a flat 2D grid — only rendering changes.
 */

import type { FullSnapshot, ChunkUpdate, IWorldRenderer } from "./renderer.js";

// ── Material Heights ───────────────────────────────────────────────────────

const MAT_HEIGHT: number[] = [
  /* 0  Empty */     0,
  /* 1  Rock */      3,
  /* 2  Soil */      2,
  /* 3  Sand */      1,
  /* 4  Water */     0,
  /* 5  Plant */     2,
  /* 6  Fire */      2,
  /* 7  Vapor */     0,
  /* 8  Smoke */     0,
  /* 9  Lava */      1,
  /* 10 Ice */       1,
  /* 11 Ash */       1,
  /* 12 Oil */       0,
  /* 13 Ember */     1,
  /* 14 Herbivore */ 1,
  /* 15 Predator */  1,
  /* 16 Cloud */     0,
];

// ── Material Colors (top face RGB, side face RGB) ──────────────────────────

interface MatColor {
  topR: number; topG: number; topB: number;
  sideR: number; sideG: number; sideB: number;
  alpha: number;
}

const MAT_COLORS: MatColor[] = [
  /* 0  Empty */     { topR: 0, topG: 0, topB: 0, sideR: 0, sideG: 0, sideB: 0, alpha: 0 },
  /* 1  Rock */      { topR: 0.35, topG: 0.32, topB: 0.30, sideR: 0.22, sideG: 0.20, sideB: 0.18, alpha: 1 },
  /* 2  Soil */      { topR: 0.40, topG: 0.26, topB: 0.13, sideR: 0.28, sideG: 0.18, sideB: 0.09, alpha: 1 },
  /* 3  Sand */      { topR: 0.82, topG: 0.71, topB: 0.39, sideR: 0.60, sideG: 0.50, sideB: 0.28, alpha: 1 },
  /* 4  Water */     { topR: 0.12, topG: 0.39, topB: 0.78, sideR: 0.08, sideG: 0.28, sideB: 0.60, alpha: 0.85 },
  /* 5  Plant */     { topR: 0.20, topG: 0.60, topB: 0.20, sideR: 0.14, sideG: 0.38, sideB: 0.14, alpha: 1 },
  /* 6  Fire */      { topR: 0.94, topG: 0.39, topB: 0.08, sideR: 0.70, sideG: 0.20, sideB: 0.04, alpha: 1 },
  /* 7  Vapor */     { topR: 0.71, topG: 0.82, topB: 0.94, sideR: 0.55, sideG: 0.65, sideB: 0.75, alpha: 0.5 },
  /* 8  Smoke */     { topR: 0.31, topG: 0.31, topB: 0.31, sideR: 0.22, sideG: 0.22, sideB: 0.22, alpha: 0.6 },
  /* 9  Lava */      { topR: 1.0, topG: 0.31, topB: 0.0, sideR: 0.80, sideG: 0.15, sideB: 0.0, alpha: 1 },
  /* 10 Ice */       { topR: 0.71, topG: 0.90, topB: 1.0, sideR: 0.50, sideG: 0.70, sideB: 0.85, alpha: 1 },
  /* 11 Ash */       { topR: 0.20, topG: 0.18, topB: 0.16, sideR: 0.14, sideG: 0.12, sideB: 0.10, alpha: 0.8 },
  /* 12 Oil */       { topR: 0.31, topG: 0.24, topB: 0.08, sideR: 0.22, sideG: 0.16, sideB: 0.05, alpha: 1 },
  /* 13 Ember */     { topR: 0.78, topG: 0.20, topB: 0.04, sideR: 0.55, sideG: 0.12, sideB: 0.02, alpha: 0.7 },
  /* 14 Herbivore */ { topR: 0.39, topG: 0.78, topB: 0.24, sideR: 0.28, sideG: 0.55, sideB: 0.16, alpha: 1 },
  /* 15 Predator */  { topR: 0.86, topG: 0.20, topB: 0.20, sideR: 0.60, sideG: 0.12, sideB: 0.12, alpha: 1 },
  /* 16 Cloud */     { topR: 0.78, topG: 0.78, topB: 0.86, sideR: 0.60, sideG: 0.60, sideB: 0.68, alpha: 0.7 },
];

// ── Shader Sources ─────────────────────────────────────────────────────────

const ISO_VERT_SRC = /* glsl */ `#version 300 es
precision highp float;

// Per-vertex (tile quad mesh: top face + left face + right face)
in vec3 a_pos;       // local mesh vertex
in vec3 a_normal;    // face normal for lighting
in float a_faceID;   // 0=top, 1=left-side, 2=right-side

// Per-instance
in vec2 a_cellPos;   // grid x, y
in float a_matID;    // material ID
in float a_height;   // tile height

// Uniforms
uniform vec2 u_viewOffset;   // camera pan offset
uniform float u_scale;       // zoom scale
uniform vec2 u_resolution;   // canvas size
uniform float u_time;        // animation time

// Output
out vec3 v_normal;
out float v_faceID;
flat out int v_matID;
out vec2 v_worldPos;
out float v_height;

void main() {
  int matID = int(a_matID);
  float height = a_height;

  // Fire: animated flickering height
  if (matID == 6) {
    float flicker = sin(u_time * 8.0 + a_cellPos.x * 3.7 + a_cellPos.y * 2.3) * 0.3
                  + sin(u_time * 12.0 + a_cellPos.x * 5.1) * 0.2;
    height += flicker;
  }

  // Water: subtle animated wave
  if (matID == 4) {
    float wave = sin(u_time * 2.0 + a_cellPos.x * 0.5 + a_cellPos.y * 0.3) * 0.15;
    height += wave;
  }

  // Scale tile height in mesh
  vec3 pos = a_pos;
  pos.y *= height;

  // Isometric projection:
  //   screenX = (cellX - cellY) * tileWidth/2
  //   screenY = (cellX + cellY) * tileHeight/4 - elevation * elevationScale
  float tileW = 16.0;
  float tileH = 8.0;
  float elevScale = 6.0;

  float isoX = (a_cellPos.x - a_cellPos.y) * tileW * 0.5;
  float isoY = (a_cellPos.x + a_cellPos.y) * tileH * 0.25 - pos.y * elevScale;

  // Add local vertex offset (tile mesh)
  isoX += pos.x * tileW * 0.5;
  isoY += pos.z * tileH * 0.25 - pos.y * elevScale;

  // Apply camera transform
  vec2 screenPos = vec2(isoX, isoY);
  screenPos -= u_viewOffset;
  screenPos *= u_scale;

  // Convert to NDC
  vec2 ndc = (screenPos / u_resolution) * 2.0;

  gl_Position = vec4(ndc.x, -ndc.y, 0.0, 1.0);

  v_normal = a_normal;
  v_faceID = a_faceID;
  v_matID = matID;
  v_worldPos = a_cellPos;
  v_height = height;
}`;

const ISO_FRAG_SRC = /* glsl */ `#version 300 es
precision highp float;

in vec3 v_normal;
in float v_faceID;
flat in int v_matID;
in vec2 v_worldPos;
in float v_height;

uniform float u_time;
uniform float u_dayNight; // 0.0 = midnight, 1.0 = noon

out vec4 fragColor;

// Material colors (top and side)
struct MatColor {
  vec3 top;
  vec3 side;
  float alpha;
};

MatColor getMatColor(int id) {
  // Hardcoded lookup since GLSL ES 3.0 doesn't have SSBOs easily
  MatColor c;
  c.alpha = 1.0;

  if (id == 0) { c.top = vec3(0.0); c.side = vec3(0.0); c.alpha = 0.0; }
  else if (id == 1)  { c.top = vec3(0.35, 0.32, 0.30); c.side = vec3(0.22, 0.20, 0.18); }
  else if (id == 2)  { c.top = vec3(0.40, 0.26, 0.13); c.side = vec3(0.28, 0.18, 0.09); }
  else if (id == 3)  { c.top = vec3(0.82, 0.71, 0.39); c.side = vec3(0.60, 0.50, 0.28); }
  else if (id == 4)  { c.top = vec3(0.12, 0.39, 0.78); c.side = vec3(0.08, 0.28, 0.60); c.alpha = 0.85; }
  else if (id == 5)  { c.top = vec3(0.20, 0.60, 0.20); c.side = vec3(0.14, 0.38, 0.14); }
  else if (id == 6)  { c.top = vec3(0.94, 0.39, 0.08); c.side = vec3(0.70, 0.20, 0.04); }
  else if (id == 7)  { c.top = vec3(0.71, 0.82, 0.94); c.side = vec3(0.55, 0.65, 0.75); c.alpha = 0.5; }
  else if (id == 8)  { c.top = vec3(0.31, 0.31, 0.31); c.side = vec3(0.22, 0.22, 0.22); c.alpha = 0.6; }
  else if (id == 9)  { c.top = vec3(1.0, 0.31, 0.0); c.side = vec3(0.80, 0.15, 0.0); }
  else if (id == 10) { c.top = vec3(0.71, 0.90, 1.0); c.side = vec3(0.50, 0.70, 0.85); }
  else if (id == 11) { c.top = vec3(0.20, 0.18, 0.16); c.side = vec3(0.14, 0.12, 0.10); c.alpha = 0.8; }
  else if (id == 12) { c.top = vec3(0.31, 0.24, 0.08); c.side = vec3(0.22, 0.16, 0.05); }
  else if (id == 13) { c.top = vec3(0.78, 0.20, 0.04); c.side = vec3(0.55, 0.12, 0.02); c.alpha = 0.7; }
  else if (id == 14) { c.top = vec3(0.39, 0.78, 0.24); c.side = vec3(0.28, 0.55, 0.16); }
  else if (id == 15) { c.top = vec3(0.86, 0.20, 0.20); c.side = vec3(0.60, 0.12, 0.12); }
  else if (id == 16) { c.top = vec3(0.78, 0.78, 0.86); c.side = vec3(0.60, 0.60, 0.68); c.alpha = 0.7; }
  else { c.top = vec3(0.5); c.side = vec3(0.3); }

  return c;
}

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

void main() {
  if (v_matID == 0) discard; // Empty cells are invisible

  MatColor mc = getMatColor(v_matID);

  // Select face color
  vec3 col;
  if (v_faceID < 0.5) {
    // Top face — brightest (sky-facing)
    col = mc.top;
  } else if (v_faceID < 1.5) {
    // Left side — medium shadow
    col = mc.side * 0.85;
  } else {
    // Right side — darkest shadow
    col = mc.side * 0.65;
  }

  // Per-cell noise variation
  float noise = hash(v_worldPos) * 0.06 - 0.03;
  col += noise;

  // Plant: green top extension
  if (v_matID == 5 && v_faceID < 0.5) {
    float greenBoost = hash(v_worldPos + 42.0) * 0.1;
    col.g += greenBoost + 0.05;
  }

  // Fire: animated color flickering
  if (v_matID == 6) {
    float flicker = hash(v_worldPos + floor(u_time * 10.0)) * 0.3;
    col = mix(col, vec3(1.0, 0.6, 0.0), flicker);
    if (v_faceID < 0.5) {
      // Fire tips glow on top
      float tipGlow = sin(u_time * 6.0 + v_worldPos.x * 4.0) * 0.5 + 0.5;
      col = mix(col, vec3(1.0, 0.9, 0.2), tipGlow * 0.3);
    }
  }

  // Lava: slow pulse
  if (v_matID == 9) {
    float pulse = sin(u_time * 3.0 + v_worldPos.x + v_worldPos.y) * 0.5 + 0.5;
    col = mix(col, vec3(1.0, 0.5, 0.0), pulse * 0.2);
  }

  // Water: reflection shimmer
  if (v_matID == 4 && v_faceID < 0.5) {
    float wave1 = sin(v_worldPos.x * 0.8 + u_time * 2.0) * 0.03;
    float wave2 = sin(v_worldPos.y * 0.6 + u_time * 1.5) * 0.02;
    float sparkle = hash(v_worldPos + floor(u_time * 4.0)) * 0.08;
    col += vec3(wave1 + sparkle * 0.3, wave2 + sparkle * 0.5, (wave1 + wave2) * 2.0 + sparkle);
  }

  // Ice: subtle shimmer
  if (v_matID == 10 && v_faceID < 0.5) {
    float shimmer = hash(v_worldPos + floor(u_time * 2.0)) * 0.05;
    col += shimmer;
  }

  // Day/night cycle: affects global brightness
  float dayLight = mix(0.4, 1.0, u_dayNight);
  col *= dayLight;

  // Creatures: dot on top
  if ((v_matID == 14 || v_matID == 15) && v_faceID < 0.5) {
    col = mix(col, v_matID == 14 ? vec3(0.2, 1.0, 0.3) : vec3(1.0, 0.2, 0.2), 0.4);
  }

  col = clamp(col, 0.0, 1.0);
  fragColor = vec4(col, mc.alpha);
}`;

// ── Isometric tile mesh geometry ───────────────────────────────────────────

interface TileMesh {
  vertices: Float32Array;  // x, y, z, nx, ny, nz, faceID per vertex
  indices: Uint16Array;
}

function buildTileMesh(): TileMesh {
  // An isometric tile is a diamond from above.
  // We build: top face (diamond), left-side face, right-side face.
  // All coordinates are in tile-local space where:
  //   x: -1 to 1 (width), y: 0 to 1 (height/elevation), z: -1 to 1 (depth)

  const verts: number[] = [];
  const idxs: number[] = [];
  let vi = 0;

  // Helper to push a vertex
  function v(x: number, y: number, z: number, nx: number, ny: number, nz: number, faceID: number) {
    verts.push(x, y, z, nx, ny, nz, faceID);
    return vi++;
  }

  // TOP FACE (diamond shape, y=1 plane — the top of the tile)
  // Diamond vertices: N(0,-1), E(1,0), S(0,1), W(-1,0)
  const tN = v(0, 1, -1, 0, 1, 0, 0);
  const tE = v(1, 1, 0, 0, 1, 0, 0);
  const tS = v(0, 1, 1, 0, 1, 0, 0);
  const tW = v(-1, 1, 0, 0, 1, 0, 0);
  // Two triangles for diamond
  idxs.push(tN, tE, tS);
  idxs.push(tN, tS, tW);

  // LEFT SIDE FACE (facing left-down in iso view)
  // From W-bottom to S-bottom, elevated to W-top and S-top
  const lTL = v(-1, 1, 0, -0.7, 0, 0.7, 1);  // W top
  const lBL = v(-1, 0, 0, -0.7, 0, 0.7, 1);  // W bottom
  const lTR = v(0, 1, 1, -0.7, 0, 0.7, 1);   // S top
  const lBR = v(0, 0, 1, -0.7, 0, 0.7, 1);   // S bottom
  idxs.push(lTL, lBL, lBR);
  idxs.push(lTL, lBR, lTR);

  // RIGHT SIDE FACE (facing right-down in iso view)
  // From S-bottom to E-bottom, elevated to S-top and E-top
  const rTL = v(0, 1, 1, 0.7, 0, 0.7, 2);   // S top
  const rBL = v(0, 0, 1, 0.7, 0, 0.7, 2);   // S bottom
  const rTR = v(1, 1, 0, 0.7, 0, 0.7, 2);   // E top
  const rBR = v(1, 0, 0, 0.7, 0, 0.7, 2);   // E bottom
  idxs.push(rTL, rBL, rBR);
  idxs.push(rTL, rBR, rTR);

  return {
    vertices: new Float32Array(verts),
    indices: new Uint16Array(idxs),
  };
}

// ── GL Helpers ─────────────────────────────────────────────────────────────

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
    throw new Error("Program link error: " + gl.getProgramInfoLog(p));
  }
  return p;
}

// ── Isometric Renderer Class ───────────────────────────────────────────────

export class IsometricRenderer implements IWorldRenderer {
  private gl: WebGL2RenderingContext;
  private program: WebGLProgram;
  private vao: WebGLVertexArrayObject;
  private instanceBuffer: WebGLBuffer;
  private indexCount: number;
  private instanceCount = 0;
  private animFrame = 0;
  private instanceData: Float32Array | null = null;
  private dirty = true;

  /** Current viewport origin (isometric pan offset) */
  viewX = 0;
  viewY = 0;

  /** World dimensions */
  worldW = 0;
  worldH = 0;

  /** Chunk size */
  chunkSize = 64;

  /** Zoom scale */
  zoom = 1;

  /** Discrete zoom levels */
  private static readonly ZOOM_LEVELS = [0.25, 0.5, 1, 2, 4];

  /** Local material cache */
  private materialCache: Uint8Array | null = null;

  /** Uniform locations */
  private locs: Record<string, WebGLUniformLocation | null> = {};

  /** How many world cells are visible (approximate for iso) */
  get visibleW(): number {
    return Math.ceil(this.canvas.width / (16 * this.zoom)) * 2;
  }
  get visibleH(): number {
    return Math.ceil(this.canvas.height / (8 * this.zoom)) * 2;
  }

  constructor(private readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext("webgl2", { antialias: true, alpha: false });
    if (!gl) throw new Error("WebGL2 not available");
    this.gl = gl;

    this.program = createProgram(gl, ISO_VERT_SRC, ISO_FRAG_SRC);

    // Build tile mesh
    const mesh = buildTileMesh();
    this.indexCount = mesh.indices.length;

    // Create VAO
    this.vao = gl.createVertexArray()!;
    gl.bindVertexArray(this.vao);

    // Vertex buffer (per-vertex data)
    const vertBuf = gl.createBuffer()!;
    gl.bindBuffer(gl.ARRAY_BUFFER, vertBuf);
    gl.bufferData(gl.ARRAY_BUFFER, mesh.vertices, gl.STATIC_DRAW);

    const stride = 7 * 4; // 7 floats per vertex
    // a_pos (vec3)
    const posLoc = gl.getAttribLocation(this.program, "a_pos");
    gl.enableVertexAttribArray(posLoc);
    gl.vertexAttribPointer(posLoc, 3, gl.FLOAT, false, stride, 0);
    // a_normal (vec3)
    const normLoc = gl.getAttribLocation(this.program, "a_normal");
    gl.enableVertexAttribArray(normLoc);
    gl.vertexAttribPointer(normLoc, 3, gl.FLOAT, false, stride, 12);
    // a_faceID (float)
    const faceLoc = gl.getAttribLocation(this.program, "a_faceID");
    gl.enableVertexAttribArray(faceLoc);
    gl.vertexAttribPointer(faceLoc, 1, gl.FLOAT, false, stride, 24);

    // Index buffer
    const idxBuf = gl.createBuffer()!;
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, idxBuf);
    gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, mesh.indices, gl.STATIC_DRAW);

    // Instance buffer (per-instance: cellX, cellY, matID, height)
    this.instanceBuffer = gl.createBuffer()!;
    gl.bindBuffer(gl.ARRAY_BUFFER, this.instanceBuffer);
    // Allocate empty initially
    gl.bufferData(gl.ARRAY_BUFFER, 4, gl.DYNAMIC_DRAW);

    const instStride = 4 * 4; // 4 floats
    // a_cellPos (vec2)
    const cellPosLoc = gl.getAttribLocation(this.program, "a_cellPos");
    gl.enableVertexAttribArray(cellPosLoc);
    gl.vertexAttribPointer(cellPosLoc, 2, gl.FLOAT, false, instStride, 0);
    gl.vertexAttribDivisor(cellPosLoc, 1);
    // a_matID (float)
    const matLoc = gl.getAttribLocation(this.program, "a_matID");
    gl.enableVertexAttribArray(matLoc);
    gl.vertexAttribPointer(matLoc, 1, gl.FLOAT, false, instStride, 8);
    gl.vertexAttribDivisor(matLoc, 1);
    // a_height (float)
    const htLoc = gl.getAttribLocation(this.program, "a_height");
    gl.enableVertexAttribArray(htLoc);
    gl.vertexAttribPointer(htLoc, 1, gl.FLOAT, false, instStride, 12);
    gl.vertexAttribDivisor(htLoc, 1);

    gl.bindVertexArray(null);

    // Cache uniforms
    const uniformNames = ["u_viewOffset", "u_scale", "u_resolution", "u_time", "u_dayNight"];
    for (const u of uniformNames) {
      this.locs[u] = gl.getUniformLocation(this.program, u);
    }

    // Enable blending for semi-transparent materials
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

    // Enable depth test for proper tile ordering
    gl.enable(gl.DEPTH_TEST);
    gl.depthFunc(gl.LEQUAL);

    this.startRenderLoop();
  }

  initWorld(w: number, h: number): void {
    this.worldW = w;
    this.worldH = h;
    this.materialCache = new Uint8Array(w * h);
    this.dirty = true;
  }

  stepZoom(direction: 1 | -1): number {
    const levels = IsometricRenderer.ZOOM_LEVELS;
    const curIdx = levels.indexOf(this.zoom);
    const idx = curIdx === -1
      ? levels.reduce((best, v, i) => Math.abs(v - this.zoom) < Math.abs(levels[best] - this.zoom) ? i : best, 0)
      : curIdx;
    const newIdx = Math.max(0, Math.min(levels.length - 1, idx + direction));
    this.zoom = levels[newIdx];
    return this.zoom;
  }

  applySnapshot(snap: FullSnapshot): void {
    if (!this.materialCache) return;
    for (let row = 0; row < snap.h; row++) {
      for (let col = 0; col < snap.w; col++) {
        const wx = snap.x + col;
        const wy = snap.y + row;
        if (wx >= 0 && wx < this.worldW && wy >= 0 && wy < this.worldH) {
          this.materialCache[wy * this.worldW + wx] = snap.data[row * snap.w + col];
        }
      }
    }
    this.dirty = true;
  }

  applyChunkUpdates(updates: ChunkUpdate[]): void {
    if (!this.materialCache) return;
    const cs = this.chunkSize;
    for (const u of updates) {
      const cx0 = u.cx * cs;
      const cy0 = u.cy * cs;
      const cw = Math.min(cs, this.worldW - cx0);
      const ch = Math.min(cs, this.worldH - cy0);
      if (cw <= 0 || ch <= 0) continue;
      let idx = 0;
      for (let y = cy0; y < cy0 + ch; y++) {
        for (let x = cx0; x < cx0 + cw; x++) {
          this.materialCache[y * this.worldW + x] = u.data[idx++];
        }
      }
    }
    this.dirty = true;
  }

  onResize(): void {
    this.gl.viewport(0, 0, this.canvas.width, this.canvas.height);
    this.dirty = true;
  }

  getMaterialCache(): Uint8Array | null {
    return this.materialCache;
  }

  drawImmediate(): void {
    this.dirty = true;
  }

  dispose(): void {
    cancelAnimationFrame(this.animFrame);
    const gl = this.gl;
    gl.deleteProgram(this.program);
    gl.deleteBuffer(this.instanceBuffer);
  }

  // ── Instance data rebuild ────────────────────────────────────────────────

  private rebuildInstances(): void {
    if (!this.materialCache || this.worldW === 0) return;

    // Determine visible region in grid coords (approximate for iso)
    const tileW = 16 * this.zoom;
    const tileH = 8 * this.zoom;
    const cw = this.canvas.width;
    const ch = this.canvas.height;

    // Visible tile radius from center
    const tilesX = Math.ceil(cw / tileW) + 4;
    const tilesY = Math.ceil(ch / tileH) + 8;

    // Center cell based on viewX/viewY (these are in iso-space offset)
    // Convert view offset to approximate grid center
    const centerX = Math.floor(this.viewX / 16 + this.viewY / 8);
    const centerY = Math.floor(this.viewY / 8 - this.viewX / 16);

    const halfX = Math.ceil(tilesX / 2) + 2;
    const halfY = Math.ceil(tilesY / 2) + 2;

    const x0 = Math.max(0, centerX - halfX);
    const x1 = Math.min(this.worldW, centerX + halfX);
    const y0 = Math.max(0, centerY - halfY);
    const y1 = Math.min(this.worldH, centerY + halfY);

    // Count non-empty cells in visible range
    let count = 0;
    for (let y = y0; y < y1; y++) {
      for (let x = x0; x < x1; x++) {
        const mat = this.materialCache[y * this.worldW + x];
        if (mat !== 0) count++;
      }
    }

    // Allocate instance data: 4 floats per instance (cellX, cellY, matID, height)
    if (!this.instanceData || this.instanceData.length < count * 4) {
      this.instanceData = new Float32Array(count * 4);
    }

    let i = 0;
    // Render back-to-front for depth ordering (higher y first in iso)
    for (let y = y0; y < y1; y++) {
      for (let x = x0; x < x1; x++) {
        const mat = this.materialCache[y * this.worldW + x];
        if (mat === 0) continue;
        const height = mat < MAT_HEIGHT.length ? MAT_HEIGHT[mat] : 1;
        this.instanceData[i]     = x;
        this.instanceData[i + 1] = y;
        this.instanceData[i + 2] = mat;
        this.instanceData[i + 3] = height;
        i += 4;
      }
    }

    this.instanceCount = count;

    // Upload to GPU
    const gl = this.gl;
    gl.bindBuffer(gl.ARRAY_BUFFER, this.instanceBuffer);
    gl.bufferData(gl.ARRAY_BUFFER, this.instanceData.subarray(0, count * 4), gl.DYNAMIC_DRAW);

    this.dirty = false;
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

    // Rebuild instances if data changed
    if (this.dirty) {
      this.rebuildInstances();
    }

    if (this.instanceCount === 0) return;

    gl.viewport(0, 0, cw, ch);
    gl.clearColor(0.05, 0.06, 0.08, 1.0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.useProgram(this.program);

    // Day/night cycle: 60 second period
    const dayPhase = (Math.sin(time * 0.001 * Math.PI / 30.0) + 1.0) * 0.5;

    gl.uniform2f(this.locs["u_viewOffset"]!, this.viewX, this.viewY);
    gl.uniform1f(this.locs["u_scale"]!, this.zoom);
    gl.uniform2f(this.locs["u_resolution"]!, cw * 0.5, ch * 0.5);
    gl.uniform1f(this.locs["u_time"]!, time * 0.001);
    gl.uniform1f(this.locs["u_dayNight"]!, dayPhase);

    gl.bindVertexArray(this.vao);
    gl.drawElementsInstanced(gl.TRIANGLES, this.indexCount, gl.UNSIGNED_SHORT, 0, this.instanceCount);
    gl.bindVertexArray(null);
  }
}
