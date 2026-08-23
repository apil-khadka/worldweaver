/**
 * settings.ts — in-game settings panel, session menu and logout
 *
 * Preferences are persisted to localStorage and applied on load, so a refresh
 * keeps the player's choices. Everything here is presentation only: nothing in
 * this module can change authoritative world state.
 */

import { AudioEngine } from "./audio.js";
import { endSession } from "./lobby.js";

const STORAGE_KEY = "ww_settings";

export interface Settings {
  volume: number;      // 0..1
  muted: boolean;
  showMinimap: boolean;
  showMetrics: boolean;
  showCursors: boolean;
  /** Scales the render resolution. Below 1 trades sharpness for frame rate. */
  renderScale: number;
}

const DEFAULTS: Settings = {
  volume: 0.8,
  muted: false,
  showMinimap: true,
  showMetrics: true,
  showCursors: true,
  renderScale: 1,
};

/** Loads persisted settings, falling back to defaults for anything missing. */
export function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...DEFAULTS };
    return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<Settings>) };
  } catch {
    // Corrupt or unavailable storage should never stop the game loading.
    return { ...DEFAULTS };
  }
}

function saveSettings(s: Settings): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
  } catch {
    // Private browsing can refuse writes; the session still works in memory.
  }
}

export class SettingsPanel {
  private settings: Settings;
  private overlay: HTMLElement | null = null;

  /** Called when renderScale changes, so the canvas can be resized. */
  onRenderScaleChange: ((scale: number) => void) | null = null;

  constructor(private readonly nickname: string, private readonly worldName: string) {
    this.settings = loadSettings();
  }

  get current(): Settings {
    return this.settings;
  }

  /** Applies every persisted preference to the live page. */
  applyAll(): void {
    const audio = AudioEngine.getInstance();
    audio.setVolume(this.settings.volume);
    if (this.settings.muted) audio.setVolume(0);

    this.applyVisibility("minimap", this.settings.showMinimap);
    this.applyVisibility("footer", this.settings.showMetrics);
    applyCursorVisibility(this.settings.showCursors);
    this.onRenderScaleChange?.(this.settings.renderScale);
  }

  private applyVisibility(id: string, visible: boolean): void {
    const el = document.getElementById(id);
    if (el) el.style.display = visible ? "" : "none";
  }

  private update<K extends keyof Settings>(key: K, value: Settings[K]): void {
    this.settings[key] = value;
    saveSettings(this.settings);
  }

