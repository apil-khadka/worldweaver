/**
 * brush-overlay.ts — Brush preview ring and creature markers
 *
 * Two problems this solves, both of them "I cannot see what is going on":
 *
 *  1. There was no indication of where a brush would land or how big it was.
 *     The player aimed a 24-cell force with a 1-pixel arrow cursor and found
 *     out the extent only after the terrain changed. A ring drawn at the exact
 *     radius the server will use makes the brush an object you aim rather than
 *     a guess.
 *
 *  2. Creatures are single cells in a world that is normally zoomed out to fit,
 *     so a sheep occupies well under one screen pixel and is invisible however
 *     hard you look. They were there the whole time. Markers scale with zoom and
 *     never shrink below a legible size, so a flock reads as a flock.
 *
 * Everything here draws to a dedicated transparent canvas above the world and
 * below the DOM HUD, and never captures pointer events.
 */

import type { IGameRenderer } from "./network.js";
import type { InputHandler } from "./input.js";

/**
 * Creature material IDs, mirroring internal/world/materials.go.
 *
 * These are NOT contiguous and the ordering is not intuitive: 20 is Carrion and
 * 22 is Grass, so an off-by-one here silently draws grass as a predator. Keep
 * them pinned to the Go constants.
 */
export const MAT_HERBIVORE = 14;
export const MAT_PREDATOR  = 15;
export const MAT_SHEEP     = 21;

interface CreatureStyle {
  /** Marker fill. */
  fill: string;
  /** Marker outline, for contrast against similar terrain. */
  stroke: string;
  label: string;
}

const CREATURE_STYLES: Record<number, CreatureStyle> = {
  [MAT_HERBIVORE]: { fill: "#E8C77A", stroke: "#6B4A17", label: "Grazer" },
  [MAT_SHEEP]:     { fill: "#F6F2E6", stroke: "#5C5346", label: "Sheep" },
  [MAT_PREDATOR]:  { fill: "#C8542F", stroke: "#4A1B0C", label: "Predator" },
};

/**
 * Cell stride used when scanning the visible area for creatures.
 *
 * A full per-cell scan of a 1024x640 viewport every frame is a million reads
 * for a few dozen hits. Creatures move at most one cell per tick, so sampling
 * on a stride and drawing every hit found is visually identical at these
 * marker sizes while costing a fraction of the work.
 */
const SCAN_STRIDE = 1;

/** Below this zoom a creature cell is sub-pixel, so markers are drawn instead. */
const MARKER_ZOOM_THRESHOLD = 6;

/** Marker radius in screen pixels, before zoom scaling. */
const MARKER_MIN_RADIUS = 3.5;
const MARKER_MAX_RADIUS = 9;

export class BrushOverlay {
  private ctx: CanvasRenderingContext2D;
  private animFrame: number | null = null;

  /** Cached counts so the HUD legend does not recount on its own schedule. */
  counts: Record<number, number> = {
    [MAT_HERBIVORE]: 0,
    [MAT_SHEEP]: 0,
    [MAT_PREDATOR]: 0,
  };

  /** Whether creature markers are drawn. Toggled by the HUD button / C key. */
  showCreatures = true;

  /** Whether the brush ring is drawn. */
  showBrush = true;

  constructor(
    private readonly canvas: HTMLCanvasElement,
    private readonly worldCanvas: HTMLCanvasElement,
    private readonly renderer: IGameRenderer,
    private readonly input: InputHandler,
  ) {
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("2D context unavailable for brush overlay");
    this.ctx = ctx;
  }

  start(): void {
    if (this.animFrame !== null) return;
    const tick = () => {
      this.draw();
      this.animFrame = requestAnimationFrame(tick);
    };
    this.animFrame = requestAnimationFrame(tick);
  }

  stop(): void {
    if (this.animFrame !== null) {
      cancelAnimationFrame(this.animFrame);
      this.animFrame = null;
    }
  }

  resize(cssW: number, cssH: number): void {
    this.canvas.width = cssW;
    this.canvas.height = cssH;
  }

