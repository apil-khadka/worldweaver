/**
 * effects.ts — Power visual feedback overlay
 *
 * Renders on a dedicated transparent canvas layered above the world canvas:
 *  1. Radius indicator: semi-transparent circle following the cursor,
 *     coloured by the active power and sized to the server's default radius (24 cells).
 *  2. Application flash: expanding ring that fades out over 300ms on click/drag.
 */

import { IGameRenderer } from "./network.js";

/** Power index → RGBA colour string */
const POWER_COLORS: Record<number, string> = {
  0: "rgba(30, 100, 220, 0.25)",   // Rain — blue
  1: "rgba(240, 140, 20, 0.25)",   // Heat — orange
  2: "rgba(50, 200, 80, 0.25)",    // Wind — green
  3: "rgba(0, 180, 160, 0.25)",    // Growth — teal
};

const POWER_STROKE_COLORS: Record<number, string> = {
  0: "rgba(30, 100, 220, 0.6)",
  1: "rgba(240, 140, 20, 0.6)",
  2: "rgba(50, 200, 80, 0.6)",
  3: "rgba(0, 180, 160, 0.6)",
};

const POWER_FLASH_COLORS: Record<number, string> = {
  0: "rgba(30, 100, 220, 1)",
  1: "rgba(240, 140, 20, 1)",
  2: "rgba(50, 200, 80, 1)",
  3: "rgba(0, 180, 160, 1)",
};

const RADIUS = 24; // cells — matches server default power radius
const FLASH_DURATION = 300; // ms
const GLOW_DURATION = 500; // ms — slightly longer than flash for prediction

interface FlashEffect {
  x: number;          // screen x
  y: number;          // screen y
  startTime: number;  // performance.now() when created
  power: number;      // power index for colour
}

interface GlowEffect {
  wx: number;         // world x
  wy: number;         // world y
  radius: number;     // world-space radius
  startTime: number;
  power: number;
}

export class PowerEffects {
  private readonly ctx: CanvasRenderingContext2D;
  private cursorX = -1;
  private cursorY = -1;
  private cursorVisible = false;
  private activePower = 0;
  private flashes: FlashEffect[] = [];
  private glows: GlowEffect[] = [];
  private animating = false;

  constructor(
    private readonly overlay: HTMLCanvasElement,
    private readonly worldCanvas: HTMLCanvasElement,
    private readonly renderer: IGameRenderer,
  ) {
    const ctx = overlay.getContext("2d");
    if (!ctx) throw new Error("Overlay canvas 2D not supported");
    this.ctx = ctx;
  }

  attach(): void {
    this.worldCanvas.addEventListener("mousemove", (e) => this.onMouseMove(e));
    this.worldCanvas.addEventListener("mouseenter", () => { this.cursorVisible = true; this.scheduleFrame(); });
    this.worldCanvas.addEventListener("mouseleave", () => { this.cursorVisible = false; this.scheduleFrame(); });
    this.worldCanvas.addEventListener("mousedown", (e) => this.onApply(e));

    // Touch support
    this.worldCanvas.addEventListener("touchstart", (e) => {
      if (e.touches.length === 1) {
        const t = e.touches[0];
        this.triggerFlash(t.clientX, t.clientY);
      }
    }, { passive: true });

    this.scheduleFrame();
  }

  /** Call from main when active power changes */
  setActivePower(power: number): void {
    this.activePower = power;
    this.scheduleFrame();
  }

  /**
   * Trigger a colored glow/pulse at a world position — called by ClientPrediction
   * to reinforce the instant-feedback feel.
   */
  triggerGlow(wx: number, wy: number, power: number, radius: number): void {
    this.glows.push({
      wx, wy, radius,
      startTime: performance.now(),
      power,
    });
    this.scheduleFrame();
  }

  /** Called on resize to sync overlay dimensions */
  resize(w: number, h: number): void {
    this.overlay.width = w;
    this.overlay.height = h;
    this.scheduleFrame();
  }

  private onMouseMove(e: MouseEvent): void {
    const rect = this.worldCanvas.getBoundingClientRect();
    this.cursorX = e.clientX - rect.left;
    this.cursorY = e.clientY - rect.top;
    this.cursorVisible = true;
    this.scheduleFrame();
  }

