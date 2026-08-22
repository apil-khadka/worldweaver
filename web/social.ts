/**
 * social.ts — Multiplayer social interactions
 *
 * Features:
 *  - Shift+Click: send a location ping (pulsing circle for 3s)
 *  - E key: open emoji picker (6 emotes), selected emote appears at cursor for all
 *  - Power combos: detect when 2+ players apply different powers within 32 cells / 500ms
 *    and display enhanced combo visual
 */

import type { WorldNetwork, IGameRenderer } from "./network.js";

const PING_DURATION_MS = 3000;
const EMOTES = ["👍", "🔥", "💧", "🌱", "⚠️", "❤️"] as const;
const COMBO_DURATION_MS = 2000;

interface LocationPing {
  playerID: number;
  worldX: number;
  worldY: number;
  createdAt: number;
}

interface EmoteDisplay {
  playerID: number;
  emote: string;
  worldX: number;
  worldY: number;
  createdAt: number;
}

interface ComboDisplay {
  playerIDs: number[];
  powers: number[];
  worldX: number;
  worldY: number;
  createdAt: number;
}

export class SocialSystem {
  private pings: LocationPing[] = [];
  private emotes: EmoteDisplay[] = [];
  private combos: ComboDisplay[] = [];
  private pickerVisible = false;
  private pickerEl: HTMLElement;
  private overlayCanvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;

  constructor(
    private readonly network: WorldNetwork,
    private readonly renderer: IGameRenderer,
    private readonly canvas: HTMLCanvasElement,
    gameContainer: HTMLElement,
  ) {
    // Create overlay canvas for pings/emotes/combos
    this.overlayCanvas = document.createElement("canvas");
    this.overlayCanvas.id = "social-overlay-canvas";
    this.overlayCanvas.style.cssText = "position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:19;";
    gameContainer.appendChild(this.overlayCanvas);
    this.ctx = this.overlayCanvas.getContext("2d")!;

    // Create emoji picker
    this.pickerEl = document.createElement("div");
    this.pickerEl.id = "emote-picker";
    this.pickerEl.classList.add("hidden");
    this.pickerEl.innerHTML = EMOTES.map(
      (e) => `<button class="emote-btn" data-emote="${e}">${e}</button>`
    ).join("");
    gameContainer.appendChild(this.pickerEl);

    this.setupStyles();
    this.setupEvents();
    this.startRenderLoop();
  }

  private setupStyles(): void {
    const style = document.createElement("style");
    style.textContent = `
      #emote-picker {
        position: absolute;
        bottom: 60px;
        left: 50%;
        transform: translateX(-50%);
        display: flex;
        gap: 4px;
        background: rgba(0,0,0,0.8);
        border: 1px solid rgba(255,255,255,0.2);
        border-radius: 8px;
        padding: 8px 12px;
        z-index: 110;
        transition: opacity 0.15s, transform 0.15s;
      }
      #emote-picker.hidden {
        display: none;
      }
      .emote-btn {
        width: 36px;
        height: 36px;
        font-size: 20px;
        border: none;
        background: rgba(255,255,255,0.1);
        border-radius: 6px;
        cursor: pointer;
        transition: background 0.1s, transform 0.1s;
      }
      .emote-btn:hover {
        background: rgba(255,255,255,0.25);
        transform: scale(1.2);
      }
    `;
    document.head.appendChild(style);
  }

  private setupEvents(): void {
    // Shift+Click for location ping
    this.canvas.addEventListener("mousedown", (e) => {
      if (e.shiftKey && e.button === 0) {
        e.preventDefault();
        e.stopPropagation();
        const [wx, wy] = this.screenToWorld(e.clientX, e.clientY);
        this.sendPing(wx, wy);
      }
    }, { capture: true });

    // E key for emote picker
    window.addEventListener("keydown", (e) => {
      if (document.activeElement?.tagName === "INPUT" || document.activeElement?.tagName === "TEXTAREA") return;
      if (e.key === "e" || e.key === "E") {
        e.preventDefault();
        this.togglePicker();
      }
    });

    // Emote button clicks
    this.pickerEl.addEventListener("click", (e) => {
      const btn = (e.target as HTMLElement).closest(".emote-btn") as HTMLElement | null;
      if (!btn) return;
      const emote = btn.dataset["emote"];
      if (emote) {
        this.sendEmote(emote);
        this.hidePicker();
      }
    });

    // Close picker on click outside
    document.addEventListener("mousedown", (e) => {
      if (this.pickerVisible && !this.pickerEl.contains(e.target as Node)) {
        this.hidePicker();
      }
    });
  }

