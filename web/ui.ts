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

// Power colors, shown as a small badge on each remote cursor.
const POWER_COLORS = ["#4da8ff", "#ff6b35", "#88cc44", "#44ddaa", "#c77dff"];

/**
 * A stable, distinct colour per player.
 *
 * Cursors are coloured by identity rather than by the power being held, so two
 * players wielding the same force are still tellable apart. Hues are spread by
 * an irrational-ish step so nearby player IDs do not land on similar colours.
 */
function playerColor(playerID: number): string {
  const hue = (playerID * 137.508) % 360;
  return `hsl(${hue.toFixed(0)}, 72%, 55%)`;
}
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

  // ── Smooth number animation state ──────────────────────────────────────
  private displayedTPS = 0;
  private displayedP95 = 0;
  private displayedCells = 0;
  private displayedActive = 0;
  private displayedChunks = 0;
  private displayedNet = 0;
  private targetTPS = 0;
  private targetP95 = 0;
  private targetCells = 0;
  private targetActive = 0;
  private targetChunks = 0;
  private targetNet = 0;
  private lerpAnimFrame: number | null = null;

  // ── Influence warning state ────────────────────────────────────────────
  private elInfluenceLowText: HTMLElement | null = null;
  private elInfluenceTrack: HTMLElement | null = null;
  private influenceLow = false;

  // ── Level progression state ────────────────────────────────────────────
  private currentLevel = 1;
  private elLevelValue: HTMLElement | null = null;
  private elXpFill: HTMLElement | null = null;
  private elXpText: HTMLElement | null = null;
  private elLevelupOverlay: HTMLElement | null = null;
  private elLevelupText: HTMLElement | null = null;

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

    this.elInfluenceLowText = document.getElementById("influence-low-text");
    this.elInfluenceTrack = document.getElementById("influence-track");

    // Level progression elements
    this.elLevelValue = document.querySelector("#hdr-level b");
    this.elXpFill = document.getElementById("xp-bar-fill");
    this.elXpText = document.getElementById("xp-bar-text");
    this.elLevelupOverlay = document.getElementById("levelup-overlay");
    this.elLevelupText = document.getElementById("levelup-text");

    // Initialize locked power button states
    this.updateLockedPowers(1);

    this.network.callbacks = {
      onConnected:    () => this.onConnected(),
      onWorldReady:   () => this.onWorldReady(),
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

    // Start smooth number animation loop
    this.startLerpLoop();
  }

  private onConnected(): void {
    elBadge.textContent = "🟢 Connected";
    elBadge.style.color = "";
    this.showStatus("Loading world…", "Receiving the initial world state");
  }

  private onWorldReady(): void {
    this.hideStatus();
  }

  private onDisconnected(): void {
    elBadge.textContent = "🔴 Disconnected";
    elBadge.style.color = "var(--danger)";
    this.showStatus("Connection lost", "Reconnecting…");
  }

  private onMetrics(m: MetricsData): void {
    // Set lerp targets — DOM is updated in the animation frame
    this.targetTPS    = m.tps;
    this.targetP95    = m.tickP95Ms;
    this.targetCells  = m.activeCells;
    this.targetActive = m.activeCells;
    this.targetChunks = m.activeChunks;
    this.targetNet    = m.outboundBPS / 1024;

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

    // Influence warning at < 20%
    const isLow = pct < 20;
    if (isLow !== this.influenceLow) {
      this.influenceLow = isLow;
      if (isLow) {
        this.elInfluenceTrack?.classList.add("low-influence");
        elInfluenceFill.classList.add("low-gradient");
        this.elInfluenceLowText?.classList.add("visible");
      } else {
        this.elInfluenceTrack?.classList.remove("low-influence");
        elInfluenceFill.classList.remove("low-gradient");
        this.elInfluenceLowText?.classList.remove("visible");
      }
    }

    // Level & XP bar
    if (this.elLevelValue) {
      this.elLevelValue.textContent = s.level.toString();
    }
    if (this.elXpFill && this.elXpText) {
      if (s.nextLevelScore > 0) {
        // Calculate XP progress within current level
        const prevLevelScore = this.getLevelThreshold(s.level - 1);
        const progress = s.score - prevLevelScore;
        const needed = s.nextLevelScore - prevLevelScore;
        const xpPct = Math.min(100, Math.max(0, (progress / needed) * 100));
        this.elXpFill.style.width = `${xpPct}%`;
        this.elXpText.textContent = `${s.score} / ${s.nextLevelScore}`;
      } else {
        // Max level
        this.elXpFill.style.width = "100%";
        this.elXpText.textContent = `${s.score} ★ MAX`;
      }
    }

    // Level up detection
    if (s.level > this.currentLevel && this.currentLevel > 0) {
      this.showLevelUp(s.level);
      this.updateLockedPowers(s.level);
    }
    this.currentLevel = s.level;
  }

  // Level score thresholds (mirrors server LevelThresholds)
  private getLevelThreshold(level: number): number {
    const thresholds = [0, 0, 100, 500, 2000, 10000];
    return thresholds[level] ?? 0;
  }

  private showLevelUp(newLevel: number): void {
    if (!this.elLevelupOverlay || !this.elLevelupText) return;

    const unlockText: Record<number, string> = {
      2: "Larger power radius unlocked!",
      3: "Faster influence regen!",
      4: "🐾 Life power unlocked!",
      5: "Max influence increased to 150!",
    };

    this.elLevelupText.textContent = `Level ${newLevel} — ${unlockText[newLevel] ?? ""}`;
    this.elLevelupOverlay.classList.add("visible");

    // Play level-up sound via AudioEngine if available
    try {
      const audio = (window as any).__wwAudio;
      if (audio?.playLevelUp) audio.playLevelUp();
    } catch { /* ignore */ }

    setTimeout(() => {
      this.elLevelupOverlay?.classList.remove("visible");
    }, 2200);
  }

  private updateLockedPowers(level: number): void {
    const buttons = document.querySelectorAll<HTMLButtonElement>(".power-btn[data-unlock-level]");
    buttons.forEach(btn => {
      const requiredLevel = parseInt(btn.dataset["unlockLevel"] ?? "99", 10);
      if (level >= requiredLevel) {
        btn.classList.remove("power-locked");
        btn.removeAttribute("data-lock-text");
        // Brief unlock glow
        btn.classList.add("power-unlocked");
        setTimeout(() => btn.classList.remove("power-unlocked"), 1500);
      } else {
        btn.classList.add("power-locked");
        btn.setAttribute("data-lock-text", `Lv. ${requiredLevel}`);
      }
    });
  }

  // ── Smooth Number Animation Loop ──────────────────────────────────────
  private startLerpLoop(): void {
    const LERP_FACTOR = 0.1;
    const tick = () => {
      this.displayedTPS    += (this.targetTPS - this.displayedTPS) * LERP_FACTOR;
      this.displayedP95    += (this.targetP95 - this.displayedP95) * LERP_FACTOR;
      this.displayedCells  += (this.targetCells - this.displayedCells) * LERP_FACTOR;
      this.displayedActive += (this.targetActive - this.displayedActive) * LERP_FACTOR;
      this.displayedChunks += (this.targetChunks - this.displayedChunks) * LERP_FACTOR;
      this.displayedNet    += (this.targetNet - this.displayedNet) * LERP_FACTOR;

      elFtTPS.textContent    = this.displayedTPS.toFixed(1);
      elFtP95.textContent    = this.displayedP95.toFixed(1);
      elFtCells.textContent  = formatNum(Math.round(this.displayedCells));
      elFtActive.textContent = formatNum(Math.round(this.displayedActive));
      elFtChunks.textContent = Math.round(this.displayedChunks).toString();
      elFtNet.textContent    = this.displayedNet.toFixed(1);

      this.lerpAnimFrame = requestAnimationFrame(tick);
    };
    this.lerpAnimFrame = requestAnimationFrame(tick);
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
      // Built without interpolating the nickname: it is player-supplied, so it
      // is assigned via textContent rather than innerHTML.
      el.innerHTML = `
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <path d="M1 1L6 14L8 8L14 6L1 1Z" fill="currentColor" stroke="rgba(0,0,0,0.3)" stroke-width="0.5"/>
        </svg>
        <span class="cursor-label">
          <span class="cursor-power"></span>
          <span class="cursor-name"></span>
        </span>
      `;
      const nameEl = el.querySelector<HTMLElement>(".cursor-name")!;
      nameEl.textContent = cursor.nickname || `Player ${cursor.playerID}`;
      elCanvasWrapper.appendChild(el);
      entry = { el, timeout: setTimeout(() => this.removeCursor(cursor.playerID), 5000) };
      this.cursors.set(cursor.playerID, entry);
    }

    // World coordinates to CSS pixels. Zoom is canvas pixels per cell, and the
    // canvas backing store is renderScale times its displayed size, so CSS
    // pixels per cell is zoom / renderScale. Omitting zoom put every cursor in
    // the wrong place at any zoom level other than 1.
    const canvas = document.getElementById("world-canvas") as HTMLCanvasElement;
    const viewX = parseFloat(canvas.dataset["viewX"] ?? "0");
    const viewY = parseFloat(canvas.dataset["viewY"] ?? "0");
    const zoom = parseFloat(canvas.dataset["zoom"] ?? "1") || 1;
    const scale = parseFloat(canvas.dataset["renderScale"] ?? "1") || 1;
    const cssPerCell = zoom / scale;

    const screenX = (cursor.x - viewX) * cssPerCell;
    const screenY = (cursor.y - viewY) * cssPerCell;

    entry.el.style.left = `${screenX}px`;
    entry.el.style.top = `${screenY}px`;

    // Colour identifies the player; the badge shows what they are holding. Using
    // the power for colour made two players wielding the same force identical.
    entry.el.style.color = playerColor(cursor.playerID);

    const badge = entry.el.querySelector<HTMLElement>(".cursor-power");
    if (badge) badge.style.background = POWER_COLORS[cursor.power] ?? POWER_COLORS[0];

    if (cursor.nickname) {
      const nameEl = entry.el.querySelector<HTMLElement>(".cursor-name");
      if (nameEl && nameEl.textContent !== cursor.nickname) {
        nameEl.textContent = cursor.nickname;
      }
    }

    // Hide once off-screen rather than letting it pile up at the edge.
    const withinX = screenX >= 0 && screenX <= canvas.clientWidth;
    const withinY = screenY >= 0 && screenY <= canvas.clientHeight;
    entry.el.style.display = withinX && withinY ? "block" : "none";

    // An idle cursor is stale, not gone: hide it, but do not announce a
    // departure the server never reported.
    clearTimeout(entry.timeout);
    entry.timeout = setTimeout(() => this.hideCursor(cursor.playerID), 5000);
  }

  /** Hides a cursor that has stopped reporting, keeping the element for reuse. */
  private hideCursor(playerID: number): void {
    const entry = this.cursors.get(playerID);
    if (entry) entry.el.style.display = "none";
  }

  /** Removes a cursor for a player who has actually disconnected. */
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
