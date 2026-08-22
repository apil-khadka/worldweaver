/**
 * input.ts — Pointer and keyboard input handler
 *
 * Captures:
 *  - Mouse click/drag → apply selected power at world coordinates
 *  - Touch drag       → apply selected power (mobile)
 *  - WASD / arrow     → pan camera with momentum (acceleration/friction)
 *  - Number keys 1-4  → select power
 *  - Scroll wheel / pinch → zoom (0.5x, 1x, 2x, 4x)
 *
 * Converts screen coordinates to world coordinates by subtracting the
 * renderer's viewport origin and accounting for zoom scale.
 * The renderer is the single source of truth for the current viewport position.
 */

import { WorldNetwork, IGameRenderer } from "./network.js";
import { ClientPrediction } from "./prediction.js";
import { AudioEngine } from "./audio.js";

const POWER_KEYS: Record<string, number> = {
  "1": 0, "2": 1, "3": 2, "4": 3,
};

// ── Camera momentum constants ────────────────────────────────────────────────
const CAM_MAX_SPEED   = 16;   // cells/frame
const CAM_ACCEL       = 2;    // cells/frame²
const CAM_FRICTION    = 0.85; // velocity multiplier per frame when no input

// ── Screen Shake constants ───────────────────────────────────────────────────
const SHAKE_RADIUS_THRESHOLD = 20; // trigger shake when radius > this

export class InputHandler {
  private applying = false;
  private lastPowerX = 0;
  private lastPowerY = 0;

  // ── Camera momentum state ────────────────────────────────────────────────
  private panKeys = new Set<string>();
  private velX = 0;
  private velY = 0;
  private animFrame: number | null = null;

  private lastCursorSend = 0;
  private readonly CURSOR_THROTTLE_MS = 100; // 10Hz
  prediction: ClientPrediction | null = null;

  // ── Zoom indicator ────────────────────────────────────────────────────────
  private zoomIndicator: HTMLElement | null = null;
  private zoomHideTimeout: ReturnType<typeof setTimeout> | null = null;

  // ── Screen shake ────────────────────────────────────────────────────────
  private canvasWrapper: HTMLElement | null = null;
  private shaking = false;

  // ── Shortcuts overlay ───────────────────────────────────────────────────
  private shortcutsOverlay: HTMLElement | null = null;

  constructor(
    private readonly canvas: HTMLCanvasElement,
    private readonly network: WorldNetwork,
    private renderer: IGameRenderer,
  ) {
    this.zoomIndicator = document.getElementById("zoom-indicator");
    this.canvasWrapper = document.getElementById("canvas-wrapper");
    this.shortcutsOverlay = document.getElementById("shortcuts-overlay");
  }

  /** Hot-swap the renderer backend (used by view mode toggle). */
  swapRenderer(r: IGameRenderer): void {
    this.renderer = r;
  }

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

    // Zoom: mouse wheel
    this.canvas.addEventListener("wheel", (e) => this.onWheel(e), { passive: false });

