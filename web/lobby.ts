/**
 * lobby.ts — sign-in and world selection.
 *
 * The lobby does three things: prove the player's identity with their keypair,
 * show the worlds they are allowed to see, and let them open a world of their own
 * as public, unlisted or private.
 *
 * Sign-in has no password field. identity.ts holds an Ed25519 keypair for this
 * device and signs a server challenge; the name typed here is a label attached to
 * that key, not a credential. The previous lobby sent a nickname and received a
 * token in return, which meant any player could present themselves as any other.
 *
 * Collaboration is the point of the invite panel: sharing a world should be one
 * code you hand to a friend, not an account they have to be granted.
 */

import {
  authHeader,
  exportIdentity,
  forgetIdentity,
  importIdentity,
  loadIdentity,
  resumeSession,
  signIn,
  signOut,
  UnsupportedCryptoError,
  type Session,
} from "./identity.js";

export type Visibility = "public" | "unlisted" | "private";

export interface WorldInfo {
  id: string;
  name: string;
  seed: number;
  width: number;
  height: number;
  creatorName: string;
  createdAt: string;
  playerCount: number;
  maxPlayers: number;
  size?: string;

  /** Set by the server per caller: whether this world is theirs. */
  owned?: boolean;
  visibility?: Visibility;
}

export interface WorldSizePreset {
  Name: string;
  Width: number;
  Height: number;
  Description: string;
}

export interface LobbySelection {
  token: string;
  worldID: string;
  nickname: string;
}

const STORAGE_KEY_NICKNAME = "ww_nickname";
const STORAGE_KEY_TOKEN = "ww_token";
const STORAGE_KEY_WORLD = "ww_world";

/** Clears the stored session so the next load shows the lobby. */
export function clearStoredSession(): void {
  localStorage.removeItem(STORAGE_KEY_TOKEN);
  localStorage.removeItem(STORAGE_KEY_WORLD);
}

/**
 * Signs out properly: the server drops the session, then local state goes.
 *
 * Clearing localStorage alone left the token valid server-side until it expired,
 * so "log out" did not actually end the session.
 */
export async function endSession(): Promise<void> {
  const token = localStorage.getItem(STORAGE_KEY_TOKEN);
  if (token) await signOut(token);
  clearStoredSession();
}

export class Lobby {
  private overlay: HTMLElement;
  private nicknameInput: HTMLInputElement;
  private worldList: HTMLElement;
  private createBtn: HTMLButtonElement;
  private createPanel: HTMLElement;
  private statusEl: HTMLElement;
  private identityEl: HTMLElement | null;
  private sizeSelect: HTMLSelectElement | null;

  private session: Session | null = null;
  private resolve: ((sel: LobbySelection) => void) | null = null;

  constructor() {
    this.overlay = document.getElementById("lobby-overlay")!;
    this.nicknameInput = document.getElementById("lobby-nickname") as HTMLInputElement;
    this.worldList = document.getElementById("lobby-world-list")!;
    this.createBtn = document.getElementById("lobby-create-btn") as HTMLButtonElement;
    this.createPanel = document.getElementById("lobby-create-panel")!;
    this.statusEl = document.getElementById("lobby-status")!;
    this.identityEl = document.getElementById("lobby-identity");
    this.sizeSelect = document.getElementById("lobby-create-size") as HTMLSelectElement | null;

    const saved = localStorage.getItem(STORAGE_KEY_NICKNAME);
    if (saved) this.nicknameInput.value = saved;

    this.createBtn.addEventListener("click", () => this.toggleCreatePanel());
    document
      .getElementById("lobby-create-submit")!
      .addEventListener("click", () => this.createWorld());

    document
      .getElementById("lobby-redeem-btn")
      ?.addEventListener("click", () => this.redeemInvite());

    document
      .getElementById("lobby-identity-export")
      ?.addEventListener("click", () => this.showIdentityKey());
    document
      .getElementById("lobby-identity-import")
      ?.addEventListener("click", () => this.importIdentityKey());
  }

