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
import { WorldNetwork } from "./network.js";
import { WorldRenderer } from "./renderer.js";
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

  function resizeCanvas(): void {
    canvas.width  = wrapper.clientWidth;
    canvas.height = wrapper.clientHeight;
    overlayCanvas.width  = wrapper.clientWidth;
    overlayCanvas.height = wrapper.clientHeight;
    effects.resize(wrapper.clientWidth, wrapper.clientHeight);
  }

  const renderer = new WorldRenderer(canvas);
  const wsUrl = buildWsUrl(selection.token, selection.worldID);
  const network  = new WorldNetwork(wsUrl, renderer);
  const ui       = new UIController(network);
  const input    = new InputHandler(canvas, network, renderer);
  const effects  = new PowerEffects(overlayCanvas, canvas, renderer);
  const minimapCanvas = document.getElementById("minimap") as HTMLCanvasElement;
  const minimap  = new Minimap(minimapCanvas, renderer, canvas);

  resizeCanvas();
  window.addEventListener("resize", resizeCanvas);

  network.connect();
  input.attach();
  ui.attach();
  effects.attach();
  minimap.start();

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
  });

  // Sync renderer viewport to canvas data attributes for cursor positioning
  setInterval(() => {
    canvas.dataset["viewX"] = renderer.viewX.toString();
    canvas.dataset["viewY"] = renderer.viewY.toString();
  }, 50);
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
