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
import { WorldRenderer } from "./renderer.js";
import { WebGL2WorldRenderer, isWebGL2Available } from "./webgl2-renderer.js";
import { InputHandler } from "./input.js";
import { UIController } from "./ui.js";
import { PowerEffects } from "./effects.js";
import { Minimap } from "./minimap.js";
import { AudioEngine } from "./audio.js";
import { ChatSystem } from "./chat.js";
import { SocialSystem } from "./social.js";

// ── Lobby ──────────────────────────────────────────────────────────────────
const lobby = new Lobby();

async function main() {
  const selection = await lobby.show();
  lobby.hide();

  // Show loading overlay while connecting
  const loadingOverlay = document.getElementById("loading-overlay");
  if (loadingOverlay) loadingOverlay.classList.add("visible");

  // Show the game UI
  document.getElementById("game-container")!.classList.add("visible");

  // ── Bootstrap game ─────────────────────────────────────────────────────
  const canvas = document.getElementById("world-canvas") as HTMLCanvasElement;
  const wrapper = document.getElementById("canvas-wrapper") as HTMLDivElement;
  const overlayCanvas = document.getElementById("power-overlay") as HTMLCanvasElement;

  // ── Create renderer ───────────────────────────────────────────────────────
  const renderer = createRenderer(canvas);
  const activeRenderer: IGameRenderer = renderer;

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

  // Hide loading overlay on connection
  const origOnConnected = network.callbacks.onConnected;
  network.callbacks.onConnected = () => {
    origOnConnected?.();
    const lo = document.getElementById("loading-overlay");
    if (lo) lo.classList.remove("visible");
  };

  // ── Social systems (chat, pings, emotes, combos) ─────────────────────────
  const gameContainerEl = document.getElementById("game-container")!;
  const chat = new ChatSystem(network, renderer, gameContainerEl);
  const social = new SocialSystem(network, renderer, canvas, gameContainerEl);

  // Wire social network callbacks
  network.callbacks.onChat = (playerID, nickname, text, x, y) => {
    chat.onChatMessage(playerID, nickname, text, x, y);
  };
  network.callbacks.onPingLocation = (playerID, x, y) => {
    social.onPingLocation(playerID, x, y);
  };
  network.callbacks.onEmote = (playerID, emote, x, y) => {
    social.onEmote(playerID, emote, x, y);
  };
  network.callbacks.onCombo = (playerIDs, powers, x, y) => {
    social.onCombo(playerIDs, powers, x, y);
  };

  // ── Audio Engine ─────────────────────────────────────────────────────────
  const audio = AudioEngine.getInstance();
  // Expose for UI level-up sound
  (window as any).__wwAudio = audio;

  // Init audio on first user interaction (browser autoplay policy)
  const initAudio = () => {
    audio.init();
    document.removeEventListener("click", initAudio);
    document.removeEventListener("keydown", initAudio);
  };
  document.addEventListener("click", initAudio);
  document.addEventListener("keydown", initAudio);

  // Mute toggle button
  const muteBtn = document.createElement("button");
  muteBtn.id = "audio-mute";
  muteBtn.className = "hdr-btn";
  muteBtn.textContent = "🔊";
  muteBtn.title = "Toggle sound effects";
  muteBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    const muted = audio.toggleMute();
    muteBtn.textContent = muted ? "🔇" : "🔊";
  });
  // Insert into header controls
  const headerRight = document.querySelector(".header-right, .hdr-controls");
  if (headerRight) {
    headerRight.prepend(muteBtn);
  } else {
    document.querySelector("header")?.appendChild(muteBtn);
  }

  // Wire player join/leave sounds into network callbacks
  const origCallbacks = network.callbacks;
  const origOnJoin = origCallbacks.onPlayerJoin;
  const origOnLeave = origCallbacks.onPlayerLeave;
  network.callbacks.onPlayerJoin = (id) => {
    origOnJoin?.(id);
    audio.playJoin();
  };
  network.callbacks.onPlayerLeave = (id) => {
    origOnLeave?.(id);
    audio.playLeave();
  };

  // Track score changes for ding sound
  let lastScore = 0;
  const origOnPlayerState = origCallbacks.onPlayerState;
  network.callbacks.onPlayerState = (s) => {
    origOnPlayerState?.(s);
    // Trigger ding on significant influence gain (refill)
    if (s.influence > lastScore + 10) {
      audio.playScoreDing();
    }
    lastScore = s.influence;
  };

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
      audio.playPower(p as 0 | 1 | 2 | 3);
    });
  });
  window.addEventListener("keydown", (e) => {
    const powerMap: Record<string, number> = { "1": 0, "2": 1, "3": 2, "4": 3 };
    if (e.key in powerMap) {
      effects.setActivePower(powerMap[e.key]);
    }
  });

  // Sync renderer viewport to canvas data attributes for cursor positioning
  setInterval(() => {
    canvas.dataset["viewX"] = activeRenderer.viewX.toString();
    canvas.dataset["viewY"] = activeRenderer.viewY.toString();
  }, 50);

  // Ambient sound update: scan material cache for fire/water ratios (~4Hz)
  setInterval(() => {
    const cache = activeRenderer.getMaterialCache();
    if (!cache || cache.length === 0) return;

    let fireCount = 0;
    let waterCount = 0;
    let emissive = 0;
    // Sample every 16th cell for performance
    const step = 16;
    const total = cache.length / step;
    for (let i = 0; i < cache.length; i += step) {
      const mat = cache[i];
      if (mat === 6 || mat === 13) fireCount++;  // Fire or Ember
      if (mat === 4) waterCount++;               // Water
      if (mat === 6 || mat === 13 || mat === 9) emissive++; // + Lava
    }

    audio.updateAmbient(fireCount / total, waterCount / total);

    // Let the renderer skip its light-bleed pass when nothing is burning. That
    // pass samples the material texture eight times per fragment, so switching
    // it off is a large saving on a world with no fire in view.
    if ("setHasEmissive" in activeRenderer) {
      (activeRenderer as { setHasEmissive(v: boolean): void })
        .setHasEmissive(emissive > 0);
    }
  }, 250);
}

/**
 * Builds the world renderer.
 *
 * WebGL2 is the only real target; Canvas2D exists purely so the game still
 * displays on hardware without it.
 */
function createRenderer(canvas: HTMLCanvasElement): IGameRenderer {
  if (isWebGL2Available()) {
    try {
      const gl2 = new WebGL2WorldRenderer(canvas);
      console.info("[main] using WebGL2 renderer");
      return gl2;
    } catch (e) {
      console.warn("[main] WebGL2 init failed, falling back to Canvas2D:", e);
    }
  }
  console.info("[main] using Canvas2D renderer (fallback)");
  return new WorldRenderer(canvas);
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
