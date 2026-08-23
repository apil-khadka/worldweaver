/**
 * input.ts — Pointer and keyboard input handler
 *
 * Captures:
 *  - Mouse press/drag → apply selected power at world coordinates, and keep
 *    applying while held even if the pointer is still
 *  - Touch drag       → same, for mobile
 *  - WASD / arrows    → pan camera with momentum (A/D are left/right)
 *  - Number keys 1-5  → select power
 *  - +/- or buttons   → zoom (wheel and pinch zoom are deliberately unbound)
 *
 * Converts screen coordinates to world coordinates by subtracting the canvas
 * rect origin, scaling CSS pixels into backing-store pixels, and dividing by
 * zoom. The renderer is the single source of truth for the viewport position.
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
//
// The threshold sits near the top of the 1..32 brush range so shake is reserved
// for a deliberately huge application. It used to be 20, which the default brush
// reached easily once forces started honouring the brush radius, so ordinary
// painting shook the canvas.
const SHAKE_RADIUS_THRESHOLD = 28;

export class InputHandler {
  private applying = false;
  private lastPowerX = 0;
  private lastPowerY = 0;

  // ── Continuous application ───────────────────────────────────────────────
  //
  // Holding the button down used to do nothing unless the pointer also moved,
  // because application was driven purely by mousemove events. A repeat timer
  // makes press-and-hold keep painting at a fixed rate, which is what every
  // other sandbox game does. The rate sits just under the server's 10/s power
  // budget so holding still never trips the rate limiter.
  private repeatTimer: ReturnType<typeof setInterval> | null = null;
  private lastClientX = 0;
  private lastClientY = 0;
  private static readonly REPEAT_INTERVAL_MS = 120; // ≈8/s, server allows 10/s

  /**
   * True while the current application came from the repeat timer rather than a
   * fresh press or drag.
   *
   * Effects that represent an IMPACT — screen shake, the power sound — must fire on
   * the first application and not on each repeat. Firing them per repeat meant
   * eight shakes and eight sound triggers a second for as long as the button was
   * held, which read as the canvas vibrating and the audio buzzing.
   */
  private isRepeat = false;

  /** Throttle for the power sound, so a held brush does not machine-gun it. */
  private lastPowerSoundAt = 0;
  private static readonly POWER_SOUND_INTERVAL_MS = 260;

  /** Latest pointer position in CSS pixels, for the brush preview ring. */
  cursorClientX = -1;
  cursorClientY = -1;
  cursorOverCanvas = false;

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
  private lastShakeAt = 0;
  /** Hard floor between shakes, whatever else asks for one. */
  private static readonly SHAKE_COOLDOWN_MS = 900;

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
    this.canvas.addEventListener("mouseenter", ()  => this.onMouseEnter());
    this.canvas.addEventListener("mouseleave", ()  => {
      this.cursorOverCanvas = false;
      this.onPointerEnd();
    });
    // A button released outside the canvas must still stop painting, or the
    // repeat timer keeps firing after the player has let go.
    window.addEventListener("mouseup", () => this.onPointerEnd());
    window.addEventListener("blur",    () => this.onPointerEnd());

    // Touch (mobile)
    this.canvas.addEventListener("touchstart", (e) => this.onTouchStart(e), { passive: true });
    this.canvas.addEventListener("touchmove",  (e) => this.onTouchMove(e),  { passive: true });
    this.canvas.addEventListener("touchend",   ()  => this.onPointerEnd());
    this.canvas.addEventListener("touchcancel",()  => this.onPointerEnd());

    // Keyboard
    window.addEventListener("keydown", (e) => this.onKeyDown(e));
    window.addEventListener("keyup",   (e) => this.onKeyUp(e));

    // Wheel is swallowed so the page never scrolls under the canvas, but it no
    // longer zooms — see the note on zoomBy().
    this.canvas.addEventListener("wheel", (e) => e.preventDefault(), { passive: false });

    // Explicit zoom buttons replace wheel zoom.
    document.getElementById("zoom-in-btn")
      ?.addEventListener("click", () => this.zoomBy(1));
    document.getElementById("zoom-out-btn")
      ?.addEventListener("click", () => this.zoomBy(-1));

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

  /**
   * Converts a client (CSS pixel) coordinate into world cell coordinates.
   *
   * The canvas backing store is `renderScale` times its displayed CSS size, and
   * `renderer.zoom` is expressed in BACKING-STORE pixels per cell. Feeding CSS
   * pixels straight into that conversion put every placement off by exactly the
   * render scale, which is the "things land in the wrong spot" bug: at 0.75
   * scale a click 400px from the left edge landed 533 cells in instead of 400.
   * Scaling by width ratio also absorbs any CSS transform on the canvas.
   */
  private screenToWorld(clientX: number, clientY: number): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return [0, 0];

    // CSS px → backing-store px.
    const scaleX = this.canvas.width / rect.width;
    const scaleY = this.canvas.height / rect.height;

    const sx = (clientX - rect.left) * scaleX;
    const sy = (clientY - rect.top) * scaleY;

    const zoom = this.renderer.zoom || 1;
    return [
      Math.floor(this.renderer.viewX + sx / zoom),
      Math.floor(this.renderer.viewY + sy / zoom),
    ];
  }

  /** Radius in cells that the current tool actually affects. */
  get effectiveRadius(): number {
    return this.brushRadius;
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

    // Forces use the same brush radius as the god tools. They used to be pinned
    // at 24 cells while the on-screen brush control said otherwise, so the ring
    // the player aimed with never matched the area that actually changed.
    const radius = this.brushRadius;
    this.prediction?.predict(this.network.activePower, wx, wy, radius);
    this.network.sendPower(this.network.activePower, wx, wy, radius);
    // Procedural sound effect. Throttled for the same reason as the shake: eight
    // triggers a second overlap into a buzz rather than reading as feedback.
    const now = Date.now();
    if (now - this.lastPowerSoundAt >= InputHandler.POWER_SOUND_INTERVAL_MS) {
      this.lastPowerSoundAt = now;
      AudioEngine.getInstance().playPower(this.network.activePower as 0 | 1 | 2 | 3);
    }
    // Screen shake is for the IMPACT of starting a large application, not for
    // every frame of holding one. Continuous painting fires this path about eight
    // times a second, and shaking on each one made the whole canvas vibrate
    // continuously and was genuinely unpleasant to use.
    if (radius > SHAKE_RADIUS_THRESHOLD && !this.isRepeat) {
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
    // The brush control stays up for every tool, forces included, because the
    // radius it sets is now the radius forces actually use. Hiding it on the
    // force bar made the size look like a god-tool-only setting.
    document.getElementById("brush-control")?.classList.add("visible");
  }

  /** Selects the material painted by the place tool. */
  setMaterial(material: number): void {
    this.activeMaterial = material;
  }

  /** Activates the place tool after a material is selected from the drawer. */
  selectPlaceTool(): void {
    this.selectTool("place");
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

  /** Index of the selected elemental force, for the brush ring tint. */
  get activePowerIndex(): number {
    return this.network.activePower;
  }

  get brush(): number {
    return this.brushRadius;
  }

  private onMouseDown(e: MouseEvent): void {
    if (e.button !== 0) return;
    this.applying = true;
    this.isRepeat = false; // fresh press: impact effects are wanted
    this.lastClientX = e.clientX;
    this.lastClientY = e.clientY;
    this.applyPowerAt(e.clientX, e.clientY);
    this.startRepeat();
  }

  private onMouseMove(e: MouseEvent): void {
    this.lastClientX = e.clientX;
    this.lastClientY = e.clientY;
    this.cursorClientX = e.clientX;
    this.cursorClientY = e.clientY;
    this.cursorOverCanvas = true;

    // Send cursor position for multiplayer presence (throttled)
    const now = Date.now();
    if (now - this.lastCursorSend > this.CURSOR_THROTTLE_MS) {
      const [wx, wy] = this.screenToWorld(e.clientX, e.clientY);
      this.network.sendCursor(wx, wy, this.network.activePower);
      this.lastCursorSend = now;
    }
    if (!this.applying) return;
    // A drag is a continuation, not a new impact — same reasoning as the repeat
    // timer. Shaking on every mousemove while painting was the single largest
    // contributor to the canvas vibrating.
    this.isRepeat = true;
    this.applyPowerAt(e.clientX, e.clientY);
  }

  private onMouseEnter(): void {
    this.cursorOverCanvas = true;
  }

  private onTouchStart(e: TouchEvent): void {
    if (e.touches.length === 1) {
      this.applying = true;
      this.lastClientX = e.touches[0].clientX;
      this.lastClientY = e.touches[0].clientY;
      this.cursorClientX = this.lastClientX;
      this.cursorClientY = this.lastClientY;
      this.cursorOverCanvas = true;
      this.applyPowerAt(this.lastClientX, this.lastClientY);
      this.startRepeat();
    }
  }

  private onTouchMove(e: TouchEvent): void {
    if (e.touches.length !== 1) return;
    this.lastClientX = e.touches[0].clientX;
    this.lastClientY = e.touches[0].clientY;
    this.cursorClientX = this.lastClientX;
    this.cursorClientY = this.lastClientY;
    if (!this.applying) return;
    this.applyPowerAt(this.lastClientX, this.lastClientY);
  }

  private onPointerEnd(): void {
    this.applying = false;
    this.stopRepeat();
  }

  /** Keeps applying at the held position until the button is released. */
  private startRepeat(): void {
    this.stopRepeat();
    this.repeatTimer = setInterval(() => {
      if (!this.applying) {
        this.stopRepeat();
        return;
      }
      // Marked as a repeat so impact effects (shake, sound) stay on the initial
      // press and do not fire eight times a second.
      this.isRepeat = true;
      this.applyPowerAt(this.lastClientX, this.lastClientY);
    }, InputHandler.REPEAT_INTERVAL_MS);
  }

  private stopRepeat(): void {
    if (this.repeatTimer !== null) {
      clearInterval(this.repeatTimer);
      this.repeatTimer = null;
    }
    this.isRepeat = false;
  }

  // ── Zoom ─────────────────────────────────────────────────────────────────
  //
  // Wheel and pinch zoom are deliberately NOT bound. Scrolling over the canvas
  // while aiming a brush changed the zoom under the cursor and threw the aim
  // off, and on a trackpad an ordinary two-finger scroll did it by accident.
  // Zoom is now an explicit act: the +/- keys or the on-screen buttons.

  /** Steps zoom and updates the indicator. Called by keys and by the buttons. */
  zoomBy(direction: 1 | -1): void {
    const newZoom = this.renderer.stepZoom(direction);
    this.showZoomIndicator(newZoom);
    this.network.sendViewport(
      this.renderer.viewX, this.renderer.viewY,
      this.renderer.visibleW, this.renderer.visibleH,
    );
  }

  private showZoomIndicator(zoom: number): void {
    if (!this.zoomIndicator) return;
    // Fractional zooms need a decimal or 0.5x and 0.25x both render as "0x".
    this.zoomIndicator.textContent = zoom < 1
      ? `${zoom.toFixed(2).replace(/0+$/, "").replace(/\.$/, "")}×`
      : `${Math.round(zoom * 10) / 10}×`;
    this.zoomIndicator.classList.add("visible");
    if (this.zoomHideTimeout) clearTimeout(this.zoomHideTimeout);
    this.zoomHideTimeout = setTimeout(() => {
      this.zoomIndicator?.classList.remove("visible");
    }, 1500);
  }

  // ── Keyboard ─────────────────────────────────────────────────────────────

  /** True when a text field owns the keyboard, so WASD types instead of pans. */
  private static isTypingTarget(t: EventTarget | null): boolean {
    const el = t as HTMLElement | null;
    if (!el || !el.tagName) return false;
    const tag = el.tagName.toLowerCase();
    return tag === "input" || tag === "textarea" || tag === "select" || el.isContentEditable;
  }

  private onKeyDown(e: KeyboardEvent): void {
    // Never steal keys from a text field — typing "sand" in chat used to pan the
    // camera and select tools at the same time.
    if (InputHandler.isTypingTarget(e.target)) return;
    // A modifier chord is a browser shortcut, not a game input.
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    // Pan keys are matched case-insensitively on e.code so they work with Caps
    // Lock on and while Shift is held. Previously only lowercase "a"/"d" were
    // registered, so A and D silently stopped panning under Caps Lock.
    const panCodes = new Set([
      "KeyW", "KeyA", "KeyS", "KeyD",
      "ArrowUp", "ArrowLeft", "ArrowDown", "ArrowRight",
    ]);
    if (panCodes.has(e.code)) {
      this.panKeys.add(e.code);
      // Arrows scroll the page otherwise, which fights the camera.
      e.preventDefault();
      return;
    }

    const key = e.key.toLowerCase();

    // Shortcuts overlay toggle
    if (e.key === "?") {
      this.toggleShortcuts();
      return;
    }

    // Minimap toggle
    if (key === "m") {
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
      p: "place",
      e: "erase",
      r: "raise",
      f: "lower",
    };
    if (key in toolKeys) {
      const tool = toolKeys[key];
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
      this.zoomBy(1);
      return;
    }
    if (e.key === "-" || e.key === "_") {
      this.zoomBy(-1);
      return;
    }
  }

  private onKeyUp(e: KeyboardEvent): void {
    this.panKeys.delete(e.code);
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
    // Determine input direction. A and D are left and right, W and S are up and
    // down, and the arrow keys mirror them.
    let inputX = 0;
    let inputY = 0;
    if (this.panKeys.has("ArrowLeft")  || this.panKeys.has("KeyA")) inputX -= 1;
    if (this.panKeys.has("ArrowRight") || this.panKeys.has("KeyD")) inputX += 1;
    if (this.panKeys.has("ArrowUp")    || this.panKeys.has("KeyW")) inputY -= 1;
    if (this.panKeys.has("ArrowDown")  || this.panKeys.has("KeyS")) inputY += 1;

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
  /**
   * Shakes the canvas once.
   *
   * The `shaking` flag alone was not enough: the animation is short, so a new
   * shake could start the instant the previous one ended, and at eight
   * applications a second that chained into continuous vibration. The cooldown is
   * a hard floor on how often the canvas is allowed to move at all.
   */
  triggerShake(): void {
    if (this.shaking || !this.canvasWrapper) return;

    const now = Date.now();
    if (now - this.lastShakeAt < InputHandler.SHAKE_COOLDOWN_MS) return;
    this.lastShakeAt = now;

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
