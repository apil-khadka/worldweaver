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

import { WorldNetwork, IGameRenderer, type GodTool } from "./network.js";
import { ClientPrediction } from "./prediction.js";
import { AudioEngine } from "./audio.js";

const POWER_KEYS: Record<string, number> = {
  "1": 0, "2": 1, "3": 2, "4": 3, "5": 4,
};

/** Must match game.MaxToolRadius on the server. */
const MAX_BRUSH_RADIUS = 32;

// ── Camera momentum constants ────────────────────────────────────────────────
//
// Speeds are expressed in screen pixels per frame and converted to world cells
// using the current zoom. Working in cells made panning feel wildly different
// between zoom levels, because a cell is a different number of pixels at each.
const CAM_MAX_SPEED_PX = 14;   // screen px/frame
const CAM_ACCEL_PX     = 2.0;  // screen px/frame²
const CAM_FRICTION     = 0.85; // velocity multiplier per frame when no input

// A wide world is traversed mostly sideways, so the horizontal axis is given a
// little more speed than the vertical.
const CAM_HORIZONTAL_BOOST = 1.4;

/** Viewport updates are sent at this rate while panning, not every frame. */
const VIEWPORT_SEND_INTERVAL_MS = 100;

// ── Screen Shake constants ───────────────────────────────────────────────────
const SHAKE_RADIUS_THRESHOLD = 20; // trigger shake when radius > this

export class InputHandler {
  private applying = false;
  private lastPowerX = 0;
  private lastPowerY = 0;

  // ── God-mode tool state ──────────────────────────────────────────────────
  private activeTool: "force" | GodTool = "force";
  private activeMaterial = 4; // water
  private brushRadius = 8;

  // ── Camera momentum state ────────────────────────────────────────────────
  private panKeys = new Set<string>();
  private velX = 0;
  private velY = 0;
  private animFrame: number | null = null;

