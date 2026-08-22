/**
 * WebGPURenderer.ts — Optional modern GPU renderer
 *
 * # Status: Stub / Work-in-progress
 *
 * WebGPU is loaded only when navigator.gpu is available and adapter
 * initialization succeeds (see IWorldRenderer.ts::selectRenderer).
 *
 * # Architecture
 *
 * The strategy mirrors WebGL2Renderer:
 *   - material data → GPUTexture (r8uint)
 *   - palette       → uniform buffer or small GPUTexture
 *   - fullscreen quad rendered by WGSL fragment shader
 *
 * Dirty chunks update texture sub-regions via writeTexture() rather than
 * uploading the entire world each frame.
 *
 * # Potential advantages over WebGL2
 *
 *   - Explicit resource management (no hidden state machine)
 *   - Compute shaders for future client-side visual effects
 *   - Better GPU timing APIs for diagnostics
 *   - Storage buffers for larger world data if needed
 *
 * # IMPORTANT constraint
 *
 * WebGPU is a visualization backend only.
 * The world simulation remains server-authoritative.
 * No physics or gameplay logic runs in WGSL. (ADR-005)
 *
 * References:
 *   - WebGPU spec: https://www.w3.org/TR/webgpu/
 *   - WGSL spec:   https://www.w3.org/TR/WGSL/
 */

import type {
  IWorldRenderer,
  RendererConfig,
  WorldSnapshot,
  ChunkUpdate,
  Camera,
} from "../IWorldRenderer.js";

export class WebGPURenderer implements IWorldRenderer {
  readonly name = "WebGPU";

  private device!: GPUDevice;
  private context!: GPUCanvasContext;
  private pipeline!: GPURenderPipeline;
  private matTexture!: GPUTexture;
  private paletteBuffer!: GPUBuffer;
  private uniformBuffer!: GPUBuffer;
  private bindGroup!: GPUBindGroup;
  private worldW   = 0;
  private worldH   = 0;
  private chunkSize = 64;
  private camera: Camera = { x: 0, y: 0, width: 0, height: 0, zoom: 1 };
  private pendingChunks: ChunkUpdate[] = [];
  private pendingSnapshot: WorldSnapshot | null = null;

  async initialize(config: RendererConfig): Promise<void> {
    const gpu = (navigator as any).gpu as GPU;
    const adapter = await gpu.requestAdapter({ powerPreference: "high-performance" });
    if (!adapter) throw new Error("WebGPU adapter not available");
    this.device = await adapter.requestDevice();

    const canvas  = config.canvas;
    this.context  = canvas.getContext("webgpu") as GPUCanvasContext;
    this.worldW   = config.worldW;
    this.worldH   = config.worldH;
    this.chunkSize = config.chunkSize;
    this.camera   = { x: 0, y: 0, width: canvas.width, height: canvas.height, zoom: 1 };

    const format = gpu.getPreferredCanvasFormat();
    this.context.configure({ device: this.device, format, alphaMode: "premultiplied" });

    this.matTexture   = this.createMaterialTexture();
    this.paletteBuffer = this.createPaletteBuffer();
    this.uniformBuffer = this.createUniformBuffer();
    this.pipeline      = await this.createPipeline(format);
    this.bindGroup     = this.createBindGroup();

    console.info("[WebGPURenderer] initialized — world", config.worldW, "×", config.worldH);
  }

  applySnapshot(snapshot: WorldSnapshot): void {
    this.pendingSnapshot = snapshot;
  }

  applyChunk(update: ChunkUpdate): void {
    this.pendingChunks.push(update);
  }

  setCamera(camera: Camera): void {
    this.camera = camera;
    this.writeUniforms();
  }

  resize(width: number, height: number): void {
    this.camera.width  = width;
    this.camera.height = height;
    this.writeUniforms();
  }

  render(_time: number): void {
    const { device } = this;

    // Upload pending texture data
    if (this.pendingSnapshot) {
      this.uploadSnapshot(this.pendingSnapshot);
      this.pendingSnapshot = null;
    }
    for (const c of this.pendingChunks) {
      this.uploadChunk(c);
    }
    this.pendingChunks = [];

    this.writeUniforms();

    const encoder   = device.createCommandEncoder();
    const pass      = encoder.beginRenderPass({
      colorAttachments: [{
        view:       this.context.getCurrentTexture().createView(),
        loadOp:     "clear",
        storeOp:    "store",
        clearValue: { r: 0.05, g: 0.05, b: 0.063, a: 1 },
      }],
    });

    pass.setPipeline(this.pipeline);
    pass.setBindGroup(0, this.bindGroup);
    pass.draw(4);
    pass.end();
    device.queue.submit([encoder.finish()]);
  }

  dispose(): void {
    this.matTexture.destroy();
    this.paletteBuffer.destroy();
    this.uniformBuffer.destroy();
    this.device.destroy();
  }

  // ── Private helpers ────────────────────────────────────────────────────

  private createMaterialTexture(): GPUTexture {
    return this.device.createTexture({
      size:   [this.worldW, this.worldH, 1],
      format: "r8uint",
      usage:  GPUTextureUsage.TEXTURE_BINDING | GPUTextureUsage.COPY_DST,
    });
  }

