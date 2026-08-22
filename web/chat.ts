/**
 * chat.ts — Multiplayer chat system
 *
 * Features:
 *  - Press Enter or T to focus the chat input (bottom-left)
 *  - Messages appear as floating bubbles near sender's cursor for 4s
 *  - Toggleable scrolling chat log (press C)
 *  - Max 50 characters per message
 */

import type { WorldNetwork, IGameRenderer } from "./network.js";

const MAX_CHAT_LENGTH = 50;
const BUBBLE_LIFETIME_MS = 4000;
const MAX_LOG_MESSAGES = 100;

interface ChatBubble {
  playerID: number;
  nickname: string;
  text: string;
  worldX: number;
  worldY: number;
  createdAt: number;
}

interface ChatLogEntry {
  playerID: number;
  nickname: string;
  text: string;
  time: string;
}

export class ChatSystem {
  private container: HTMLElement;
  private input: HTMLInputElement;
  private logPanel: HTMLElement;
  private logVisible = false;
  private bubbles: ChatBubble[] = [];
  private logMessages: ChatLogEntry[] = [];
  private overlayCanvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;

  constructor(
    private readonly network: WorldNetwork,
    private readonly renderer: IGameRenderer,
    gameContainer: HTMLElement,
  ) {
    // Create chat input container
    this.container = document.createElement("div");
    this.container.id = "chat-container";
    this.container.innerHTML = `
      <input id="chat-input" type="text" maxlength="${MAX_CHAT_LENGTH}"
             placeholder="Press Enter to chat..." autocomplete="off" />
      <span id="chat-charcount">0/${MAX_CHAT_LENGTH}</span>
    `;
    gameContainer.appendChild(this.container);

    this.input = document.getElementById("chat-input") as HTMLInputElement;

    // Create chat log panel
    this.logPanel = document.createElement("div");
    this.logPanel.id = "chat-log";
    this.logPanel.classList.add("hidden");
    gameContainer.appendChild(this.logPanel);

    // Create overlay canvas for bubbles
    this.overlayCanvas = document.createElement("canvas");
    this.overlayCanvas.id = "chat-bubbles-canvas";
    this.overlayCanvas.style.cssText = "position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:20;";
    gameContainer.appendChild(this.overlayCanvas);
    this.ctx = this.overlayCanvas.getContext("2d")!;

    this.setupStyles();
    this.setupEvents();
    this.startBubbleLoop();
  }

  private setupStyles(): void {
    const style = document.createElement("style");
    style.textContent = `
      #chat-container {
        position: absolute;
        bottom: 16px;
        left: 16px;
        z-index: 100;
        display: flex;
        align-items: center;
        gap: 6px;
      }
      #chat-input {
        width: 260px;
        padding: 6px 10px;
        border-radius: 6px;
        border: 1px solid rgba(255,255,255,0.2);
        background: rgba(0,0,0,0.6);
        color: #fff;
        font-size: 13px;
        outline: none;
        opacity: 0.5;
        transition: opacity 0.2s;
      }
      #chat-input:focus {
        opacity: 1;
        border-color: rgba(100,180,255,0.6);
      }
      #chat-charcount {
        color: rgba(255,255,255,0.4);
        font-size: 11px;
        font-family: monospace;
      }
      #chat-log {
        position: absolute;
        bottom: 52px;
        left: 16px;
        width: 280px;
        max-height: 200px;
        overflow-y: auto;
        background: rgba(0,0,0,0.5);
        border-radius: 6px;
        padding: 8px;
        z-index: 99;
        font-size: 12px;
        color: #ddd;
        scrollbar-width: thin;
      }
      #chat-log.hidden { display: none; }
      .chat-log-entry {
        margin-bottom: 3px;
        line-height: 1.3;
      }
      .chat-log-entry .nick {
        color: #7cb8ff;
        font-weight: bold;
      }
      .chat-log-entry .time {
        color: rgba(255,255,255,0.3);
        font-size: 10px;
        margin-right: 4px;
      }
    `;
    document.head.appendChild(style);
  }

  private setupEvents(): void {
    // Focus input on Enter or T (when not already focused)
    window.addEventListener("keydown", (e) => {
      if (document.activeElement === this.input) {
        if (e.key === "Escape") {
          this.input.blur();
          e.preventDefault();
        } else if (e.key === "Enter") {
          this.sendMessage();
          e.preventDefault();
        }
        return;
      }

      if (e.key === "Enter" || e.key === "t" || e.key === "T") {
        // Don't capture if user is typing elsewhere
        if (document.activeElement?.tagName === "INPUT" || document.activeElement?.tagName === "TEXTAREA") return;
        e.preventDefault();
        this.input.focus();
      }

      if (e.key === "c" || e.key === "C") {
        if (document.activeElement?.tagName === "INPUT" || document.activeElement?.tagName === "TEXTAREA") return;
        this.toggleLog();
      }
    });

    // Char counter
    this.input.addEventListener("input", () => {
      const counter = document.getElementById("chat-charcount");
      if (counter) counter.textContent = `${this.input.value.length}/${MAX_CHAT_LENGTH}`;
    });

    // Prevent game input while typing
    this.input.addEventListener("keydown", (e) => {
      e.stopPropagation();
    });
    this.input.addEventListener("keyup", (e) => {
      e.stopPropagation();
    });
  }