  private screenToWorld(clientX: number, clientY: number): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    const sx = clientX - rect.left;
    const sy = clientY - rect.top;
    const zoom = this.renderer.zoom;
    return [
      Math.floor(this.renderer.viewX + sx / zoom),
      Math.floor(this.renderer.viewY + sy / zoom),
    ];
  }

  private sendPing(x: number, y: number): void {
    this.network.sendPingLocation(x, y);
  }

  private sendEmote(emote: string): void {
    this.network.sendEmote(emote);
  }

  private togglePicker(): void {
    this.pickerVisible = !this.pickerVisible;
    this.pickerEl.classList.toggle("hidden", !this.pickerVisible);
  }

  private hidePicker(): void {
    this.pickerVisible = false;
    this.pickerEl.classList.add("hidden");
  }

  // ---- Inbound event handlers (called from main.ts) ----

  onPingLocation(playerID: number, x: number, y: number): void {
    this.pings.push({ playerID, worldX: x, worldY: y, createdAt: Date.now() });
  }

  onEmote(playerID: number, emote: string, x: number, y: number): void {
    this.emotes.push({ playerID, emote, worldX: x, worldY: y, createdAt: Date.now() });
  }

  onCombo(playerIDs: number[], powers: number[], x: number, y: number): void {
    this.combos.push({ playerIDs, powers, worldX: x, worldY: y, createdAt: Date.now() });
  }

  // ---- Render loop ----

  private startRenderLoop(): void {
    const draw = () => {
      this.drawOverlay();
      requestAnimationFrame(draw);
    };
    requestAnimationFrame(draw);
  }

  private drawOverlay(): void {
    const parent = this.overlayCanvas.parentElement!;
    if (this.overlayCanvas.width !== parent.clientWidth || this.overlayCanvas.height !== parent.clientHeight) {
      this.overlayCanvas.width = parent.clientWidth;
      this.overlayCanvas.height = parent.clientHeight;
    }

    this.ctx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
    const now = Date.now();
    const zoom = this.renderer.zoom;

    // Draw pings
    this.pings = this.pings.filter((p) => now - p.createdAt < PING_DURATION_MS);
    for (const ping of this.pings) {
      const age = now - ping.createdAt;
      const progress = age / PING_DURATION_MS;
      const alpha = 1 - progress;
      const radius = 10 + progress * 30;

      const sx = (ping.worldX - this.renderer.viewX) * zoom;
      const sy = (ping.worldY - this.renderer.viewY) * zoom;

      if (sx < -50 || sx > this.overlayCanvas.width + 50 || sy < -50 || sy > this.overlayCanvas.height + 50) continue;

      this.ctx.save();
      this.ctx.globalAlpha = alpha;

      // Pulsing circle
      this.ctx.strokeStyle = "#ffcc00";
      this.ctx.lineWidth = 2.5;
      this.ctx.beginPath();
      this.ctx.arc(sx, sy, radius * zoom / 4, 0, Math.PI * 2);
      this.ctx.stroke();

      // Inner dot
      this.ctx.fillStyle = "#ffcc00";
      this.ctx.beginPath();
      this.ctx.arc(sx, sy, 3, 0, Math.PI * 2);
      this.ctx.fill();

      this.ctx.restore();
    }

    // Draw emotes
    this.emotes = this.emotes.filter((e) => now - e.createdAt < 2000);
    for (const emote of this.emotes) {
      const age = now - emote.createdAt;
      const alpha = Math.max(0, 1 - age / 2000);
      const floatY = -(age / 2000) * 20; // float upward

      const sx = (emote.worldX - this.renderer.viewX) * zoom;
      const sy = (emote.worldY - this.renderer.viewY) * zoom + floatY;

      if (sx < -50 || sx > this.overlayCanvas.width + 50 || sy < -50 || sy > this.overlayCanvas.height + 50) continue;

      this.ctx.save();
      this.ctx.globalAlpha = alpha;
      this.ctx.font = "24px system-ui, sans-serif";
      this.ctx.textAlign = "center";
      this.ctx.fillText(emote.emote, sx, sy);
      this.ctx.restore();
    }

    // Draw combos
    this.combos = this.combos.filter((c) => now - c.createdAt < COMBO_DURATION_MS);
    for (const combo of this.combos) {
      const age = now - combo.createdAt;
      const progress = age / COMBO_DURATION_MS;
      const alpha = 1 - progress;

      const sx = (combo.worldX - this.renderer.viewX) * zoom;
      const sy = (combo.worldY - this.renderer.viewY) * zoom;

      if (sx < -100 || sx > this.overlayCanvas.width + 100 || sy < -100 || sy > this.overlayCanvas.height + 100) continue;

      this.ctx.save();
      this.ctx.globalAlpha = alpha;

      // Big expanding ring
      const ringRadius = 20 + progress * 60;
      this.ctx.strokeStyle = "#ff6600";
      this.ctx.lineWidth = 3;
      this.ctx.shadowColor = "#ff6600";
      this.ctx.shadowBlur = 15;
      this.ctx.beginPath();
      this.ctx.arc(sx, sy, ringRadius, 0, Math.PI * 2);
      this.ctx.stroke();

      // Second inner ring
      this.ctx.strokeStyle = "#ffaa00";
      this.ctx.lineWidth = 2;
      this.ctx.beginPath();
      this.ctx.arc(sx, sy, ringRadius * 0.6, 0, Math.PI * 2);
      this.ctx.stroke();

      // "COMBO!" text
      this.ctx.shadowBlur = 0;
      this.ctx.fillStyle = "#fff";
      this.ctx.font = "bold 16px system-ui, sans-serif";
      this.ctx.textAlign = "center";
      this.ctx.fillText("COMBO!", sx, sy - ringRadius - 8);

      this.ctx.restore();
    }
  }
}