  private createPaletteBuffer(): GPUBuffer {
    // 256 × RGBA8 entries = 1024 bytes
    const data = buildPaletteData();
    const buf  = this.device.createBuffer({
      size:             data.byteLength,
      usage:            GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST,
      mappedAtCreation: true,
    });
    new Uint8Array(buf.getMappedRange()).set(data);
    buf.unmap();
    return buf;
  }

  private createUniformBuffer(): GPUBuffer {
    // worldSize(8) + camera(8) + viewport(8) + time(4) + pad(4) = 32 bytes
    return this.device.createBuffer({
      size:  32,
      usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST,
    });
  }

  private writeUniforms(): void {
    const data = new Float32Array(8);
    data[0] = this.worldW;
    data[1] = this.worldH;
    data[2] = this.camera.x;
    data[3] = this.camera.y;
    data[4] = this.camera.width;
    data[5] = this.camera.height;
    data[6] = performance.now() * 0.001;
    this.device.queue.writeBuffer(this.uniformBuffer, 0, data);
  }

  private async createPipeline(format: GPUTextureFormat): Promise<GPURenderPipeline> {
    const wgsl = /* wgsl */ `
struct Uniforms {
  worldSize : vec2<f32>,
  camera    : vec2<f32>,
  viewport  : vec2<f32>,
  time      : f32,
  _pad      : f32,
}
@group(0) @binding(0) var<uniform>          u  : Uniforms;
@group(0) @binding(1) var                   mat_tex : texture_2d<u32>;
@group(0) @binding(2) var<uniform>          palette : array<vec4<u32>, 256>;

struct VSOut { @builtin(position) pos: vec4<f32>, @location(0) uv: vec2<f32> }

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> VSOut {
  var pos = array<vec2<f32>,4>(
    vec2(-1.0,-1.0), vec2(1.0,-1.0), vec2(-1.0,1.0), vec2(1.0,1.0)
  );
  var out: VSOut;
  out.pos = vec4(pos[idx], 0.0, 1.0);
  out.uv  = pos[idx] * 0.5 + 0.5;
  return out;
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
  let world_pos = u.camera + in.uv * u.viewport;
  let cell_uv   = vec2<i32>(world_pos);
  let mat_id    = textureLoad(mat_tex, cell_uv, 0).r;
  let entry     = palette[mat_id];
  return vec4<f32>(
    f32(entry.r) / 255.0,
    f32(entry.g) / 255.0,
    f32(entry.b) / 255.0,
    f32(entry.a) / 255.0,
  );
}`;

    const mod = this.device.createShaderModule({ code: wgsl });
    return this.device.createRenderPipeline({
      layout:  "auto",
      vertex:  { module: mod, entryPoint: "vs_main" },
      fragment: {
        module: mod,
        entryPoint: "fs_main",
        targets: [{ format }],
      },
      primitive: { topology: "triangle-strip" },
    });
  }

  private createBindGroup(): GPUBindGroup {
    return this.device.createBindGroup({
      layout: this.pipeline.getBindGroupLayout(0),
      entries: [
        { binding: 0, resource: { buffer: this.uniformBuffer } },
        { binding: 1, resource: this.matTexture.createView() },
        { binding: 2, resource: { buffer: this.paletteBuffer } },
      ],
    });
  }

  private uploadSnapshot(snap: WorldSnapshot): void {
    this.device.queue.writeTexture(
      { texture: this.matTexture, origin: { x: snap.x, y: snap.y } },
      snap.data,
      { bytesPerRow: snap.width },
      { width: snap.width, height: snap.height },
    );
  }

  private uploadChunk(update: ChunkUpdate): void {
    const cs  = this.chunkSize;
    const cx0 = update.cx * cs;
    const cy0 = update.cy * cs;
    const cw  = Math.min(cs, this.worldW - cx0);
    const ch  = Math.min(cs, this.worldH - cy0);
    if (cw <= 0 || ch <= 0) return;
    this.device.queue.writeTexture(
      { texture: this.matTexture, origin: { x: cx0, y: cy0 } },
      update.data,
      { bytesPerRow: cw },
      { width: cw, height: ch },
    );
  }
}

function buildPaletteData(): Uint8Array {
  const data = new Uint8Array(256 * 4);
  const defs: [number, number, number, number, number][] = [
    [ 0,  13,  13,  16, 255], [ 1,  80,  75,  70, 255],
    [ 2, 101,  67,  33, 255], [ 3, 210, 180, 100, 255],
    [ 4,  30, 100, 200, 220], [ 5,  50, 140,  50, 255],
    [ 6, 240, 100,  20, 255], [ 7, 180, 210, 240, 120],
    [ 8,  80,  80,  80, 160], [ 9, 220,  60,   0, 255],
    [10, 180, 220, 255, 230], [11, 140, 130, 120, 255],
    [12,  40,  35,  20, 255], [13, 255, 160,  40, 220],
  ];
  for (const [id, r, g, b, a] of defs) {
    data[id * 4] = r; data[id * 4 + 1] = g;
    data[id * 4 + 2] = b; data[id * 4 + 3] = a;
  }
  return data;
}
