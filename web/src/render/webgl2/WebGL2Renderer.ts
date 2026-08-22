/**
 * WebGL2Renderer.ts — Primary accelerated renderer
 *
 * # Strategy
 *
 * Material data is uploaded to a GPU texture (R8UI format: one uint8 per cell).
 * A fullscreen quad draws the world using a fragment shader that performs a
 * palette lookup per pixel — no per-cell draw calls.
 *
 * Dirty chunks update texture sub-regions via texSubImage2D(), so only
 * changed data is uploaded each frame.
 *
 * # Shader pipeline
 *
 *   material texture (R8UI, worldW×worldH)
 *          ↓
 *   fragment shader
 *          ↓
 *   palette lookup (uniform sampler2D, 256×1 RGBA)
 *          ↓
 *   procedural colour variation (coordinate hash)
 *          ↓
 *   animated water / fire effects (time uniform)
 *          ↓
 *   screen output
 *
 * # Layer system
 *
 * Each optional visual layer is toggled by a bool uniform.
 * Disabling layers reduces fill rate and improves mobile performance.
 */

import type {
  IWorldRenderer,
  RendererConfig,
  WorldSnapshot,
  ChunkUpdate,
  Camera,
} from "../IWorldRenderer.js";

const VERT_SRC = /* glsl */ `#version 300 es
in  vec2 a_pos;      // -1..+1 clip space quad
out vec2 v_uv;
void main() {
  gl_Position = vec4(a_pos, 0.0, 1.0);
  v_uv = a_pos * 0.5 + 0.5;
}`;

const FRAG_SRC = /* glsl */ `#version 300 es
precision highp float;
precision highp usampler2D;

in  vec2 v_uv;
out vec4 fragColor;

uniform usampler2D u_material;   // R8UI texture, worldW × worldH
uniform sampler2D  u_palette;    // 256 × 1 RGBA lookup
uniform vec2       u_worldSize;  // vec2(worldW, worldH)
uniform vec2       u_camera;     // world-coordinate top-left
uniform vec2       u_viewport;  // canvas size in pixels
uniform float      u_time;
uniform bool       u_water_anim;
uniform bool       u_fire_anim;

// Quick hash for procedural variation (no external library)
float hash(vec2 p) {
  return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

void main() {
  // Map fragment to world coordinate
  vec2 worldPos = u_camera + v_uv * u_viewport;
  vec2 cellUV   = worldPos / u_worldSize;

  // Sample material ID
  uint matID = texture(u_material, cellUV).r;

  // Base colour from palette
  vec4 col = texture(u_palette, vec2((float(matID) + 0.5) / 256.0, 0.5));

  // Procedural variation per cell
  vec2 cellCoord = floor(worldPos);
  float noise = hash(cellCoord) * 0.08 - 0.04;

  // Animated water
  if (u_water_anim && matID == 4u) {
    float wave = sin(worldPos.x * 0.3 + u_time * 2.0) * 0.05 +
                 sin(worldPos.y * 0.5 + u_time * 1.3) * 0.03;
    col.r += wave * 0.2;
    col.g += wave * 0.4;
    col.b += wave * 0.6;
  }

  // Animated fire flicker
  if (u_fire_anim && matID == 6u) {
    float flicker = hash(cellCoord + floor(u_time * 12.0));
    col.rgb = mix(col.rgb, vec3(1.0, 0.3, 0.0), flicker * 0.5);
  }

  // Apply variation
  col.rgb += noise;
  col.rgb = clamp(col.rgb, 0.0, 1.0);

  fragColor = col;
}`;

/** RGBA palette: 256 entries × 4 bytes, indexed by material ID */
function buildPaletteData(): Uint8Array {
  const data = new Uint8Array(256 * 4);
  const defs: [number, number, number, number, number][] = [
    /*  0 Empty */  [ 0,  13,  13,  16, 255],
    /*  1 Rock  */  [ 1,  80,  75,  70, 255],
    /*  2 Soil  */  [ 2, 101,  67,  33, 255],
    /*  3 Sand  */  [ 3, 210, 180, 100, 255],
    /*  4 Water */  [ 4,  30, 100, 200, 220],
    /*  5 Plant */  [ 5,  50, 140,  50, 255],
    /*  6 Fire  */  [ 6, 240, 100,  20, 255],
    /*  7 Vapor */  [ 7, 180, 210, 240, 120],
    /*  8 Smoke */  [ 8,  80,  80,  80, 160],
    /*  9 Lava  */  [ 9, 220,  60,   0, 255],
    /* 10 Ice   */  [10, 180, 220, 255, 230],
    /* 11 Ash   */  [11, 140, 130, 120, 255],
    /* 12 Oil   */  [12,  40,  35,  20, 255],
    /* 13 Ember */  [13, 255, 160,  40, 220],
  ];
  for (const [id, r, g, b, a] of defs) {
    data[id * 4]     = r;
    data[id * 4 + 1] = g;
    data[id * 4 + 2] = b;
    data[id * 4 + 3] = a;
  }
  return data;
}

export class WebGL2Renderer implements IWorldRenderer {
  readonly name = "WebGL2";

  private gl!: WebGL2RenderingContext;
  private program!: WebGLProgram;
  private matTex!: WebGLTexture;
  private paletteTex!: WebGLTexture;
  private vao!: WebGLVertexArrayObject;

