/**
 * renderer.ts — Canvas 2D world renderer
 *
 * The renderer maintains a local pixel cache of the visible world region.
 * It is updated by the network layer when dirty-chunk data arrives.
 * The renderer performs NO simulation logic.
 *
 * Design notes:
 *  - Uses ImageData for bulk pixel writes (faster than fillRect per cell)
 *  - Cell size is 1px at 1:1 zoom; zoom is handled by canvas CSS scaling
 *  - Material colours are looked up from a static palette
 *
 * References:
 *  - MDN ImageData: https://developer.mozilla.org/en-US/docs/Web/API/ImageData
 */

/** Material IDs must match internal/world/materials.go */
const enum Mat {
  Empty = 0,
  Rock  = 1,
  Soil  = 2,
  Sand  = 3,
  Water = 4,
  Plant = 5,
  Fire  = 6,
  Vapor = 7,
  Smoke = 8,
}

/** RGBA palette for each material ID. Index = mat * 4, layout = [r, g, b, a] */
const PALETTE = new Uint8Array([
  /* Empty */ 13,  13,  16,  255,
  /* Rock  */ 80,  75,  70,  255,
  /* Soil  */ 101, 67,  33,  255,
  /* Sand  */ 210, 180, 100, 255,
  /* Water */ 30,  100, 200, 220,
  /* Plant */ 50,  140, 50,  255,
  /* Fire  */ 240, 100, 20,  255,
  /* Vapor */ 180, 210, 240, 120,
  /* Smoke */ 80,  80,  80,  160,
]);

export interface ChunkUpdate {
  cx: number;
  cy: number;
  tick: number;
  data: Uint8Array; // flat material array for this chunk
}

export interface FullSnapshot {
  tick: number;
  x: number;
  y: number;
  w: number;
  h: number;
  data: Uint8Array;
}

export class WorldRenderer {
  private readonly ctx: CanvasRenderingContext2D;
  private imageData: ImageData;

  /** Current viewport origin (top-left cell coordinates) */
  viewX = 0;
  viewY = 0;

  /** World dimensions, set from WELCOME message */
  worldW = 0;
  worldH = 0;

  /** Chunk size in cells — must match the server ChunkSize */
  chunkSize = 64;

  /** Local material cache. Indexed as [y * worldW + x]. */
  private materialCache: Uint8Array | null = null;

  constructor(private readonly canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Canvas 2D not supported");
    this.ctx = ctx;
    this.imageData = ctx.createImageData(canvas.width, canvas.height);
  }

  /** Called when the server sends world dimensions. */
  initWorld(w: number, h: number): void {
    this.worldW = w;
    this.worldH = h;
    this.materialCache = new Uint8Array(w * h);
  }

  /** Apply a full viewport snapshot from the server. */
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
    this.draw();
  }

  /** Apply a list of dirty chunk updates from the server. */
  applyChunkUpdates(updates: ChunkUpdate[]): void {
    if (!this.materialCache) return;
    const cs = this.chunkSize;
    for (const u of updates) {
      const cx0 = u.cx * cs;
      const cy0 = u.cy * cs;
      const cxEnd = Math.min(cx0 + cs, this.worldW);
      const cyEnd = Math.min(cy0 + cs, this.worldH);
      let idx = 0;
      for (let y = cy0; y < cyEnd; y++) {
        for (let x = cx0; x < cxEnd; x++) {
          this.materialCache[y * this.worldW + x] = u.data[idx++];
        }
      }
    }
    this.draw();
  }

  /** Resize internal ImageData after canvas resize. */
  onResize(): void {
    this.imageData = this.ctx.createImageData(this.canvas.width, this.canvas.height);
    this.draw();
  }

  private draw(): void {
    if (!this.materialCache) return;

    const { width: cw, height: ch } = this.canvas;
    const buf = this.imageData.data;
    const wx0 = this.viewX;
    const wy0 = this.viewY;

    for (let py = 0; py < ch; py++) {
      const wy = wy0 + py;
      for (let px = 0; px < cw; px++) {
        const wx = wx0 + px;
        let mat = 0;
        if (wx >= 0 && wx < this.worldW && wy >= 0 && wy < this.worldH) {
          mat = this.materialCache[wy * this.worldW + wx];
        }
        const p  = (py * cw + px) * 4;
        const m  = mat * 4;
        buf[p]     = PALETTE[m];
        buf[p + 1] = PALETTE[m + 1];
        buf[p + 2] = PALETTE[m + 2];
        buf[p + 3] = PALETTE[m + 3];
      }
    }

    this.ctx.putImageData(this.imageData, 0, 0);
  }
}
