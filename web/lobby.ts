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
const STORAGE_KEY_WORLD    = "ww_world";

/** Clears the stored session so the next load shows the lobby. */
export function clearStoredSession(): void {
  localStorage.removeItem(STORAGE_KEY_TOKEN);
  localStorage.removeItem(STORAGE_KEY_WORLD);
}

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
   * Resolves with the world to join.
   *
   * On a page refresh we try to resume the previous session so the player is
   * not sent back through the lobby every time. The lobby UI is only shown
   * when there is nothing to resume, the stored world is gone, or the player
   * explicitly asked for it via `?lobby=1`.
   */
  async show(): Promise<LobbySelection> {
    const params = new URLSearchParams(location.search);
    const forceLobby = params.has("lobby");

    // Direct join: /play?world=genesis skips the lobby entirely. Useful for
    // sharing a link straight into a specific world.
    const requested = params.get("world");
    if (requested && !forceLobby) {
      const nickname = params.get("nick")
        ?? localStorage.getItem(STORAGE_KEY_NICKNAME)
        ?? "Guest";
      if (await this.requestLogin(nickname)) {
        localStorage.setItem(STORAGE_KEY_WORLD, requested);
        return { token: this.token, worldID: requested, nickname: this.nickname };
      }
    }

    if (!forceLobby) {
      const resumed = await this.tryResume();
      if (resumed) return resumed;
    }

    this.overlay.classList.add("visible");
    this.loadWorlds();

    return new Promise((resolve) => {
      this.resolve = resolve;
    });
  }

  /**
   * Attempts to restore the previous session without user interaction.
   *
   * Auth sessions are held in server memory, so a server restart invalidates
   * stored tokens. In that case we silently re-login with the saved nickname
   * rather than interrupting the player.
   */
  private async tryResume(): Promise<LobbySelection | null> {
    const worldID  = localStorage.getItem(STORAGE_KEY_WORLD);
    const nickname = localStorage.getItem(STORAGE_KEY_NICKNAME);
    if (!worldID || !nickname) return null;

    // The world must still exist before we try to rejoin it.
    let worlds: WorldInfo[];
    try {
      const resp = await fetch("/api/worlds");
      if (!resp.ok) return null;
      worlds = await resp.json();
    } catch {
      return null;
    }
    const target = worlds.find((w) => w.id === worldID);
    if (!target || target.playerCount >= target.maxPlayers) return null;

    // Reuse the stored token when the server still recognises it.
    const stored = localStorage.getItem(STORAGE_KEY_TOKEN);
    if (stored) {
      try {
        const resp = await fetch("/api/session", {
          headers: { Authorization: stored },
        });
        if (resp.ok) {
          const data = await resp.json();
          this.token = stored;
          this.nickname = data.nickname ?? nickname;
          return { token: this.token, worldID, nickname: this.nickname };
        }
      } catch {
        return null;
      }
    }

    // Token missing or expired — re-authenticate quietly with the same name.
    if (!(await this.requestLogin(nickname))) return null;
    return { token: this.token, worldID, nickname: this.nickname };
  }

  hide(): void {
    this.overlay.classList.remove("visible");
  }

  private async login(): Promise<boolean> {
    const nickname = this.nicknameInput.value.trim() || "Anonymous";
    localStorage.setItem(STORAGE_KEY_NICKNAME, nickname);

    // Reuse the stored token only if the server still accepts it. Sessions are
    // in-memory server-side, so a restart makes old tokens unusable.
    const existingToken = localStorage.getItem(STORAGE_KEY_TOKEN);
    if (existingToken) {
      try {
        const resp = await fetch("/api/session", {
          headers: { Authorization: existingToken },
        });
        if (resp.ok) {
          this.token = existingToken;
          this.nickname = nickname;
          return true;
        }
      } catch {
        this.setStatus("Connection error", true);
        return false;
      }
      localStorage.removeItem(STORAGE_KEY_TOKEN);
    }

    return this.requestLogin(nickname);
  }

  /** Requests a fresh token for the given nickname. */
  private async requestLogin(nickname: string): Promise<boolean> {
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
      localStorage.setItem(STORAGE_KEY_NICKNAME, data.nickname);
      return true;
    } catch {
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

    // Remember the world so a refresh can rejoin it without the lobby.
    localStorage.setItem(STORAGE_KEY_WORLD, worldID);

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