  /**
   * Resolves with the world to join.
   *
   * A refresh resumes the previous session rather than sending the player back
   * through the lobby. The lobby appears when there is nothing to resume, the
   * stored world is gone, or `?lobby=1` asks for it.
   */
  async show(): Promise<LobbySelection> {
    const params = new URLSearchParams(location.search);
    const forceLobby = params.has("lobby");

    // An invite link (/play?invite=CODE) redeems before anything else, so the
    // code puts the player straight into the world it belongs to.
    const inviteCode = params.get("invite");

    if (!forceLobby || inviteCode) {
      if (inviteCode) {
        const joined = await this.joinByCode(inviteCode);
        if (joined) return joined;
      }

      // Direct join: /play?world=<id> skips the lobby, which is what makes a
      // shared link to an unlisted world work.
      const requested = params.get("world");
      if (requested) {
        const nickname =
          params.get("nick") ?? localStorage.getItem(STORAGE_KEY_NICKNAME) ?? "Guest";
        if (await this.authenticate(nickname)) {
          localStorage.setItem(STORAGE_KEY_WORLD, requested);
          return this.selection(requested);
        }
      }

      const resumed = await this.tryResume();
      if (resumed) return resumed;
    }

    this.overlay.classList.add("visible");
    void this.showIdentity();
    void this.loadSizes();
    void this.loadWorlds();

    return new Promise((resolve) => {
      this.resolve = resolve;
    });
  }

  hide(): void {
    this.overlay.classList.remove("visible");
  }

  // ── Identity ──────────────────────────────────────────────────────────────

  /** Shows the key fingerprint, so a player can tell which identity is loaded. */
  private async showIdentity(): Promise<void> {
    if (!this.identityEl) return;
    try {
      const id = await loadIdentity();
      this.identityEl.textContent = `Signing as key ${id.fingerprint}`;
    } catch (err) {
      this.identityEl.textContent =
        err instanceof UnsupportedCryptoError ? err.message : "No identity available";
    }
  }

  /**
   * Reveals the private key so the player can keep a copy.
   *
   * Deliberately behind an explicit action with a warning: this string *is* the
   * identity, and there is no way to reissue it if it is lost or leaked.
   */
  private showIdentityKey(): void {
    const box = document.getElementById("lobby-identity-key") as HTMLTextAreaElement | null;
    if (!box) return;
    const blob = exportIdentity();
    if (!blob) {
      this.setStatus("No identity to export yet", true);
      return;
    }
    box.value = blob;
    box.classList.add("visible");
    box.select();
    this.setStatus("This is your identity. Anyone who has it is you — store it somewhere safe.", false);
  }

  /** Restores an identity from a previously exported key. */
  private async importIdentityKey(): Promise<void> {
    const box = document.getElementById("lobby-identity-key") as HTMLTextAreaElement | null;
    if (!box) return;

    if (!box.classList.contains("visible")) {
      box.value = "";
      box.classList.add("visible");
      box.placeholder = "Paste an identity key, then press Restore again";
      box.focus();
      return;
    }

    const blob = box.value.trim();
    if (!blob) {
      this.setStatus("Paste your identity key first", true);
      return;
    }

    try {
      const id = await importIdentity(blob);
      // The restored key is a different player, so the old session is void.
      clearStoredSession();
      this.session = null;
      box.value = "";
      box.classList.remove("visible");
      this.setStatus(`Restored key ${id.fingerprint}`, false);
      await this.showIdentity();
      await this.loadWorlds();
    } catch (err) {
      this.setStatus(err instanceof Error ? err.message : "Could not restore that key", true);
    }
  }

  /** Abandons this device's identity. Used by the "new identity" control. */
  async resetIdentity(): Promise<void> {
    await endSession();
    forgetIdentity();
    this.session = null;
    await this.showIdentity();
    await this.loadWorlds();
  }

  // ── Session ───────────────────────────────────────────────────────────────

