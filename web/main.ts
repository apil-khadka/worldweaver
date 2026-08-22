/**
 * main.ts — WorldWeaver browser client entry point
 *
 * Responsibilities:
 *  - Show lobby for login + world selection
 *  - Bootstrap all subsystems (network, renderer, input, UI)
 *  - Wire up the data flow: network → renderer, input → network
 *  - This file should contain no game logic
 *
 * Architecture:
 *  Lobby handles login + world selection before connecting.
 *  Network receives world state from the server and exposes events.
 *  Renderer reads world state and draws to the canvas.
 *  Input captures user gestures and calls network.sendPower().
 *  UI reads metrics/stability from network and updates DOM elements.
 */

import { Lobby } from "./lobby.js";
import { WorldNetwork, type IGameRenderer } from "./network.js";
import { WorldRenderer, type FullSnapshot } from "./renderer.js";
import { WebGL2WorldRenderer, isWebGL2Available } from "./webgl2-renderer.js";
import { IsometricRenderer } from "./isometric-renderer.js";
import { InputHandler } from "./input.js";
import { UIController } from "./ui.js";
import { PowerEffects } from "./effects.js";
import { Minimap } from "./minimap.js";

// ── Lobby ──────────────────────────────────────────────────────────────────
const lobby = new Lobby();

async function main() {
  const selection = await lobby.show();
  lobby.hide();

  // Show the game UI
  document.getElementById("game-container")!.classList.add("visible");

  // ── Bootstrap game ─────────────────────────────────────────────────────
  const canvas = document.getElementById("world-canvas") as HTMLCanvasElement;
  const wrapper = document.getElementById("canvas-wrapper") as HTMLDivElement;
  const overlayCanvas = document.getElementById("power-overlay") as HTMLCanvasElement;

  // ── Create initial renderer (default: isometric 2.5D) ──────────────────
  const renderer = (() => {
    if (isWebGL2Available()) {
      try {
        const iso = new IsometricRenderer(canvas);
        console.info("[main] using Isometric 2.5D renderer (WebGL2)");
        return iso;
      } catch (e) {
        console.warn("[main] Isometric renderer failed, trying flat WebGL2:", e);
        try {
          const gl2 = new WebGL2WorldRenderer(canvas);
          console.info("[main] using WebGL2 renderer (GPU-accelerated)");
          return gl2;
        } catch (e2) {
          console.warn("[main] WebGL2 init failed, falling back to Canvas2D:", e2);
        }
      }
    }
    console.info("[main] using Canvas2D renderer (fallback)");
    return new WorldRenderer(canvas);
  })();

  // Track which renderer is active (can be swapped by the view toggle)
  let activeRenderer: IGameRenderer = renderer;

  // ── Wire up subsystems ─────────────────────────────────────────────────
  const wsUrl = buildWsUrl(selection.token, selection.worldID);
  const network  = new WorldNetwork(wsUrl, renderer);
  const ui       = new UIController(network);
  const input    = new InputHandler(canvas, network, renderer);
  const effects  = new PowerEffects(overlayCanvas, canvas, renderer);
  const minimapCanvas = document.getElementById("minimap") as HTMLCanvasElement;
  const minimap  = new Minimap(minimapCanvas, renderer, canvas);

  function resizeCanvas(): void {
    canvas.width  = wrapper.clientWidth;
    canvas.height = wrapper.clientHeight;
    overlayCanvas.width  = wrapper.clientWidth;
    overlayCanvas.height = wrapper.clientHeight;
    activeRenderer.onResize();
    effects.resize(wrapper.clientWidth, wrapper.clientHeight);
  }

  resizeCanvas();
  window.addEventListener("resize", resizeCanvas);

  network.connect();
  input.attach();
  ui.attach();
  effects.attach();
  minimap.start();

  // ── View mode toggle (2D ↔ 2.5D) ──────────────────────────────────────
  let viewMode: "2.5D" | "2D" = renderer instanceof IsometricRenderer ? "2.5D" : "2D";

  const viewToggle = document.getElementById("view-toggle") as HTMLButtonElement | null;
  if (viewToggle) {
    viewToggle.textContent = viewMode === "2.5D" ? "◆ 2.5D" : "▦ 2D";
    viewToggle.addEventListener("click", () => {
      if (viewMode === "2.5D") {
        // Switch to flat 2D
        viewMode = "2D";
        viewToggle.textContent = "▦ 2D";
        const flatRenderer = (() => {
          try { return new WebGL2WorldRenderer(canvas); }
          catch { return new WorldRenderer(canvas); }
        })();
        transferState(activeRenderer, flatRenderer);
        if ("dispose" in activeRenderer) (activeRenderer as any).dispose();
        activeRenderer = flatRenderer;
        network.swapRenderer(flatRenderer);
        input.swapRenderer(flatRenderer);
        minimap.swapRenderer(flatRenderer);
        resizeCanvas();
      } else {
        // Switch to isometric 2.5D
        viewMode = "2.5D";
        viewToggle.textContent = "◆ 2.5D";
        try {
          const isoRenderer = new IsometricRenderer(canvas);
          transferState(activeRenderer, isoRenderer);
          if ("dispose" in activeRenderer) (activeRenderer as any).dispose();
          activeRenderer = isoRenderer;
          network.swapRenderer(isoRenderer);
          input.swapRenderer(isoRenderer);
          minimap.swapRenderer(isoRenderer);
          resizeCanvas();
        } catch (e) {
          console.warn("[main] Cannot switch to isometric:", e);
          viewMode = "2D";
          viewToggle.textContent = "▦ 2D";
        }
      }
    });
  }

  // Show player nickname in header
  const nicknameEl = document.getElementById("hdr-nickname");
  if (nicknameEl) {
    nicknameEl.textContent = selection.nickname;
  }

  // Sync power selection to effects overlay
  document.querySelectorAll<HTMLButtonElement>(".power-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      const p = parseInt(btn.dataset["power"] ?? "0", 10);
      effects.setActivePower(p);
    });
  });
  window.addEventListener("keydown", (e) => {
    const powerMap: Record<string, number> = { "1": 0, "2": 1, "3": 2, "4": 3 };
    if (e.key in powerMap) {
      effects.setActivePower(powerMap[e.key]);
    }
    // V key toggles view mode
    if (e.key === "v" || e.key === "V") {
      viewToggle?.click();
    }
  });

  // Sync renderer viewport to canvas data attributes for cursor positioning
  setInterval(() => {
    canvas.dataset["viewX"] = activeRenderer.viewX.toString();
    canvas.dataset["viewY"] = activeRenderer.viewY.toString();
  }, 50);
}

/** Transfer world state and camera from one renderer to another. */
function transferState(from: IGameRenderer, to: IGameRenderer): void {
  to.initWorld(from.worldW, from.worldH);
  const cache = from.getMaterialCache();
  if (cache && from.worldW > 0) {
    to.applySnapshot({
      tick: 0,
      x: 0, y: 0,
      w: from.worldW,
      h: from.worldH,
      data: new Uint8Array(cache),
    });
  }
  to.viewX = from.viewX;
  to.viewY = from.viewY;
}

function buildWsUrl(token: string, worldID: string): string {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  let url = `${proto}://${location.host}/ws?world=${encodeURIComponent(worldID)}`;
  if (token) {
    url += `&token=${encodeURIComponent(token)}`;
  }
  return url;
}

main();
