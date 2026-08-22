/**
 * PlatformAdapter.ts — Cross-platform abstraction
 *
 * WorldWeaver targets two platforms:
 *   - Browser (Chrome, Firefox, Safari)
 *   - Tauri   (Windows, macOS, Linux, Android, iOS)
 *
 * This interface isolates all platform-specific calls so that game and
 * rendering code never imports Tauri APIs directly.  Switching from
 * browser to Tauri must not require changes to gameplay. (Addendum § 14, 61)
 *
 * Runtime selection:
 *   "__TAURI__" global presence → TauriPlatform
 *   otherwise                   → BrowserPlatform
 */

export type Platform = "browser" | "tauri-desktop" | "tauri-mobile";

export interface ClientSettings {
  rendererBackend: "auto" | "webgl2" | "webgpu" | "canvas2d";
  quality:         "LOW" | "MEDIUM" | "HIGH" | "ULTRA" | "AUTO";
  renderScale:     number;
  debugMetrics:    boolean;
  serverUrl:       string;
}

const DEFAULT_SETTINGS: ClientSettings = {
  rendererBackend: "auto",
  quality:         "AUTO",
  renderScale:     1.0,
  debugMetrics:    true,
  serverUrl:       "",
};

export interface PlatformAdapter {
  readonly platform: Platform;

  enterFullscreen(): Promise<void>;
  exitFullscreen(): Promise<void>;

  loadSettings(): Promise<ClientSettings>;
  saveSettings(settings: ClientSettings): Promise<void>;

  /** True if native features (file export, haptics, etc.) are available. */
  supportsNativeFeatures(): boolean;

  /** Optional: trigger haptic feedback (Tauri mobile only) */
  haptic?(): void;
}

// ── BrowserPlatform ─────────────────────────────────────────────────────────

export class BrowserPlatform implements PlatformAdapter {
  readonly platform: Platform = "browser";

  async enterFullscreen(): Promise<void> {
    await document.documentElement.requestFullscreen?.();
  }

  async exitFullscreen(): Promise<void> {
    await document.exitFullscreen?.();
  }

  async loadSettings(): Promise<ClientSettings> {
    try {
      const raw = localStorage.getItem("ww_settings");
      if (raw) return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
    } catch { /* ignore */ }
    return { ...DEFAULT_SETTINGS };
  }

  async saveSettings(settings: ClientSettings): Promise<void> {
    try {
      localStorage.setItem("ww_settings", JSON.stringify(settings));
    } catch { /* ignore */ }
  }

  supportsNativeFeatures(): boolean { return false; }
}

// ── TauriPlatform ────────────────────────────────────────────────────────────

/**
 * TauriPlatform delegates to @tauri-apps/api for native capabilities.
 * It is only instantiated when `window.__TAURI__` is detected, so the
 * browser bundle never imports Tauri packages.
 *
 * The dynamic import ensures the Tauri IPC plugin is only loaded inside
 * a Tauri context.
 */
export class TauriPlatform implements PlatformAdapter {
  readonly platform: Platform = "tauri-desktop";

  async enterFullscreen(): Promise<void> {
    const { getCurrentWindow } = await import("@tauri-apps/api/window");
    await getCurrentWindow().setFullscreen(true);
  }

  async exitFullscreen(): Promise<void> {
    const { getCurrentWindow } = await import("@tauri-apps/api/window");
    await getCurrentWindow().setFullscreen(false);
  }

  async loadSettings(): Promise<ClientSettings> {
    try {
      // Tauri v2: fs plugin is a separate package, fallback to localStorage
      const raw = localStorage.getItem("ww_settings");
      if (raw) return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
    } catch { /* ignore */ }
    return { ...DEFAULT_SETTINGS };
  }

  async saveSettings(settings: ClientSettings): Promise<void> {
    try {
      localStorage.setItem("ww_settings", JSON.stringify(settings));
    } catch { /* ignore */ }
  }

  supportsNativeFeatures(): boolean { return true; }
}

// ── Factory ──────────────────────────────────────────────────────────────────

/** createPlatform selects the correct adapter at runtime. */
export function createPlatform(): PlatformAdapter {
  if (typeof window !== "undefined" && (window as any).__TAURI__) {
    console.info("[platform] Tauri context detected");
    return new TauriPlatform();
  }
  return new BrowserPlatform();
}
