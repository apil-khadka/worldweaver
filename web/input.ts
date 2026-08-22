/**
 * input.ts — Pointer and keyboard input handler
 *
 * Captures:
 *  - Mouse click/drag → apply selected power at world coordinates
 *  - Touch drag       → apply selected power (mobile)
 *  - WASD / arrow     → pan camera
 *  - Number keys 1-4  → select power
 *  - Scroll wheel     → zoom (stretch feature, currently stubbed)
 *
 * Converts screen coordinates to world coordinates by subtracting the
 * renderer's viewport origin.  The renderer is the single source of truth
 * for the current viewport position.
 */

import { WorldNetwork } from "./network.js";
import { WorldRenderer } from "./renderer.js";

const POWER_KEYS: Record<string, number> = {
  "1": 0, "2": 1, "3": 2, "4": 3,
};
const CAM_SPEED = 8; // cells per keyboard press

export class InputHandler {
  private applying = false;
  private lastPowerX = 0;
  private lastPowerY = 0;
  private panKeys = new Set<string>();
  private panLoop: ReturnType<typeof setInterval> | null = null;

  constructor(
    private readonly canvas: HTMLCanvasElement,
    private readonly network: WorldNetwork,
    private readonly renderer: WorldRenderer,
  ) {}

  attach(): void {
    // Mouse
    this.canvas.addEventListener("mousedown",  (e) => this.onMouseDown(e));
    this.canvas.addEventListener("mousemove",  (e) => this.onMouseMove(e));
    this.canvas.addEventListener("mouseup",    ()  => this.onPointerEnd());
    this.canvas.addEventListener("mouseleave", ()  => this.onPointerEnd());

    // Touch (mobile)
    this.canvas.addEventListener("touchstart", (e) => this.onTouchStart(e), { passive: true });
    this.canvas.addEventListener("touchmove",  (e) => this.onTouchMove(e),  { passive: true });
    this.canvas.addEventListener("touchend",   ()  => this.onPointerEnd());

    // Keyboard
    window.addEventListener("keydown", (e) => this.onKeyDown(e));
    window.addEventListener("keyup",   (e) => this.onKeyUp(e));

    // Power button clicks in the toolbar
    document.querySelectorAll<HTMLButtonElement>(".power-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const p = parseInt(btn.dataset["power"] ?? "0", 10);
        this.network.activePower = p;
        this.updatePowerButtons(p);
      });
    });
  }

  private screenToWorld(clientX: number, clientY: number): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    const sx   = clientX - rect.left;
    const sy   = clientY - rect.top;
    return [
      Math.floor(this.renderer.viewX + sx),
      Math.floor(this.renderer.viewY + sy),
    ];
  }

  private applyPowerAt(clientX: number, clientY: number): void {
    const [wx, wy] = this.screenToWorld(clientX, clientY);
    this.lastPowerX = wx;
    this.lastPowerY = wy;
    this.network.sendPower(this.network.activePower, wx, wy);
  }

  private onMouseDown(e: MouseEvent): void {
    if (e.button !== 0) return;
    this.applying = true;
    this.applyPowerAt(e.clientX, e.clientY);
  }

  private onMouseMove(e: MouseEvent): void {
    if (!this.applying) return;
    this.applyPowerAt(e.clientX, e.clientY);
  }

  private onTouchStart(e: TouchEvent): void {
    if (e.touches.length === 1) {
      this.applying = true;
      this.applyPowerAt(e.touches[0].clientX, e.touches[0].clientY);
    }
  }

  private onTouchMove(e: TouchEvent): void {
    if (!this.applying || e.touches.length !== 1) return;
    this.applyPowerAt(e.touches[0].clientX, e.touches[0].clientY);
  }

  private onPointerEnd(): void {
    this.applying = false;
  }

  private onKeyDown(e: KeyboardEvent): void {
    // Power selection
    if (e.key in POWER_KEYS) {
      const p = POWER_KEYS[e.key];
      this.network.activePower = p;
      this.updatePowerButtons(p);
      return;
    }

    // Camera pan
    this.panKeys.add(e.key);
    if (!this.panLoop) {
      this.panLoop = setInterval(() => this.processPan(), 33);
    }
  }

  private onKeyUp(e: KeyboardEvent): void {
    this.panKeys.delete(e.key);
    if (this.panKeys.size === 0 && this.panLoop) {
      clearInterval(this.panLoop);
      this.panLoop = null;
    }
  }

  private processPan(): void {
    let dx = 0;
    let dy = 0;
    if (this.panKeys.has("ArrowLeft")  || this.panKeys.has("a")) dx -= CAM_SPEED;
    if (this.panKeys.has("ArrowRight") || this.panKeys.has("d")) dx += CAM_SPEED;
    if (this.panKeys.has("ArrowUp")    || this.panKeys.has("w")) dy -= CAM_SPEED;
    if (this.panKeys.has("ArrowDown")  || this.panKeys.has("s")) dy += CAM_SPEED;

    if (dx === 0 && dy === 0) return;

    this.renderer.viewX = Math.max(0,
      Math.min(this.renderer.worldW - this.canvas.width,
        this.renderer.viewX + dx));
    this.renderer.viewY = Math.max(0,
      Math.min(this.renderer.worldH - this.canvas.height,
        this.renderer.viewY + dy));

    this.network.sendViewport(
      this.renderer.viewX, this.renderer.viewY,
      this.canvas.width,   this.canvas.height,
    );
  }

  private updatePowerButtons(active: number): void {
    document.querySelectorAll<HTMLButtonElement>(".power-btn").forEach((btn) => {
      const p = parseInt(btn.dataset["power"] ?? "0", 10);
      btn.classList.toggle("active", p === active);
    });
  }
}