  private lastCursorSend = 0;
  private readonly CURSOR_THROTTLE_MS = 100; // 10Hz
  private lastViewportSend = 0;
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
        // Don't allow selecting locked powers
        if (btn.classList.contains("power-locked")) return;
        const p = parseInt(btn.dataset["power"] ?? "0", 10);
        this.network.activePower = p;
        // Choosing a force leaves god-mode editing.
        this.selectTool("force");
        this.updatePowerButtons(p);
        this.bouncePowerButton(btn);
      });
    });

    this.attachGodTools();

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

    // Direct world edit: no prediction, because the outcome depends on terrain
    // the server owns (raise and lower only act on exposed ground).
    if (this.activeTool !== "force") {
      this.network.sendEdit(this.activeTool, this.activeMaterial, wx, wy, this.brushRadius);
      return;
    }

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

  // ── God-mode tool wiring ─────────────────────────────────────────────────

  private attachGodTools(): void {
    document.querySelectorAll<HTMLButtonElement>(".tool-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const tool = btn.dataset["tool"] as GodTool | undefined;
        if (!tool) return;
        // Clicking the active tool switches back to the elemental forces.
        this.selectTool(this.activeTool === tool ? "force" : tool);
      });
    });

    document.querySelectorAll<HTMLButtonElement>(".mat-swatch").forEach((btn) => {
      btn.addEventListener("click", () => {
        const mat = parseInt(btn.dataset["material"] ?? "4", 10);
        this.activeMaterial = mat;
        document.querySelectorAll(".mat-swatch").forEach((b) => b.classList.remove("active"));
        btn.classList.add("active");
      });
    });

    const slider = document.getElementById("brush-slider") as HTMLInputElement | null;
    slider?.addEventListener("input", () => {
      this.setBrushRadius(parseInt(slider.value, 10));
    });
  }

  /**
   * Switches tool and reconciles the UI: only one of the force bar and the tool
   * bar is active at a time, and the material palette follows the place tool.
   */
  private selectTool(tool: "force" | GodTool): void {
    this.activeTool = tool;

    document.querySelectorAll<HTMLButtonElement>(".tool-btn").forEach((b) => {
      b.classList.toggle("active", b.dataset["tool"] === tool);
    });

    // The force bar only reads as selected while a force is actually in use.
    if (tool !== "force") {
      document.querySelectorAll(".power-btn").forEach((b) => b.classList.remove("active"));
    } else {
      this.updatePowerButtons(this.network.activePower);
    }

    document.getElementById("material-palette")
      ?.classList.toggle("visible", tool === "place");
    document.getElementById("brush-control")
      ?.classList.toggle("visible", tool !== "force");
  }

  /** Selects the material painted by the place tool. */
  setMaterial(material: number): void {
    this.activeMaterial = material;
  }

  /** Adjusts the brush radius, clamped to the server's cap. */
  setBrushRadius(radius: number): void {
    this.brushRadius = Math.max(1, Math.min(MAX_BRUSH_RADIUS, Math.round(radius)));
    const label = document.getElementById("brush-value");
    if (label) label.textContent = String(this.brushRadius);
    const slider = document.getElementById("brush-slider") as HTMLInputElement | null;
    if (slider) slider.value = String(this.brushRadius);
  }

  get tool(): "force" | GodTool {
    return this.activeTool;
  }

  get brush(): number {
    return this.brushRadius;
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
      const btn = document.querySelector<HTMLButtonElement>(`.power-btn[data-power="${p}"]`);
      // Don't allow selecting locked powers via keyboard
      if (btn?.classList.contains("power-locked")) return;
      this.network.activePower = p;
      this.selectTool("force");
      this.updatePowerButtons(p);
      if (btn) this.bouncePowerButton(btn);
      return;
    }

    // God-mode tool selection
    const toolKeys: Record<string, GodTool> = {
      p: "place", P: "place",
      e: "erase", E: "erase",
      r: "raise", R: "raise",
      f: "lower", F: "lower",
    };
    if (e.key in toolKeys) {
      const tool = toolKeys[e.key];
      this.selectTool(this.activeTool === tool ? "force" : tool);
      return;
    }

    // Brush size
    if (e.key === "[") {
      this.setBrushRadius(this.brushRadius - 2);
      return;
    }
    if (e.key === "]") {
      this.setBrushRadius(this.brushRadius + 2);
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

    // Convert pixel-space speeds into world cells at the current zoom, so a key
    // press moves the view by the same visible distance however far in you are.
    const zoom = this.renderer.zoom || 1;
    const accel = (CAM_ACCEL_PX / zoom);
    const maxSpeed = (CAM_MAX_SPEED_PX / zoom);

    // Apply acceleration
    if (inputX !== 0) {
      this.velX += inputX * accel * CAM_HORIZONTAL_BOOST;
    } else {
      this.velX *= CAM_FRICTION;
    }
    if (inputY !== 0) {
      this.velY += inputY * accel;
    } else {
      this.velY *= CAM_FRICTION;
    }

    // Clamp velocity
    const maxX = maxSpeed * CAM_HORIZONTAL_BOOST;
    this.velX = Math.max(-maxX, Math.min(maxX, this.velX));
    this.velY = Math.max(-maxSpeed, Math.min(maxSpeed, this.velY));

    // Kill tiny residual velocity
    if (Math.abs(this.velX) < 0.05) this.velX = 0;
    if (Math.abs(this.velY) < 0.05) this.velY = 0;

    if (this.velX === 0 && this.velY === 0) return;

    // Move viewport (account for zoom: visible world cells = canvas / zoom)
    const maxViewX = Math.max(0, this.renderer.worldW - this.renderer.visibleW);
    const maxViewY = Math.max(0, this.renderer.worldH - this.renderer.visibleH);

    this.renderer.viewX = Math.max(0, Math.min(maxViewX, this.renderer.viewX + this.velX));
    this.renderer.viewY = Math.max(0, Math.min(maxViewY, this.renderer.viewY + this.velY));

    // Trigger redraw
    this.renderer.drawImmediate();

    // Notify the server at a fixed rate rather than on every animation frame.
    // Panning previously emitted 60 messages a second per client, each of which
    // the server processed and considered for a fresh snapshot.
    const now = Date.now();
    if (now - this.lastViewportSend >= VIEWPORT_SEND_INTERVAL_MS) {
      this.lastViewportSend = now;
      this.network.sendViewport(
        this.renderer.viewX, this.renderer.viewY,
        this.renderer.visibleW, this.renderer.visibleH,
      );
    }
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
