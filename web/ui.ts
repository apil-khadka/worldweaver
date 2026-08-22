/**
 * ui.ts — DOM controller for HUD elements
 *
 * Updates:
 *  - Connection badge (header)
 *  - Stability % (header)
 *  - Player count (header)
 *  - Influence bar and value (power bar)
 *  - Footer metrics (TPS, p95, cells, active, chunks, net, RTT)
 *  - Status overlay (shown on disconnect)
 *
 * UI reads data exclusively from the WorldNetwork event callbacks.
 * It never touches world state directly.
 */

import { WorldNetwork, MetricsData, PlayerState, RemoteCursor } from "./network.js";

// Power colors for cursor rendering
const POWER_COLORS = ["#4da8ff", "#ff6b35", "#88cc44", "#44ddaa"];
const POWER_NAMES  = ["Rain", "Heat", "Wind", "Growth"];

// DOM references resolved once at attach time
let elBadge:      HTMLElement;
let elStability:  HTMLElement;
let elPlayers:    HTMLElement;
let elInfluenceFill:  HTMLElement;
let elInfluenceValue: HTMLElement;
let elStatusOverlay:  HTMLElement;
let elStatusTitle:    HTMLElement;
let elStatusBody:     HTMLElement;
let elFtTPS:    HTMLElement;
let elFtP95:    HTMLElement;
let elFtCells:  HTMLElement;
let elFtActive: HTMLElement;
let elFtChunks: HTMLElement;
let elFtNet:    HTMLElement;
let elFtRtt:    HTMLElement;
let elCanvasWrapper: HTMLElement;

function q(id: string): HTMLElement {
  return document.getElementById(id)!;
}

export class UIController {
  private rttInterval: ReturnType<typeof setInterval> | null = null;
  private cursors = new Map<number, { el: HTMLElement; timeout: ReturnType<typeof setTimeout> }>();

  constructor(private readonly network: WorldNetwork) {}

  attach(): void {
    elBadge      = q("connection-badge");
    elStability  = q("hdr-stability");
    elPlayers    = q("hdr-players");
    elInfluenceFill  = q("influence-fill");
    elInfluenceValue = q("influence-value");
    elStatusOverlay  = q("status-overlay");
    elStatusTitle    = q("status-title");
    elStatusBody     = q("status-body");
    elFtTPS    = q("ft-tps");
    elFtP95    = q("ft-p95");
    elFtCells  = q("ft-cells");
    elFtActive = q("ft-active");
    elFtChunks = q("ft-chunks");
    elFtNet    = q("ft-net");
    elFtRtt    = q("ft-rtt");
    elCanvasWrapper = q("canvas-wrapper");

    this.network.callbacks = {
      onConnected:    () => this.onConnected(),
      onDisconnected: () => this.onDisconnected(),
      onMetrics:      (m) => this.onMetrics(m),
      onPlayerState:  (s) => this.onPlayerState(s),
      onError:        (msg) => console.warn("[ui] server error:", msg),
      onCursorUpdate: (c) => this.onCursorUpdate(c),
      onPlayerJoin:   (id) => this.showToast(`Player ${id} joined the world`, "join"),
      onPlayerLeave:  (id) => this.removeCursor(id),
    };

    this.showStatus("Connecting to world…", "");

    // RTT display updates every second
    this.rttInterval = setInterval(() => {
      elFtRtt.textContent = this.network.lastRttMs.toFixed(0);
    }, 1000);
  }

  private onConnected(): void {
    elBadge.textContent = "🟢 Connected";
    elBadge.style.color = "";
    this.hideStatus();
  }

  private onDisconnected(): void {
    elBadge.textContent = "🔴 Disconnected";
    elBadge.style.color = "var(--danger)";
    this.showStatus("Connection lost", "Reconnecting…");
  }