  private worldW   = 0;
  private worldH   = 0;
  private chunkSize = 64;
  private camera: Camera = { x: 0, y: 0, width: 0, height: 0, zoom: 1 };

  // Pending chunk uploads (batched per frame)
  private pendingChunks: ChunkUpdate[] = [];
  private pendingSnapshot: WorldSnapshot | null = null;

  async initialize(config: RendererConfig): Promise<void> {
    const gl = config.canvas.getContext("webgl2");
    if (!gl) throw new Error("WebGL2 not available");
    this.gl        = gl;
    this.worldW    = config.worldW;
    this.worldH    = config.worldH;
    this.chunkSize = config.chunkSize;

    this.program = createProgram(gl, VERT_SRC, FRAG_SRC);
    this.setupQuad();
    this.matTex     = this.createMaterialTexture();
    this.paletteTex = this.createPaletteTexture();
    this.camera     = { x: 0, y: 0, width: config.canvas.width, height: config.canvas.height, zoom: 1 };
  }

  applySnapshot(snapshot: WorldSnapshot): void {
    this.pendingSnapshot = snapshot;
  }

  applyChunk(update: ChunkUpdate): void {
    this.pendingChunks.push(update);
  }

  setCamera(camera: Camera): void {
    this.camera = camera;
  }

  resize(width: number, height: number): void {
    this.gl.viewport(0, 0, width, height);
    this.camera.width  = width;
    this.camera.height = height;
  }

  render(time: number): void {
    const gl = this.gl;

    // Upload pending texture data
    if (this.pendingSnapshot) {
      this.uploadSnapshot(this.pendingSnapshot);
      this.pendingSnapshot = null;
    }
    for (const c of this.pendingChunks) {
      this.uploadChunk(c);
    }
    this.pendingChunks = [];

    gl.clearColor(0.05, 0.05, 0.063, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(this.program);

    // Bind textures
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    gl.uniform1i(gl.getUniformLocation(this.program, "u_material"), 0);

    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, this.paletteTex);
    gl.uniform1i(gl.getUniformLocation(this.program, "u_palette"), 1);

    // Uniforms
    gl.uniform2f(gl.getUniformLocation(this.program, "u_worldSize"), this.worldW, this.worldH);
    gl.uniform2f(gl.getUniformLocation(this.program, "u_camera"),    this.camera.x, this.camera.y);
    gl.uniform2f(gl.getUniformLocation(this.program, "u_viewport"),  this.camera.width, this.camera.height);
    gl.uniform1f(gl.getUniformLocation(this.program, "u_time"),      time * 0.001);
    gl.uniform1i(gl.getUniformLocation(this.program, "u_water_anim"), 1);
    gl.uniform1i(gl.getUniformLocation(this.program, "u_fire_anim"),  1);

    gl.bindVertexArray(this.vao);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    gl.bindVertexArray(null);
  }

  dispose(): void {
    const gl = this.gl;
    gl.deleteProgram(this.program);
    gl.deleteTexture(this.matTex);
    gl.deleteTexture(this.paletteTex);
  }

  private createMaterialTexture(): WebGLTexture {
    const gl  = this.gl;
    const tex = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(
      gl.TEXTURE_2D, 0, gl.R8UI,
      this.worldW, this.worldH, 0,
      gl.RED_INTEGER, gl.UNSIGNED_BYTE,
      new Uint8Array(this.worldW * this.worldH),
    );
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    return tex;
  }

  private createPaletteTexture(): WebGLTexture {
    const gl   = this.gl;
    const tex  = gl.createTexture()!;
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
    return tex;
  }

  private setupQuad(): void {
    const gl  = this.gl;
    const vao = gl.createVertexArray()!;
    this.vao  = vao;
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
  }

  private uploadSnapshot(snap: WorldSnapshot): void {
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    gl.texSubImage2D(
      gl.TEXTURE_2D, 0,
      snap.x, snap.y,
      snap.width, snap.height,
      gl.RED_INTEGER, gl.UNSIGNED_BYTE,
      snap.data,
    );
  }

  private uploadChunk(update: ChunkUpdate): void {
    const gl  = this.gl;
    const cs  = this.chunkSize;
    const cx0 = update.cx * cs;
    const cy0 = update.cy * cs;
    const cw  = Math.min(cs, this.worldW - cx0);
    const ch  = Math.min(cs, this.worldH - cy0);
    if (cw <= 0 || ch <= 0) return;
    gl.bindTexture(gl.TEXTURE_2D, this.matTex);
    gl.texSubImage2D(
      gl.TEXTURE_2D, 0,
      cx0, cy0, cw, ch,
      gl.RED_INTEGER, gl.UNSIGNED_BYTE,
      update.data,
    );
  }
}

// ── WebGL2 helpers ─────────────────────────────────────────────────────────

function createProgram(gl: WebGL2RenderingContext, vert: string, frag: string): WebGLProgram {
  const p  = gl.createProgram()!;
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

function compileShader(gl: WebGL2RenderingContext, type: number, src: string): WebGLShader {
  const s = gl.createShader(type)!;
  gl.shaderSource(s, src);
  gl.compileShader(s);
  if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
    throw new Error("Shader compile error: " + gl.getShaderInfoLog(s));
  }
  return s;
}