    // Power button clicks in the toolbar
    document.querySelectorAll<HTMLButtonElement>(".power-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const p = parseInt(btn.dataset["power"] ?? "0", 10);
        this.network.activePower = p;
        this.updatePowerButtons(p);
        this.bouncePowerButton(btn);
      });
    });

    // Shortcuts overlay toggle
    const shortcutsBtn = document.getElementById("shortcuts-btn");
    if (shortcutsBtn) {
      shortcutsBtn.addEventListener("click", () => this.toggleShortcuts());
    }
    this.shortcutsOverlay?.addEventListener("click", (e) => {
      if (e.target === this.shortcutsOverlay) this.toggleShortcuts();
    });

    // Start camera animation loop
    this.startCameraLoop();
  }

  private screenToWorld(clientX: number, clientY: number): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    const sx   = clientX - rect.left;
    const sy   = clientY - rect.top;
    const zoom = this.renderer.zoom;
    return [
      Math.floor(this.renderer.viewX + sx / zoom),
      Math.floor(this.renderer.viewY + sy / zoom),
    ];
  }

  private applyPowerAt(clientX: number, clientY: number): void {
    const [wx, wy] = this.screenToWorld(clientX, clientY);
    this.lastPowerX = wx;
    this.lastPowerY = wy;
    // Client-side prediction: apply visual immediately before server round-trip
    const radius = 24; // default server radius
    this.prediction?.predict(this.network.activePower, wx, wy, radius);
    this.network.sendPower(this.network.activePower, wx, wy);
    // Procedural sound effect for power application
    AudioEngine.getInstance().playPower(this.network.activePower as 0 | 1 | 2 | 3);
    // Screen shake for large applications
    if (radius > SHAKE_RADIUS_THRESHOLD) {
      this.triggerShake();
    }
  }

  private onMouseDown(e: MouseEvent): void {
    if (e.button !== 0) return;
    this.applying = true;
    this.applyPowerAt(e.clientX, e.clientY);
  }

  private onMouseMove(e: MouseEvent): void {
    // Send cursor position for multiplayer presence (throttled)
    const now = Date.now();
    if (now - this.lastCursorSend > this.CURSOR_THROTTLE_MS) {
      const [wx, wy] = this.screenToWorld(e.clientX, e.clientY);
      this.network.sendCursor(wx, wy, this.network.activePower);
      this.lastCursorSend = now;
    }
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

  // ── Zoom ─────────────────────────────────────────────────────────────────
  private onWheel(e: WheelEvent): void {
    e.preventDefault();
    const direction: 1 | -1 = e.deltaY < 0 ? 1 : -1;
    const newZoom = this.renderer.stepZoom(direction);
    this.showZoomIndicator(newZoom);

    // Notify network of new visible area
    this.network.sendViewport(
      this.renderer.viewX, this.renderer.viewY,
      this.renderer.visibleW, this.renderer.visibleH,
    );
  }

  private showZoomIndicator(zoom: number): void {
    if (!this.zoomIndicator) return;
    this.zoomIndicator.textContent = zoom < 1 ? `${zoom}×` : `${zoom}×`;
    this.zoomIndicator.classList.add("visible");
    if (this.zoomHideTimeout) clearTimeout(this.zoomHideTimeout);
    this.zoomHideTimeout = setTimeout(() => {
      this.zoomIndicator?.classList.remove("visible");
    }, 1500);
  }

  // ── Keyboard ─────────────────────────────────────────────────────────────
  private onKeyDown(e: KeyboardEvent): void {
    // Shortcuts overlay toggle
    if (e.key === "?") {
      this.toggleShortcuts();
      return;
    }

    // Minimap toggle
    if (e.key === "m" || e.key === "M") {
      const minimap = document.getElementById("minimap");
      if (minimap) {
        minimap.style.display = minimap.style.display === "none" ? "" : "none";
      }
      return;
    }

    // Power selection
    if (e.key in POWER_KEYS) {
      const p = POWER_KEYS[e.key];
      this.network.activePower = p;
      this.updatePowerButtons(p);
      // Bounce the selected button
      const btn = document.querySelector<HTMLButtonElement>(`.power-btn[data-power="${p}"]`);
      if (btn) this.bouncePowerButton(btn);
      return;
    }

    // Zoom with +/- keys
    if (e.key === "=" || e.key === "+") {
      const z = this.renderer.stepZoom(1);
      this.showZoomIndicator(z);
      return;
    }
    if (e.key === "-") {
      const z = this.renderer.stepZoom(-1);
      this.showZoomIndicator(z);
      return;
    }

    // Camera pan keys
    this.panKeys.add(e.key);
  }

  private onKeyUp(e: KeyboardEvent): void {
    this.panKeys.delete(e.key);
  }

  // ── Camera animation loop (requestAnimationFrame) ────────────────────────
  private startCameraLoop(): void {
    const tick = () => {
      this.updateCamera();
      this.animFrame = requestAnimationFrame(tick);
    };
    this.animFrame = requestAnimationFrame(tick);
  }

  private updateCamera(): void {
    // Determine input direction
    let inputX = 0;
    let inputY = 0;
    if (this.panKeys.has("ArrowLeft")  || this.panKeys.has("a")) inputX -= 1;
    if (this.panKeys.has("ArrowRight") || this.panKeys.has("d")) inputX += 1;
    if (this.panKeys.has("ArrowUp")    || this.panKeys.has("w")) inputY -= 1;
    if (this.panKeys.has("ArrowDown")  || this.panKeys.has("s")) inputY += 1;

    // Apply acceleration
    if (inputX !== 0) {
      this.velX += inputX * CAM_ACCEL;
    } else {
      this.velX *= CAM_FRICTION;
    }
    if (inputY !== 0) {
      this.velY += inputY * CAM_ACCEL;
    } else {
      this.velY *= CAM_FRICTION;
    }

    // Clamp velocity
    this.velX = Math.max(-CAM_MAX_SPEED, Math.min(CAM_MAX_SPEED, this.velX));
    this.velY = Math.max(-CAM_MAX_SPEED, Math.min(CAM_MAX_SPEED, this.velY));

    // Kill tiny residual velocity
    if (Math.abs(this.velX) < 0.1) this.velX = 0;
    if (Math.abs(this.velY) < 0.1) this.velY = 0;

    if (this.velX === 0 && this.velY === 0) return;

    // Move viewport (account for zoom: visible world cells = canvas / zoom)
    const maxViewX = Math.max(0, this.renderer.worldW - this.renderer.visibleW);
    const maxViewY = Math.max(0, this.renderer.worldH - this.renderer.visibleH);

    this.renderer.viewX = Math.max(0, Math.min(maxViewX, this.renderer.viewX + this.velX));
    this.renderer.viewY = Math.max(0, Math.min(maxViewY, this.renderer.viewY + this.velY));

    // Trigger redraw
    this.renderer.drawImmediate();

    // Notify server of viewport change
    this.network.sendViewport(
      this.renderer.viewX, this.renderer.viewY,
      this.renderer.visibleW, this.renderer.visibleH,
    );
  }

  private updatePowerButtons(active: number): void {
    document.querySelectorAll<HTMLButtonElement>(".power-btn").forEach((btn) => {
      const p = parseInt(btn.dataset["power"] ?? "0", 10);
      btn.classList.toggle("active", p === active);
    });
  }

  // ── Screen Shake ─────────────────────────────────────────────────────────
  triggerShake(): void {
    if (this.shaking || !this.canvasWrapper) return;
    this.shaking = true;
    this.canvasWrapper.classList.add("shake");
    this.canvasWrapper.addEventListener("animationend", () => {
      this.canvasWrapper!.classList.remove("shake");
      this.shaking = false;
    }, { once: true });
  }

  // ── Power Button Bounce ──────────────────────────────────────────────────
  private bouncePowerButton(btn: HTMLButtonElement): void {
    btn.classList.remove("bounce", "pulse-border");
    // Force reflow to restart animation
    void btn.offsetWidth;
    btn.classList.add("bounce", "pulse-border");
    btn.addEventListener("animationend", () => {
      btn.classList.remove("bounce", "pulse-border");
    }, { once: true });
  }

  // ── Shortcuts Overlay ────────────────────────────────────────────────────
  private toggleShortcuts(): void {
    if (!this.shortcutsOverlay) return;
    this.shortcutsOverlay.classList.toggle("visible");
  }
}
