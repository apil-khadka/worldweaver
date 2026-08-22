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

import { WorldNetwork, MetricsData, PlayerState } from "./network.js";

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

function q(id: string): HTMLElement {
  return document.getElementById(id)!;
}

export class UIController {
  private rttInterval: ReturnType<typeof setInterval> | null = null;

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

    this.network.callbacks = {
      onConnected:    () => this.onConnected(),
      onDisconnected: () => this.onDisconnected(),
      onMetrics:      (m) => this.onMetrics(m),
      onPlayerState:  (s) => this.onPlayerState(s),
      onError:        (msg) => console.warn("[ui] server error:", msg),
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
}

function formatNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000)     return (n / 1_000).toFixed(1) + "k";
  return n.toString();
}