  private onMetrics(m: MetricsData): void {
    elFtTPS.textContent    = m.tps.toFixed(1);
    elFtP95.textContent    = m.tickP95Ms.toFixed(1);
    elFtCells.textContent  = formatNum(m.activeCells);
    elFtActive.textContent = formatNum(m.activeCells);
    elFtChunks.textContent = m.activeChunks.toString();
    elFtNet.textContent    = (m.outboundBPS / 1024).toFixed(1);
    elPlayers.textContent  = m.playerCount.toString();

    const pct = Math.round(m.stability * 100);
    elStability.textContent = `${pct}%`;
    elStability.style.color =
      pct > 70 ? "var(--good)" :
      pct > 40 ? ""            :
                 "var(--danger)";
  }

  private onPlayerState(s: PlayerState): void {
    const pct = Math.min(100, (s.influence / s.maxInfluence) * 100);
    elInfluenceFill.style.width  = `${pct}%`;
    elInfluenceValue.textContent = s.influence.toFixed(0);
  }

  private showStatus(title: string, body: string): void {
    elStatusTitle.textContent = title;
    elStatusBody.textContent  = body;
    elStatusOverlay.classList.add("visible");
  }

  private hideStatus(): void {
    elStatusOverlay.classList.remove("visible");
  }

  // ── Multiplayer Cursors ──────────────────────────────────────────────────
  private onCursorUpdate(cursor: RemoteCursor): void {
    let entry = this.cursors.get(cursor.playerID);

    if (!entry) {
      // Create a new cursor element
      const el = document.createElement("div");
      el.className = "remote-cursor";
      el.innerHTML = `
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path d="M1 1L6 14L8 8L14 6L1 1Z" fill="currentColor" stroke="rgba(0,0,0,0.3)" stroke-width="0.5"/>
        </svg>
        <span class="cursor-label">P${cursor.playerID}</span>
      `;
      elCanvasWrapper.appendChild(el);
      entry = { el, timeout: setTimeout(() => this.removeCursor(cursor.playerID), 5000) };
      this.cursors.set(cursor.playerID, entry);
    }

    // Update position (world coords → screen coords)
    const canvas = document.getElementById("world-canvas") as HTMLCanvasElement;
    const sx = cursor.x - (this.network.worldW > 0 ? 0 : 0); // viewX is on renderer
    // We approximate screen position. The renderer's viewX/viewY offset is the viewport origin.
    // Since we can't directly access renderer here, we use a data attribute set by main.ts
    const viewX = parseInt(canvas.dataset["viewX"] ?? "0", 10);
    const viewY = parseInt(canvas.dataset["viewY"] ?? "0", 10);
    const screenX = cursor.x - viewX;
    const screenY = cursor.y - viewY;

    const color = POWER_COLORS[cursor.power] ?? POWER_COLORS[0];
    entry.el.style.left = `${screenX}px`;
    entry.el.style.top  = `${screenY}px`;
    entry.el.style.color = color;
    entry.el.style.display = (screenX >= 0 && screenY >= 0) ? "block" : "none";

    // Reset timeout
    clearTimeout(entry.timeout);
    entry.timeout = setTimeout(() => this.removeCursor(cursor.playerID), 5000);
  }

  private removeCursor(playerID: number): void {
    const entry = this.cursors.get(playerID);
    if (entry) {
      entry.el.remove();
      clearTimeout(entry.timeout);
      this.cursors.delete(playerID);
    }
    this.showToast(`Player ${playerID} left`, "leave");
  }

  // ── Toast Notifications ──────────────────────────────────────────────────
  private showToast(message: string, type: "join" | "leave"): void {
    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);
    // Trigger animation
    requestAnimationFrame(() => toast.classList.add("visible"));
    setTimeout(() => {
      toast.classList.remove("visible");
      setTimeout(() => toast.remove(), 300);
    }, 3000);
  }
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000)     return (n / 1_000).toFixed(1) + "k";
  return n.toString();
}
