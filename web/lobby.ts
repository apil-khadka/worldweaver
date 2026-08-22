/**
 * lobby.ts — WorldWeaver lobby system
 *
 * Shows a login + world selection UI before connecting to the game.
 * Stores nickname and token in localStorage for session persistence.
 */

export interface WorldInfo {
  id:          string;
  name:        string;
  seed:        number;
  width:       number;
  height:      number;
  creatorName: string;
  createdAt:   string;
  playerCount: number;
  maxPlayers:  number;
}

export interface LoginResult {
  token:    string;
  playerID: number;
  nickname: string;
}

export interface LobbySelection {
  token:   string;
  worldID: string;
  nickname: string;
}

const STORAGE_KEY_NICKNAME = "ww_nickname";
const STORAGE_KEY_TOKEN    = "ww_token";

export class Lobby {
  private overlay:      HTMLElement;
  private nicknameInput: HTMLInputElement;
  private worldList:    HTMLElement;
  private createBtn:    HTMLButtonElement;
  private createPanel:  HTMLElement;
  private statusEl:     HTMLElement;

  private token = "";
  private nickname = "";
  private resolve: ((sel: LobbySelection) => void) | null = null;

  constructor() {
    this.overlay       = document.getElementById("lobby-overlay")!;
    this.nicknameInput = document.getElementById("lobby-nickname") as HTMLInputElement;
    this.worldList     = document.getElementById("lobby-world-list")!;
    this.createBtn     = document.getElementById("lobby-create-btn") as HTMLButtonElement;
    this.createPanel   = document.getElementById("lobby-create-panel")!;
    this.statusEl      = document.getElementById("lobby-status")!;

    // Restore saved nickname
    const saved = localStorage.getItem(STORAGE_KEY_NICKNAME);
    if (saved) {
      this.nicknameInput.value = saved;
    }

    this.createBtn.addEventListener("click", () => this.toggleCreatePanel());
    document.getElementById("lobby-create-submit")!.addEventListener("click", () => this.createWorld());
  }

  /**
   * Shows the lobby and returns when the user picks a world.
   */
  show(): Promise<LobbySelection> {
    this.overlay.classList.add("visible");
    this.loadWorlds();

    return new Promise((resolve) => {
      this.resolve = resolve;
    });
  }

  hide(): void {
    this.overlay.classList.remove("visible");
  }

  private async login(): Promise<boolean> {
    const nickname = this.nicknameInput.value.trim() || "Anonymous";
    localStorage.setItem(STORAGE_KEY_NICKNAME, nickname);

    // Check for existing token
    const existingToken = localStorage.getItem(STORAGE_KEY_TOKEN);
    if (existingToken) {
      this.token = existingToken;
      this.nickname = nickname;
      return true;
    }

    try {
      const resp = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname }),
      });
      if (!resp.ok) {
        this.setStatus("Login failed", true);
        return false;
      }
      const data: LoginResult = await resp.json();
      this.token = data.token;
      this.nickname = data.nickname;
      localStorage.setItem(STORAGE_KEY_TOKEN, data.token);
      return true;
    } catch (e) {
      this.setStatus("Connection error", true);
      return false;
    }
  }

  private async loadWorlds(): Promise<void> {
    try {
      const resp = await fetch("/api/worlds");
      if (!resp.ok) {
        this.setStatus("Failed to load worlds", true);
        return;
      }
      const worlds: WorldInfo[] = await resp.json();
      this.renderWorldList(worlds);
    } catch {
      this.setStatus("Cannot reach server", true);
    }
  }

  private renderWorldList(worlds: WorldInfo[]): void {
    this.worldList.innerHTML = "";

    // Sort: genesis first, then by creation date
    worlds.sort((a, b) => {
      if (a.id === "genesis") return -1;
      if (b.id === "genesis") return 1;
      return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
    });

    for (const w of worlds) {
      const row = document.createElement("div");
      row.className = "lobby-world-row";
      row.innerHTML = `
        <div class="lobby-world-info">
          <span class="lobby-world-name">${this.escapeHtml(w.name)}</span>
          <span class="lobby-world-meta">${w.width}×${w.height} · seed ${w.seed} · by ${this.escapeHtml(w.creatorName)}</span>
        </div>
        <div class="lobby-world-players">${w.playerCount}/${w.maxPlayers} players</div>
        <button class="lobby-join-btn" data-world-id="${w.id}"${w.playerCount >= w.maxPlayers ? ' disabled' : ''}>
          ${w.playerCount >= w.maxPlayers ? 'Full' : 'Join'}
        </button>
      `;
      row.querySelector(".lobby-join-btn")!.addEventListener("click", () => this.joinWorld(w.id));
      this.worldList.appendChild(row);
    }
  }

  private async joinWorld(worldID: string): Promise<void> {
    this.setStatus("Logging in…", false);
    const ok = await this.login();
    if (!ok) return;

    this.setStatus("", false);
    this.resolve?.({
      token:   this.token,
      worldID,
      nickname: this.nickname,
    });
  }

  private toggleCreatePanel(): void {
    this.createPanel.classList.toggle("visible");
  }

  private async createWorld(): Promise<void> {
    const nameInput = document.getElementById("lobby-create-name") as HTMLInputElement;
    const seedInput = document.getElementById("lobby-create-seed") as HTMLInputElement;
    const maxPlayersInput = document.getElementById("lobby-create-maxplayers") as HTMLInputElement;
    const name = nameInput.value.trim();
    if (!name) {
      this.setStatus("World name required", true);
      return;
    }

    // Login first if not already
    const ok = await this.login();
    if (!ok) return;

    const seed = parseInt(seedInput.value) || Math.floor(Math.random() * 999999);
    const maxPlayers = Math.min(8, Math.max(1, parseInt(maxPlayersInput?.value) || 8));

    try {
      const resp = await fetch("/api/worlds", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": this.token,
        },
        body: JSON.stringify({ name, seed, maxPlayers }),
      });
      if (!resp.ok) {
        const err = await resp.json();
        this.setStatus(err.error || "Failed to create", true);
        return;
      }
      this.setStatus("World created!", false);
      this.createPanel.classList.remove("visible");
      nameInput.value = "";
      seedInput.value = "";
      if (maxPlayersInput) maxPlayersInput.value = "8";
      await this.loadWorlds();
    } catch {
      this.setStatus("Connection error", true);
    }
  }

  private setStatus(msg: string, isError: boolean): void {
    this.statusEl.textContent = msg;
    this.statusEl.className = isError ? "lobby-status error" : "lobby-status";
  }

  private escapeHtml(s: string): string {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
}