  /**
   * World cell → CSS pixel on this overlay.
   *
   * The overlay is sized in CSS pixels while renderer.zoom is in backing-store
   * pixels per cell, so the ratio between the world canvas backing store and its
   * displayed width has to be divided out. Skipping that step is what made
   * earlier overlays drift away from the terrain at non-default render scales.
   */
  private worldToOverlay(wx: number, wy: number): [number, number] {
    const rect = this.worldCanvas.getBoundingClientRect();
    if (rect.width === 0) return [0, 0];
    const backingToCss = rect.width / this.worldCanvas.width;
    const zoom = (this.renderer.zoom || 1) * backingToCss;
    return [
      (wx - this.renderer.viewX) * zoom,
      (wy - this.renderer.viewY) * zoom,
    ];
  }

  /** CSS pixels per world cell on this overlay. */
  private get cssZoom(): number {
    const rect = this.worldCanvas.getBoundingClientRect();
    if (rect.width === 0) return this.renderer.zoom || 1;
    return (this.renderer.zoom || 1) * (rect.width / this.worldCanvas.width);
  }

  private draw(): void {
    const ctx = this.ctx;
    const w = this.canvas.width;
    const h = this.canvas.height;
    if (w === 0 || h === 0) return;

    ctx.clearRect(0, 0, w, h);

    if (this.showCreatures) this.drawCreatures();
    if (this.showBrush) this.drawBrushRing();
  }

  // ── Creature markers ──────────────────────────────────────────────────────

  private drawCreatures(): void {
    const cache = this.renderer.getMaterialCache();
    if (!cache || cache.length === 0) return;

    const worldW = this.renderer.worldW;
    const worldH = this.renderer.worldH;
    if (worldW === 0 || worldH === 0) return;

    const zoom = this.cssZoom;

    // Visible cell window, clamped to the world.
    const x0 = Math.max(0, Math.floor(this.renderer.viewX));
    const y0 = Math.max(0, Math.floor(this.renderer.viewY));
    const x1 = Math.min(worldW, Math.ceil(this.renderer.viewX + this.canvas.width / zoom) + 1);
    const y1 = Math.min(worldH, Math.ceil(this.renderer.viewY + this.canvas.height / zoom) + 1);

    // Reset counts for this frame.
    this.counts[MAT_HERBIVORE] = 0;
    this.counts[MAT_SHEEP] = 0;
    this.counts[MAT_PREDATOR] = 0;

    // At high zoom the cells are already large enough to see, so markers would
    // only cover the art. Counting still runs so the HUD legend stays live.
    const drawMarkers = zoom < MARKER_ZOOM_THRESHOLD;

    // Marker size grows with zoom but stays legible when fully zoomed out.
    const radius = Math.max(MARKER_MIN_RADIUS, Math.min(MARKER_MAX_RADIUS, zoom * 1.6));

    const ctx = this.ctx;
    ctx.save();
    ctx.lineWidth = 1.25;

    for (let cy = y0; cy < y1; cy += SCAN_STRIDE) {
      const row = cy * worldW;
      for (let cx = x0; cx < x1; cx += SCAN_STRIDE) {
        const mat = cache[row + cx];
        if (mat !== MAT_HERBIVORE && mat !== MAT_SHEEP && mat !== MAT_PREDATOR) continue;

        this.counts[mat]++;
        if (!drawMarkers) continue;

        const style = CREATURE_STYLES[mat];
        const [sx, sy] = this.worldToOverlay(cx + 0.5, cy + 0.5);
        if (sx < -radius || sy < -radius) continue;
        if (sx > this.canvas.width + radius || sy > this.canvas.height + radius) continue;

        ctx.beginPath();
        ctx.arc(sx, sy, radius, 0, Math.PI * 2);
        ctx.fillStyle = style.fill;
        ctx.fill();
        ctx.strokeStyle = style.stroke;
        ctx.stroke();

        // A predator gets a second ring so it reads as a threat at a glance
        // without needing the legend.
        if (mat === MAT_PREDATOR) {
          ctx.beginPath();
          ctx.arc(sx, sy, radius + 2.5, 0, Math.PI * 2);
          ctx.strokeStyle = "rgba(200,84,47,0.55)";
          ctx.stroke();
        }
      }
    }

    ctx.restore();
  }

