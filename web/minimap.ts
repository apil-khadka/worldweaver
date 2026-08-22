/**
 * minimap.ts — Low-resolution overview of the full world
 *
 * Renders the entire world at 1px per SCALE cells on a small overlay canvas.
 * Updates at 500ms intervals (not every frame) for performance.
 * Shows a white rectangle indicating the current viewport.
 */

import { IGameRenderer } from "./network.js";

/** How many world cells map to one minimap pixel */
const SCALE = 8;

/** RGBA palette matching renderer.ts — duplicated to avoid coupling */
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
  /* Lava  */ 255, 80,  0,   255,
  /* Ice   */ 180, 230, 255, 255,
  /* Ash   */ 50,  45,  40,  200,
  /* Oil   */ 80,  60,  20,  255,
  /* Ember */ 200, 50,  10,  180,
]);

export class Minimap {
  private readonly ctx: CanvasRenderingContext2D;
  private readonly canvas: HTMLCanvasElement;
  private readonly renderer: IGameRenderer;
  private readonly viewCanvas: HTMLCanvasElement;
  private intervalId: number | null = null;

  constructor(canvas: HTMLCanvasElement, renderer: IGameRenderer, viewCanvas: HTMLCanvasElement) {
    this.canvas = canvas;
    this.renderer = renderer;
    this.viewCanvas = viewCanvas;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Minimap canvas 2D not supported");
    this.ctx = ctx;
  }

  /** Start the 500ms update loop. */
  start(): void {
    this.draw();
    this.intervalId = window.setInterval(() => this.draw(), 500);
  }

  /** Stop rendering updates. */
  stop(): void {
    if (this.intervalId !== null) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  private draw(): void {
    const cache = this.renderer.getMaterialCache();
    if (!cache) return;

    const worldW = this.renderer.worldW;
    const worldH = this.renderer.worldH;
    if (worldW === 0 || worldH === 0) return;

    const mapW = Math.ceil(worldW / SCALE);
    const mapH = Math.ceil(worldH / SCALE);

    // Resize canvas to match world aspect ratio if needed
    if (this.canvas.width !== mapW || this.canvas.height !== mapH) {
      this.canvas.width = mapW;
      this.canvas.height = mapH;
    }

    const imgData = this.ctx.createImageData(mapW, mapH);
    const buf = imgData.data;

    // Sample the world at reduced resolution (pick center cell of each block)
    for (let my = 0; my < mapH; my++) {
      const wy = my * SCALE + (SCALE >> 1);
      if (wy >= worldH) continue;
      for (let mx = 0; mx < mapW; mx++) {
        const wx = mx * SCALE + (SCALE >> 1);
        if (wx >= worldW) continue;
        const mat = cache[wy * worldW + wx];
        const p = (my * mapW + mx) * 4;
        const m = mat * 4;
        buf[p]     = PALETTE[m];
        buf[p + 1] = PALETTE[m + 1];
        buf[p + 2] = PALETTE[m + 2];
        buf[p + 3] = PALETTE[m + 3];
      }
    }

    this.ctx.putImageData(imgData, 0, 0);

    // Draw viewport rectangle
    const viewX = this.renderer.viewX;
    const viewY = this.renderer.viewY;
    const viewW = this.viewCanvas.width;
    const viewH = this.viewCanvas.height;

    const rx = viewX / SCALE;
    const ry = viewY / SCALE;
    const rw = viewW / SCALE;
    const rh = viewH / SCALE;

    this.ctx.strokeStyle = "rgba(255, 255, 255, 0.9)";
    this.ctx.lineWidth = 1;
    this.ctx.strokeRect(rx, ry, rw, rh);
  }
}