  private sendMessage(): void {
    const text = this.input.value.trim();
    if (!text) return;
    this.network.sendChat(text);
    this.input.value = "";
    const counter = document.getElementById("chat-charcount");
    if (counter) counter.textContent = `0/${MAX_CHAT_LENGTH}`;
    this.input.blur();
  }

  /** Called by main.ts when a chat broadcast arrives from the server. */
  onChatMessage(playerID: number, nickname: string, text: string, x: number, y: number): void {
    // Add bubble
    this.bubbles.push({
      playerID,
      nickname: nickname || `P${playerID}`,
      text,
      worldX: x,
      worldY: y,
      createdAt: Date.now(),
    });

    // Add to log
    const timeStr = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    this.logMessages.push({ playerID, nickname: nickname || `P${playerID}`, text, time: timeStr });
    if (this.logMessages.length > MAX_LOG_MESSAGES) {
      this.logMessages.shift();
    }
    this.renderLog();
  }

  private renderLog(): void {
    this.logPanel.innerHTML = this.logMessages
      .map((m) => `<div class="chat-log-entry"><span class="time">${m.time}</span><span class="nick">${m.nickname}:</span> ${this.escapeHtml(m.text)}</div>`)
      .join("");
    this.logPanel.scrollTop = this.logPanel.scrollHeight;
  }

  private escapeHtml(str: string): string {
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  private toggleLog(): void {
    this.logVisible = !this.logVisible;
    this.logPanel.classList.toggle("hidden", !this.logVisible);
  }

  private startBubbleLoop(): void {
    const draw = () => {
      this.drawBubbles();
      requestAnimationFrame(draw);
    };
    requestAnimationFrame(draw);
  }

  private drawBubbles(): void {
    // Resize canvas to match viewport
    const parent = this.overlayCanvas.parentElement!;
    if (this.overlayCanvas.width !== parent.clientWidth || this.overlayCanvas.height !== parent.clientHeight) {
      this.overlayCanvas.width = parent.clientWidth;
      this.overlayCanvas.height = parent.clientHeight;
    }

    this.ctx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);

    const now = Date.now();
    // Remove expired bubbles
    this.bubbles = this.bubbles.filter((b) => now - b.createdAt < BUBBLE_LIFETIME_MS);

    const zoom = this.renderer.zoom;
    for (const bubble of this.bubbles) {
      const age = now - bubble.createdAt;
      const alpha = Math.max(0, 1 - age / BUBBLE_LIFETIME_MS);

      // Convert world coords to screen coords
      const sx = (bubble.worldX - this.renderer.viewX) * zoom;
      const sy = (bubble.worldY - this.renderer.viewY) * zoom - 30; // above player

      if (sx < -200 || sx > this.overlayCanvas.width + 200 || sy < -50 || sy > this.overlayCanvas.height + 50) {
        continue;
      }

      this.ctx.save();
      this.ctx.globalAlpha = alpha;

      // Draw bubble background
      this.ctx.font = "12px system-ui, sans-serif";
      const textWidth = this.ctx.measureText(bubble.text).width;
      const padding = 8;
      const bw = textWidth + padding * 2;
      const bh = 22;
      const bx = sx - bw / 2;
      const by = sy - bh;

      this.ctx.fillStyle = "rgba(0,0,0,0.7)";
      this.ctx.beginPath();
      this.ctx.roundRect(bx, by, bw, bh, 6);
      this.ctx.fill();

      // Draw nickname label
      this.ctx.fillStyle = "#7cb8ff";
      this.ctx.font = "bold 9px system-ui, sans-serif";
      this.ctx.fillText(bubble.nickname, bx + padding, by - 3);

      // Draw text
      this.ctx.fillStyle = "#fff";
      this.ctx.font = "12px system-ui, sans-serif";
      this.ctx.fillText(bubble.text, bx + padding, by + 15);

      this.ctx.restore();
    }
  }

  /** Expose send function for use from the network module. */
  sendChat(text: string): void {
    this.network.sendChat(text.slice(0, MAX_CHAT_LENGTH));
  }
}
