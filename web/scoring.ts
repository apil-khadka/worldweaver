/**
 * scoring.ts — Leaderboard and score display for WorldWeaver
 *
 * Responsibilities:
 *  - Fetch top 10 scores from /api/scores every 5 seconds when visible
 *  - Render the leaderboard panel overlay
 *  - Show the current player's score in the header
 *  - Toggle visibility via the 🏆 button
 */

export interface ScoreEntry {
  playerID: number;
  influenceSpent: number;
  cellsAffected: number;
  creaturesSpawned: number;
  firesStarted: number;
  waterCreated: number;
  stabilityContribution: number;
  playTime: number;
  score: number;
}

interface ScoresResponse {
  world: string;
  scores: ScoreEntry[] | null;
}

export class ScoreboardUI {
  private panel: HTMLElement;
  private toggleBtn: HTMLElement;
  private scoreDisplay: HTMLElement;
  private listEl: HTMLElement;
  private visible = false;
  private pollInterval: ReturnType<typeof setInterval> | null = null;
  private myPlayerID = 0;
  private worldName = "genesis";

  constructor() {
    // Create the toggle button
    this.toggleBtn = document.createElement("button");
    this.toggleBtn.id = "leaderboard-toggle";
    this.toggleBtn.textContent = "🏆";
    this.toggleBtn.title = "Toggle Leaderboard";
    this.toggleBtn.addEventListener("click", () => this.toggle());

    // Create the inline score display for header
    this.scoreDisplay = document.createElement("span");
    this.scoreDisplay.id = "my-score";
    this.scoreDisplay.innerHTML = "Score: <b>0</b>";

    // Create the leaderboard panel
    this.panel = document.createElement("div");
    this.panel.id = "leaderboard-panel";
    this.panel.innerHTML = `
      <div class="lb-header">
        <span class="lb-title">🏆 Leaderboard</span>
        <button class="lb-close" title="Close">&times;</button>
      </div>
      <div class="lb-list"></div>
    `;
    this.panel.querySelector(".lb-close")!.addEventListener("click", () => this.hide());
    this.listEl = this.panel.querySelector(".lb-list")!;
  }

  /** Attach the scoreboard UI to the DOM. Must be called after DOMContentLoaded. */
  attach(playerID: number): void {
    this.myPlayerID = playerID;

    // Insert toggle button and score display into header
    const headerMeta = document.querySelector("#header .meta")!;
    headerMeta.insertBefore(this.scoreDisplay, headerMeta.firstChild);
    headerMeta.insertBefore(this.toggleBtn, headerMeta.firstChild);

    // Insert panel into the canvas wrapper
    const wrapper = document.getElementById("canvas-wrapper")!;
    wrapper.appendChild(this.panel);

    // Inject styles
    this.injectStyles();
  }

  /** Update the known player ID (called after welcome message). */
  setPlayerID(id: number): void {
    this.myPlayerID = id;
  }

  toggle(): void {
    if (this.visible) {
      this.hide();
    } else {
      this.show();
    }
  }

  show(): void {
    this.visible = true;
    this.panel.classList.add("visible");
    this.toggleBtn.classList.add("active");
    this.fetchScores();
    this.pollInterval = setInterval(() => this.fetchScores(), 5000);
  }

  hide(): void {
    this.visible = false;
    this.panel.classList.remove("visible");
    this.toggleBtn.classList.remove("active");
    if (this.pollInterval) {
      clearInterval(this.pollInterval);
      this.pollInterval = null;
    }
  }

  private async fetchScores(): Promise<void> {
    try {
      const resp = await fetch(`/api/scores?world=${encodeURIComponent(this.worldName)}`);
      if (!resp.ok) return;
      const data: ScoresResponse = await resp.json();
      this.renderScores(data.scores ?? []);
    } catch {
      // Network error — silently ignore, will retry
    }
  }

