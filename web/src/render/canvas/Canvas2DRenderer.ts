/**
 * Canvas2DRenderer.ts — Canvas 2D reference renderer
 *
 * Role: fallback / debug / reference implementation.
 *
 * This renderer is intentionally simple.  It uses ImageData bulk-pixel
 * writes for acceptable performance but does not use GPU shaders.
 *
 * Use it to:
 *  - verify visual correctness without GPU shader complexity
 *  - support very old or constrained devices
 *  - profile the delta between CPU and GPU render paths
 *
 * Do NOT add shader-level effects here.  Complex visuals belong in
 * WebGL2Renderer or WebGPURenderer.
 */

import type {
  IWorldRenderer,
  RendererConfig,
  WorldSnapshot,
  ChunkUpdate,
  Camera,
} from "../IWorldRenderer.js";

/** RGBA palette for each material ID — index = matID * 4 */
const PALETTE = new Uint8ClampedArray([
  /* 0  Empty */  13,  13,  16, 255,
  /* 1  Rock  */  80,  75,  70, 255,
  /* 2  Soil  */ 101,  67,  33, 255,
  /* 3  Sand  */ 210, 180, 100, 255,
  /* 4  Water */  30, 100, 200, 220,
  /* 5  Plant */  50, 140,  50, 255,
  /* 6  Fire  */ 240, 100,  20, 255,
  /* 7  Vapor */ 180, 210, 240, 120,
  /* 8  Smoke */  80,  80,  80, 160,
  /* 9  Lava  */ 220,  60,   0, 255,
  /* 10 Ice   */ 180, 220, 255, 230,
  /* 11 Ash   */ 140, 130, 120, 255,
  /* 12 Oil   */  40,  35,  20, 255,
  /* 13 Ember */ 255, 160,  40, 220,
]);

export class Canvas2DRenderer implements IWorldRenderer {
  readonly name = "Canvas2D";

  private ctx!: CanvasRenderingContext2D;
  private imageData!: ImageData;
  private materialCache!: Uint8Array;
  private camera: Camera = { x: 0, y: 0, width: 0, height: 0, zoom: 1 };
  private worldW = 0;
  private worldH = 0;
  private chunkSize = 64;
  private dirty = false;

  async initialize(config: RendererConfig): Promise<void> {
    const ctx = config.canvas.getContext("2d");
    if (!ctx) throw new Error("Canvas2D not supported");
    this.ctx        = ctx;
    this.worldW     = config.worldW;
    this.worldH     = config.worldH;
    this.chunkSize  = config.chunkSize;
    this.materialCache = new Uint8Array(config.worldW * config.worldH);
    this.imageData  = ctx.createImageData(config.canvas.width, config.canvas.height);
    this.camera     = { x: 0, y: 0, width: config.canvas.width, height: config.canvas.height, zoom: 1 };
  }

  applySnapshot(snap: WorldSnapshot): void {
    for (let row = 0; row < snap.height; row++) {
      for (let col = 0; col < snap.width; col++) {
        const wx = snap.x + col;
        const wy = snap.y + row;
        if (wx < this.worldW && wy < this.worldH) {
          this.materialCache[wy * this.worldW + wx] = snap.data[row * snap.width + col];
        }
      }
    }
    this.dirty = true;
  }

  applyChunk(update: ChunkUpdate): void {
    const cs   = this.chunkSize;
    const cx0  = update.cx * cs;
    const cy0  = update.cy * cs;
    const cxEnd = Math.min(cx0 + cs, this.worldW);
    const cyEnd = Math.min(cy0 + cs, this.worldH);
    let idx = 0;
    for (let y = cy0; y < cyEnd; y++) {
      for (let x = cx0; x < cxEnd; x++) {
        this.materialCache[y * this.worldW + x] = update.data[idx++];
      }
    }
    this.dirty = true;
  }

  setCamera(camera: Camera): void {
    this.camera = camera;
    this.dirty  = true;
  }

  resize(width: number, height: number): void {
    this.imageData = this.ctx.createImageData(width, height);
    this.dirty     = true;
  }

  render(_time: number): void {
    if (!this.dirty) return;
    this.dirty = false;

    const { width: cw, height: ch } = this.imageData;
    const buf  = this.imageData.data;
    const wx0  = Math.floor(this.camera.x);
    const wy0  = Math.floor(this.camera.y);

    for (let py = 0; py < ch; py++) {
      for (let px = 0; px < cw; px++) {
        const wx = wx0 + px;
        const wy = wy0 + py;
        let mat  = 0;
        if (wx >= 0 && wx < this.worldW && wy >= 0 && wy < this.worldH) {
          mat = this.materialCache[wy * this.worldW + wx];
        }
        const p = (py * cw + px) * 4;
        const m = Math.min(mat, 13) * 4;
        buf[p]     = PALETTE[m];
        buf[p + 1] = PALETTE[m + 1];
        buf[p + 2] = PALETTE[m + 2];
        buf[p + 3] = PALETTE[m + 3];
      }
    }

    this.ctx.putImageData(this.imageData, 0, 0);
  }

  dispose(): void {
    /* Canvas2D has no GPU resources to release */
  }
}