  private onApply(e: MouseEvent): void {
    if (e.button !== 0) return;
    this.triggerFlash(e.clientX, e.clientY);

    // Also trigger on drag
    const onDragApply = (me: MouseEvent) => {
      this.triggerFlash(me.clientX, me.clientY);
    };
    const onUp = () => {
      this.worldCanvas.removeEventListener("mousemove", onDragApply);
      window.removeEventListener("mouseup", onUp);
    };
    // Throttle drag flashes to avoid overwhelming
    let lastFlash = Date.now();
    const throttledDrag = (me: MouseEvent) => {
      if (Date.now() - lastFlash > 80) {
        lastFlash = Date.now();
        onDragApply(me);
      }
    };
    this.worldCanvas.addEventListener("mousemove", throttledDrag);
    window.addEventListener("mouseup", () => {
      this.worldCanvas.removeEventListener("mousemove", throttledDrag);
    }, { once: true });
  }

  private triggerFlash(clientX: number, clientY: number): void {
    const rect = this.worldCanvas.getBoundingClientRect();
    this.flashes.push({
      x: clientX - rect.left,
      y: clientY - rect.top,
      startTime: performance.now(),
      power: this.activePower,
    });
    this.scheduleFrame();
  }

  private scheduleFrame(): void {
    if (this.animating) return;
    this.animating = true;
    requestAnimationFrame((t) => this.render(t));
  }

  private render(now: number): void {
    this.animating = false;
    const { ctx, overlay } = this;
    const w = overlay.width;
    const h = overlay.height;

    ctx.clearRect(0, 0, w, h);

    // ── Radius indicator ───────────────────────────────────────
    if (this.cursorVisible && this.cursorX >= 0 && this.cursorY >= 0) {
      const fill = POWER_COLORS[this.activePower] ?? POWER_COLORS[0];
      const stroke = POWER_STROKE_COLORS[this.activePower] ?? POWER_STROKE_COLORS[0];

      ctx.beginPath();
      ctx.arc(this.cursorX, this.cursorY, RADIUS, 0, Math.PI * 2);
      ctx.fillStyle = fill;
      ctx.fill();
      ctx.strokeStyle = stroke;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }

    // ── Flash effects ──────────────────────────────────────────
    const aliveFlashes: FlashEffect[] = [];
    for (const flash of this.flashes) {
      const elapsed = now - flash.startTime;
      if (elapsed >= FLASH_DURATION) continue;

      aliveFlashes.push(flash);
      const progress = elapsed / FLASH_DURATION; // 0→1
      const ringRadius = RADIUS * (1 + progress * 0.6); // expands 60%
      const alpha = 1 - progress; // fades out

      const baseColor = POWER_FLASH_COLORS[flash.power] ?? POWER_FLASH_COLORS[0];
      // Replace alpha in the rgba string
      const color = baseColor.replace(/[\d.]+\)$/, `${alpha.toFixed(2)})`);

      ctx.beginPath();
      ctx.arc(flash.x, flash.y, ringRadius, 0, Math.PI * 2);
      ctx.strokeStyle = color;
      ctx.lineWidth = 3 * (1 - progress * 0.5); // thins as it expands
      ctx.stroke();
    }
    this.flashes = aliveFlashes;

    // ── Glow effects (world-space, for prediction feedback) ────
    const aliveGlows: GlowEffect[] = [];
    for (const glow of this.glows) {
      const elapsed = now - glow.startTime;
      if (elapsed >= GLOW_DURATION) continue;

      aliveGlows.push(glow);
      const progress = elapsed / GLOW_DURATION; // 0→1

      // Convert world coords to screen coords
      const sx = glow.wx - this.renderer.viewX;
      const sy = glow.wy - this.renderer.viewY;

      // Skip if off-screen
      if (sx < -glow.radius || sx > w + glow.radius ||
          sy < -glow.radius || sy > h + glow.radius) continue;

      const alpha = (1 - progress) * 0.4; // fade from 0.4 to 0
      const pulseRadius = glow.radius * (0.8 + progress * 0.4); // subtle pulse outward

      const baseColor = POWER_FLASH_COLORS[glow.power] ?? POWER_FLASH_COLORS[0];
      const glowColor = baseColor.replace(/[\d.]+\)$/, `${alpha.toFixed(3)})`);

      // Radial gradient glow
      const grad = ctx.createRadialGradient(sx, sy, 0, sx, sy, pulseRadius);
      grad.addColorStop(0, glowColor);
      grad.addColorStop(0.6, glowColor.replace(/[\d.]+\)$/, `${(alpha * 0.4).toFixed(3)})`));
      grad.addColorStop(1, "rgba(0,0,0,0)");

      ctx.beginPath();
      ctx.arc(sx, sy, pulseRadius, 0, Math.PI * 2);
      ctx.fillStyle = grad;
      ctx.fill();
    }
    this.glows = aliveGlows;

    // Keep animating if there are active flashes or glows
    if (this.flashes.length > 0 || this.glows.length > 0) {
      this.animating = true;
      requestAnimationFrame((t) => this.render(t));
    }
  }
}