  private renderScores(scores: ScoreEntry[]): void {
    // Update inline header score
    const myScore = scores.find(s => s.playerID === this.myPlayerID);
    const scoreEl = this.scoreDisplay.querySelector("b")!;
    scoreEl.textContent = myScore ? myScore.score.toString() : "0";

    // Render the full list
    if (scores.length === 0) {
      this.listEl.innerHTML = '<div class="lb-empty">No scores yet — use your powers!</div>';
      return;
    }

    let html = "";
    scores.forEach((entry, i) => {
      const isMe = entry.playerID === this.myPlayerID;
      const medal = i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : `${i + 1}.`;
      html += `
        <div class="lb-row${isMe ? " lb-me" : ""}">
          <span class="lb-rank">${medal}</span>
          <span class="lb-name">Player ${entry.playerID}</span>
          <span class="lb-score">${entry.score.toLocaleString()}</span>
        </div>
      `;
    });
    this.listEl.innerHTML = html;
  }

  private injectStyles(): void {
    const style = document.createElement("style");
    style.textContent = `
      #leaderboard-toggle {
        padding: 4px 10px;
        border: 1px solid var(--border-highlight);
        border-radius: 6px;
        background: rgba(255, 255, 255, 0.03);
        color: var(--text);
        cursor: pointer;
        font-size: 14px;
        transition: all 0.2s ease;
      }
      #leaderboard-toggle:hover {
        background: rgba(255, 255, 255, 0.06);
        transform: scale(1.05);
      }
      #leaderboard-toggle.active {
        background: rgba(255, 215, 0, 0.1);
        border-color: rgba(255, 215, 0, 0.4);
        box-shadow: 0 0 10px rgba(255, 215, 0, 0.2);
      }

      #my-score {
        color: var(--muted);
        font-size: 12px;
        font-weight: 500;
      }
      #my-score b {
        color: #ffd700;
        font-family: 'JetBrains Mono', 'SF Mono', monospace;
        font-weight: 700;
      }

      #leaderboard-panel {
        position: absolute;
        top: 10px;
        right: 10px;
        width: 280px;
        max-height: 400px;
        background: rgba(14, 17, 24, 0.92);
        backdrop-filter: blur(16px);
        -webkit-backdrop-filter: blur(16px);
        border: 1px solid var(--border-highlight);
        border-radius: 12px;
        z-index: 200;
        display: none;
        flex-direction: column;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
        overflow: hidden;
      }
      #leaderboard-panel.visible {
        display: flex;
      }

      .lb-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 14px;
        border-bottom: 1px solid var(--border);
      }
      .lb-title {
        font-size: 13px;
        font-weight: 700;
        color: #ffffff;
      }
      .lb-close {
        background: none;
        border: none;
        color: var(--muted);
        font-size: 18px;
        cursor: pointer;
        padding: 0 4px;
        line-height: 1;
      }
      .lb-close:hover {
        color: var(--text);
      }

      .lb-list {
        overflow-y: auto;
        padding: 8px 0;
      }

      .lb-row {
        display: flex;
        align-items: center;
        padding: 8px 14px;
        gap: 10px;
        transition: background 0.15s;
      }
      .lb-row:hover {
        background: rgba(255, 255, 255, 0.03);
      }
      .lb-row.lb-me {
        background: rgba(77, 168, 255, 0.08);
        border-left: 3px solid var(--accent);
      }

      .lb-rank {
        font-size: 13px;
        min-width: 28px;
      }
      .lb-name {
        flex: 1;
        font-size: 12px;
        font-weight: 500;
        color: var(--text);
      }
      .lb-me .lb-name {
        color: #ffffff;
        font-weight: 700;
      }
      .lb-score {
        font-size: 12px;
        font-weight: 700;
        color: #ffd700;
        font-family: 'JetBrains Mono', 'SF Mono', monospace;
      }

      .lb-empty {
        padding: 20px 14px;
        text-align: center;
        color: var(--muted);
        font-size: 12px;
      }
    `;
    document.head.appendChild(style);
  }
}