  /** Builds the panel markup once and wires its controls. */
  attach(): void {
    if (this.overlay) return;

    const overlay = document.createElement("div");
    overlay.id = "settings-overlay";
    overlay.innerHTML = `
      <div class="settings-card" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <div class="settings-head">
          <h3 id="settings-title">Settings</h3>
          <button class="settings-close" aria-label="Close settings">&times;</button>
        </div>

        <div class="settings-session">
          <span>Signed in as <b>${escapeHtml(this.nickname)}</b></span>
          <span class="settings-world">${escapeHtml(this.worldName)}</span>
        </div>

        <div class="settings-group">
          <label for="set-volume">Volume</label>
          <input type="range" id="set-volume" min="0" max="100" step="1"
                 value="${Math.round(this.settings.volume * 100)}"
                 aria-describedby="set-volume-value" />
          <output id="set-volume-value">${Math.round(this.settings.volume * 100)}%</output>
        </div>

        <div class="settings-group">
          <label for="set-scale">Render quality</label>
          <select id="set-scale">
            <option value="0.5">Low (fastest)</option>
            <option value="0.75">Medium</option>
            <option value="1">Full</option>
          </select>
        </div>

        <div class="settings-toggles">
          <label><input type="checkbox" id="set-minimap" /> Show minimap</label>
          <label><input type="checkbox" id="set-metrics" /> Show performance bar</label>
          <label><input type="checkbox" id="set-cursors" /> Show other players' cursors</label>
        </div>

        <div class="settings-actions">
          <button id="set-change-world" class="settings-btn">Change world</button>
          <button id="set-logout" class="settings-btn settings-btn--danger">Log out</button>
        </div>
      </div>
    `;
    document.body.appendChild(overlay);
    this.overlay = overlay;

    // Clicking the backdrop dismisses, but clicks inside the card must not.
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) this.hide();
    });
    overlay.querySelector(".settings-close")!
      .addEventListener("click", () => this.hide());

    this.wireVolume(overlay);
    this.wireRenderScale(overlay);
    this.wireToggles(overlay);
    this.wireSessionActions(overlay);

    // Escape toggles the panel, except while typing: pressing Escape in the chat
    // box should dismiss the field, not open settings over it.
    window.addEventListener("keydown", (e) => {
      if (e.key !== "Escape") return;
      const el = document.activeElement;
      const typing = el instanceof HTMLInputElement
        || el instanceof HTMLTextAreaElement
        || el instanceof HTMLSelectElement;
      if (typing && !this.isOpen()) return;
      this.toggle();
    });
  }

  private wireVolume(root: HTMLElement): void {
    const slider = root.querySelector<HTMLInputElement>("#set-volume")!;
    const readout = root.querySelector<HTMLOutputElement>("#set-volume-value")!;
    slider.addEventListener("input", () => {
      const v = parseInt(slider.value, 10) / 100;
      readout.textContent = `${slider.value}%`;
      this.update("volume", v);
      this.update("muted", v === 0);
      AudioEngine.getInstance().setVolume(v);
    });
  }

  private wireRenderScale(root: HTMLElement): void {
    const select = root.querySelector<HTMLSelectElement>("#set-scale")!;
    select.value = String(this.settings.renderScale);
    select.addEventListener("change", () => {
      const scale = parseFloat(select.value);
      this.update("renderScale", scale);
      this.onRenderScaleChange?.(scale);
    });
  }

  private wireToggles(root: HTMLElement): void {
    const bind = (
      id: string,
      key: keyof Settings,
      apply: (on: boolean) => void,
    ) => {
      const box = root.querySelector<HTMLInputElement>(id)!;
      box.checked = this.settings[key] as boolean;
      box.addEventListener("change", () => {
        this.update(key, box.checked as never);
        apply(box.checked);
      });
    };

    bind("#set-minimap", "showMinimap", (on) => this.applyVisibility("minimap", on));
    bind("#set-metrics", "showMetrics", (on) => this.applyVisibility("footer", on));
    // Cursors are created and positioned continuously by the UI controller, so a
    // class on the container is used rather than setting styles on each element,
    // which would be undone on the next cursor update.
    bind("#set-cursors", "showCursors", (on) => applyCursorVisibility(on));
  }

  private wireSessionActions(root: HTMLElement): void {
    root.querySelector("#set-change-world")!.addEventListener("click", () => {
      // Keep the identity, drop the world choice, and force the lobby.
      const url = new URL(location.href);
      url.searchParams.set("lobby", "1");
      url.searchParams.delete("world");
      location.href = url.toString();
    });

    root.querySelector("#set-logout")!.addEventListener("click", async () => {
      // Tell the server first. Clearing local storage on its own left the token
      // valid until it expired, so logging out did not end the session.
      await endSession();
      const url = new URL(location.href);
      url.search = "";
      location.href = url.toString();
    });
  }

  isOpen(): boolean {
    return this.overlay?.classList.contains("visible") ?? false;
  }

  show(): void {
    this.overlay?.classList.add("visible");
  }

  hide(): void {
    this.overlay?.classList.remove("visible");
  }

  toggle(): void {
    this.isOpen() ? this.hide() : this.show();
  }
}

/** Toggles remote cursor visibility via a container class. */
function applyCursorVisibility(visible: boolean): void {
  document.getElementById("game-container")
    ?.classList.toggle("hide-cursors", !visible);
}

function escapeHtml(s: string): string {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}