  /**
   * Ensures there is a live session, signing in with the keypair if needed.
   *
   * A stored token is checked before signing again: the handshake is two round
   * trips, and there is no reason to repeat it while the old token still works.
   */
  private async authenticate(nickname?: string): Promise<boolean> {
    if (this.session) return true;

    const name =
      nickname ?? this.nicknameInput?.value.trim() ?? localStorage.getItem(STORAGE_KEY_NICKNAME) ?? "";

    const stored = localStorage.getItem(STORAGE_KEY_TOKEN);
    if (stored) {
      const resumed = await resumeSession(stored);
      if (resumed) {
        this.session = resumed;
        localStorage.setItem(STORAGE_KEY_NICKNAME, resumed.nickname);
        return true;
      }
      localStorage.removeItem(STORAGE_KEY_TOKEN);
    }

    try {
      this.session = await signIn(name || "Anonymous");
      localStorage.setItem(STORAGE_KEY_TOKEN, this.session.token);
      localStorage.setItem(STORAGE_KEY_NICKNAME, this.session.nickname);
      return true;
    } catch (err) {
      this.setStatus(err instanceof Error ? err.message : "Sign-in failed", true);
      return false;
    }
  }

  /** Restores the previous session and world without user interaction. */
  private async tryResume(): Promise<LobbySelection | null> {
    const worldID = localStorage.getItem(STORAGE_KEY_WORLD);
    if (!worldID) return null;
    if (!(await this.authenticate())) return null;

    // The world must still exist, and still have room, before rejoining it. A
    // world hidden from this player is not in the listing at all, which is the
    // same outcome as it being gone.
    const worlds = await this.fetchWorlds();
    const target = worlds?.find((w) => w.id === worldID);
    if (!target || target.playerCount >= target.maxPlayers) return null;

    return this.selection(worldID);
  }

  private selection(worldID: string): LobbySelection {
    return {
      token: this.session?.token ?? "",
      worldID,
      nickname: this.session?.nickname ?? "Anonymous",
    };
  }

  private authHeaders(): Record<string, string> {
    return {
      "Content-Type": "application/json",
      ...(this.session ? authHeader(this.session.token) : {}),
    };
  }

  // ── Worlds ────────────────────────────────────────────────────────────────

  /** Fetches the worlds this player may see. */
  private async fetchWorlds(): Promise<WorldInfo[] | null> {
    try {
      const resp = await fetch("/api/worlds", {
        headers: this.session ? authHeader(this.session.token) : {},
      });
      if (!resp.ok) return null;
      return (await resp.json()) as WorldInfo[];
    } catch {
      return null;
    }
  }

  private async loadWorlds(): Promise<void> {
    // Signing in first matters: the listing is filtered per identity, so an
    // anonymous request would omit the player's own private worlds.
    await this.authenticate();

    const worlds = await this.fetchWorlds();
    if (!worlds) {
      this.setStatus("Cannot reach server", true);
      return;
    }
    this.renderWorldList(worlds);
  }

  /** Loads the size presets from the server so the lobby cannot disagree with it. */
  private async loadSizes(): Promise<void> {
    if (!this.sizeSelect || this.sizeSelect.options.length > 0) return;
    try {
      const resp = await fetch("/api/world-sizes");
      if (!resp.ok) return;
      const presets = (await resp.json()) as WorldSizePreset[];
      for (const p of presets) {
        const opt = document.createElement("option");
        opt.value = p.Name;
        opt.textContent = `${p.Name} — ${p.Width}×${p.Height}`;
        opt.title = p.Description;
        if (p.Name === "medium") opt.selected = true;
        this.sizeSelect.appendChild(opt);
      }
    } catch {
      // Sizes are a convenience; the server defaults to medium without them.
    }
  }

  private renderWorldList(worlds: WorldInfo[]): void {
    this.worldList.textContent = "";

    worlds.sort((a, b) => {
      if (a.id === "genesis") return -1;
      if (b.id === "genesis") return 1;
      return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
    });

    if (worlds.length === 0) {
      const empty = document.createElement("div");
      empty.className = "lobby-world-meta";
      empty.textContent = "No worlds yet — create the first one.";
      this.worldList.appendChild(empty);
      return;
    }

    for (const w of worlds) {
      this.worldList.appendChild(this.worldRow(w));
    }
  }