  // ── Brush ring ────────────────────────────────────────────────────────────

  private drawBrushRing(): void {
    if (!this.input.cursorOverCanvas) return;
    if (this.input.cursorClientX < 0) return;

    const rect = this.canvas.getBoundingClientRect();
    const cx = this.input.cursorClientX - rect.left;
    const cy = this.input.cursorClientY - rect.top;

    const radiusCells = this.input.effectiveRadius;
    const r = radiusCells * this.cssZoom;

    const ctx = this.ctx;
    ctx.save();

    const tint = this.brushTint();

    // Outer ring: the exact area the server will affect.
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.strokeStyle = tint.ring;
    ctx.lineWidth = 2;
    ctx.stroke();

    // A soft inner fill makes the extent readable over busy terrain without
    // hiding it.
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.fillStyle = tint.fill;
    ctx.fill();

    // Contrast ring just inside the first, so the brush stays visible on both
    // dark stone and bright sand.
    ctx.beginPath();
    ctx.arc(cx, cy, Math.max(0, r - 2), 0, Math.PI * 2);
    ctx.strokeStyle = "rgba(255,255,255,0.35)";
    ctx.lineWidth = 1;
    ctx.stroke();

    // Centre crosshair: the cell actually under the pointer.
    ctx.beginPath();
    ctx.moveTo(cx - 5, cy);
    ctx.lineTo(cx + 5, cy);
    ctx.moveTo(cx, cy - 5);
    ctx.lineTo(cx, cy + 5);
    ctx.strokeStyle = tint.ring;
    ctx.lineWidth = 1.5;
    ctx.stroke();

    // Radius readout, offset so it does not sit under the cursor.
    ctx.font = "600 11px 'JetBrains Mono', ui-monospace, monospace";
    ctx.textAlign = "left";
    ctx.textBaseline = "middle";
    const label = `${radiusCells}`;
    const lx = cx + r + 8;
    const ly = cy;
    ctx.fillStyle = "rgba(28,20,12,0.72)";
    const tw = ctx.measureText(label).width;
    ctx.beginPath();
    ctx.roundRect(lx - 3, ly - 8, tw + 8, 16, 4);
    ctx.fill();
    ctx.fillStyle = "#FDF8F0";
    ctx.fillText(label, lx + 1, ly + 0.5);

    ctx.restore();
  }

  /**
   * Ring colour for the active tool, so the brush itself says what it will do.
   * Guessing from the toolbar alone was a constant source of misplaced edits.
   */
  private brushTint(): { ring: string; fill: string } {
    const tool = this.input.tool;
    if (tool === "erase") {
      return { ring: "rgba(198,64,48,0.95)", fill: "rgba(198,64,48,0.10)" };
    }
    if (tool === "raise") {
      return { ring: "rgba(120,92,52,0.95)", fill: "rgba(160,124,72,0.12)" };
    }
    if (tool === "lower") {
      return { ring: "rgba(88,104,132,0.95)", fill: "rgba(110,132,168,0.12)" };
    }
    if (tool === "place") {
      return { ring: "rgba(58,132,108,0.95)", fill: "rgba(78,164,132,0.12)" };
    }
    // Elemental forces are tinted per power: rain, heat, wind, growth, quake.
    const forceTints = [
      { ring: "rgba(58,110,180,0.95)", fill: "rgba(78,140,210,0.12)" }, // rain
      { ring: "rgba(212,108,42,0.95)", fill: "rgba(232,132,58,0.12)" }, // heat
      { ring: "rgba(120,150,168,0.95)", fill: "rgba(150,180,198,0.12)" }, // wind
      { ring: "rgba(78,152,72,0.95)", fill: "rgba(102,182,96,0.12)" },  // growth
      { ring: "rgba(140,96,64,0.95)", fill: "rgba(170,124,88,0.12)" },  // quake
    ];
    return forceTints[this.input.activePowerIndex] ?? forceTints[0];
  }

  /** Total creature population currently on screen. */
  get visibleCreatureTotal(): number {
    return this.counts[MAT_HERBIVORE] + this.counts[MAT_SHEEP] + this.counts[MAT_PREDATOR];
  }
}
