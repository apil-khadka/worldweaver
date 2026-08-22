/** Centralized client-side configuration. No secrets here. */
export const networkConfig = {
  reconnectDelayMs: 1_000,
  maxReconnectDelayMs: 10_000,
  pingIntervalMs: 3_000,
} as const;

export const rendererConfig = {
  preferenceOrder: ["webgpu", "webgl2", "canvas2d"] as const,
  defaultRenderScale: 1.0,
  metricsUpdateHz: 2,
} as const;

export const inputConfig = {
  keyPanSpeed: 8,
  defaultPowerRadius: 24,
  zoomStep: 0.1,
  minZoom: 0.25,
  maxZoom: 8.0,
} as const;

export const storageKeys = {
  settings: "ww_settings_v1",
} as const;

export const features = {
  webgpu: true,
  canvasFallback: true,
  waterAnimation: true,
  fireAnimation: true,
  debugMetrics: false,
} as const;