  /**
   * Builds one row of the world list.
   *
   * Built with DOM calls rather than an HTML string: world names and creator
   * names come from players, and interpolating them into innerHTML is an
   * injection waiting to happen.
   */
  private worldRow(w: WorldInfo): HTMLElement {
    const row = document.createElement("div");
    row.className = "lobby-world-row";

    const info = document.createElement("div");
    info.className = "lobby-world-info";

    const name = document.createElement("span");
    name.className = "lobby-world-name";
    name.textContent = w.name;
    if (w.visibility && w.visibility !== "public") {
      const badge = document.createElement("span");
      badge.className = `lobby-badge lobby-badge-${w.visibility}`;
      badge.textContent = w.visibility;
      name.appendChild(document.createTextNode(" "));
      name.appendChild(badge);
    }
    info.appendChild(name);

    const meta = document.createElement("span");
    meta.className = "lobby-world-meta";
    meta.textContent = `${w.width}×${w.height} · seed ${w.seed} · by ${w.creatorName}`;
    info.appendChild(meta);
    row.appendChild(info);

    const players = document.createElement("div");
    players.className = "lobby-world-players";
    players.textContent = `${w.playerCount}/${w.maxPlayers} players`;
    row.appendChild(players);

    const full = w.playerCount >= w.maxPlayers;
    const join = document.createElement("button");
    join.className = "lobby-join-btn";
    join.textContent = full ? "Full" : "Join";
    join.disabled = full;
    join.addEventListener("click", () => this.joinWorld(w.id));
    row.appendChild(join);

    // Owner controls live on the row so sharing a world is one click from seeing
    // it, rather than a separate management screen.
    if (w.owned) {
      const invite = document.createElement("button");
      invite.className = "lobby-row-btn";
      invite.textContent = "Invite";
      invite.title = "Create a code that lets a friend in";
      invite.addEventListener("click", () => this.createInvite(w));
      row.appendChild(invite);

      const cycle = document.createElement("button");
      cycle.className = "lobby-row-btn";
      cycle.textContent = w.visibility ?? "public";
      cycle.title = "Change who can find this world";
      cycle.addEventListener("click", () => this.cycleVisibility(w));
      row.appendChild(cycle);
    }

    return row;
  }

  private async joinWorld(worldID: string): Promise<void> {
    this.setStatus("Signing in…", false);
    if (!(await this.authenticate())) return;

    // Remember the world so a refresh rejoins it without the lobby.
    localStorage.setItem(STORAGE_KEY_WORLD, worldID);
    this.setStatus("", false);
    this.resolve?.(this.selection(worldID));
  }

  private toggleCreatePanel(): void {
    this.createPanel.classList.toggle("visible");
  }

  private async createWorld(): Promise<void> {
    const nameInput = document.getElementById("lobby-create-name") as HTMLInputElement;
    const seedInput = document.getElementById("lobby-create-seed") as HTMLInputElement;
    const maxPlayersInput = document.getElementById(
      "lobby-create-maxplayers",
    ) as HTMLInputElement | null;
    const visSelect = document.getElementById(
      "lobby-create-visibility",
    ) as HTMLSelectElement | null;

    const name = nameInput.value.trim();
    if (!name) {
      this.setStatus("World name required", true);
      return;
    }
    if (!(await this.authenticate())) return;

    const seed = parseInt(seedInput.value) || Math.floor(Math.random() * 999999);
    const maxPlayers = Math.min(8, Math.max(1, parseInt(maxPlayersInput?.value ?? "") || 8));

    try {
      const resp = await fetch("/api/worlds", {
        method: "POST",
        headers: this.authHeaders(),
        body: JSON.stringify({
          name,
          seed,
          maxPlayers,
          size: this.sizeSelect?.value ?? "medium",
          visibility: visSelect?.value ?? "public",
        }),
      });
      if (!resp.ok) {
        const err = (await resp.json()) as { error?: string };
        this.setStatus(err.error || "Failed to create", true);
        return;
      }
      this.setStatus("World created", false);
      this.createPanel.classList.remove("visible");
      nameInput.value = "";
      seedInput.value = "";
      await this.loadWorlds();
    } catch {
      this.setStatus("Connection error", true);
    }
  }

