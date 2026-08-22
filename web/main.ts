/**
 * main.ts — WorldWeaver browser client entry point
 *
 * Responsibilities:
 *  - Bootstrap all subsystems (network, renderer, input, UI)
 *  - Wire up the data flow: network → renderer, input → network
 *  - This file should contain no game logic
 *
 * Architecture:
 *  Network receives world state from the server and exposes events.
 *  Renderer reads world state and draws to the canvas.
 *  Input captures user gestures and calls network.sendPower().
 *  UI reads metrics/stability from network and updates DOM elements.
 */

import { WorldNetwork } from "./network.js";
import { WorldRenderer } from "./renderer.js";
import { InputHandler } from "./input.js";
import { UIController } from "./ui.js";

const canvas = document.getElementById("world-canvas") as HTMLCanvasElement;
const wrapper = document.getElementById("canvas-wrapper") as HTMLDivElement;

// ── Size canvas to wrapper ─────────────────────────────────────────────────
function resizeCanvas(): void {
  canvas.width  = wrapper.clientWidth;
  canvas.height = wrapper.clientHeight;
}
resizeCanvas();
window.addEventListener("resize", resizeCanvas);

// ── Bootstrap ──────────────────────────────────────────────────────────────
const renderer = new WorldRenderer(canvas);
const network  = new WorldNetwork(buildWsUrl(), renderer);
const ui       = new UIController(network);
const input    = new InputHandler(canvas, network, renderer);

network.connect();
input.attach();
ui.attach();

function buildWsUrl(): string {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}/ws`;
}