  // ── Invites ───────────────────────────────────────────────────────────────

  /**
   * Mints an invite code for a world and puts a shareable link on the clipboard.
   *
   * The link carries the code rather than the world ID, so the recipient becomes
   * a member on arrival instead of bouncing off a private world.
   */
  private async createInvite(w: WorldInfo): Promise<void> {
    if (!(await this.authenticate())) return;
    try {
      const resp = await fetch("/api/invites", {
        method: "POST",
        headers: this.authHeaders(),
        body: JSON.stringify({ worldId: w.id, maxUses: 0, expiresIn: "168h" }),
      });
      if (!resp.ok) {
        const err = (await resp.json()) as { error?: string };
        this.setStatus(err.error || "Could not create an invite", true);
        return;
      }
      const inv = (await resp.json()) as { code: string };
      const link = `${location.origin}/play?invite=${inv.code}`;

      try {
        await navigator.clipboard.writeText(link);
        this.setStatus(`Invite ${inv.code} copied — share the link to bring someone in`, false);
      } catch {
        // Clipboard access is denied in plenty of contexts; showing the code is
        // the part that matters.
        this.setStatus(`Invite code ${inv.code}`, false);
      }
    } catch {
      this.setStatus("Connection error", true);
    }
  }

  /** Redeems the code typed into the lobby, then joins the world behind it. */
  private async redeemInvite(): Promise<void> {
    const input = document.getElementById("lobby-invite-code") as HTMLInputElement | null;
    const code = input?.value.trim();
    if (!code) {
      this.setStatus("Enter an invite code", true);
      return;
    }
    const joined = await this.joinByCode(code);
    if (joined) {
      this.resolve?.(joined);
      return;
    }
    await this.loadWorlds();
  }

  /**
   * Redeems a code and returns the resulting selection.
   *
   * Returns null on any failure so callers can fall back to the normal lobby
   * rather than leaving the player on a dead end.
   */
  private async joinByCode(code: string): Promise<LobbySelection | null> {
    if (!(await this.authenticate())) return null;
    try {
      const resp = await fetch("/api/invites/redeem", {
        method: "POST",
        headers: this.authHeaders(),
        body: JSON.stringify({ code }),
      });
      if (!resp.ok) {
        const err = (await resp.json()) as { error?: string };
        this.setStatus(err.error || "That invite code is not valid", true);
        return null;
      }
      const data = (await resp.json()) as { worldId: string };
      localStorage.setItem(STORAGE_KEY_WORLD, data.worldId);
      return this.selection(data.worldId);
    } catch {
      this.setStatus("Connection error", true);
      return null;
    }
  }

  /**
   * Steps a world through public → unlisted → private.
   *
   * A three-way cycle rather than a dropdown per row: the row already carries the
   * current state as its label, and one control is less to explain.
   */
  private async cycleVisibility(w: WorldInfo): Promise<void> {
    const order: Visibility[] = ["public", "unlisted", "private"];
    const next = order[(order.indexOf(w.visibility ?? "public") + 1) % order.length];

    if (!(await this.authenticate())) return;
    try {
      const resp = await fetch(`/api/worlds/${encodeURIComponent(w.id)}/visibility`, {
        method: "PUT",
        headers: this.authHeaders(),
        body: JSON.stringify({ visibility: next }),
      });
      if (!resp.ok) {
        const err = (await resp.json()) as { error?: string };
        this.setStatus(err.error || "Could not change visibility", true);
        return;
      }
      this.setStatus(`${w.name} is now ${next}`, false);
      await this.loadWorlds();
    } catch {
      this.setStatus("Connection error", true);
    }
  }

  private setStatus(msg: string, isError: boolean): void {
    this.statusEl.textContent = msg;
    this.statusEl.className = isError ? "lobby-status error" : "lobby-status";
  }
}
