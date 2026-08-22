/**
 * IWorldRenderer.ts — Renderer abstraction interface
 *
 * All renderer backends (WebGL2, WebGPU, Canvas2D) implement this interface.
 * Code outside the render/ directory must never import a concrete renderer
 * directly — only this interface. This enforces the architectural invariant:
 *
 *   "Switching WebGL2 → WebGPU must not require changes to simulation,
 *    game, or transport code." (Architecture Addendum § 61)
 *
 * # Runtime selection
 *
 *   startup
 *     ↓
 *   WebGPU usable? → WebGPURenderer
 *     ↓ no
 *   WebGL2 usable? → WebGL2Renderer
 *     ↓ no
 *   Canvas2DRenderer
 *
 * # Data contract
 *
 * The renderer consumes material IDs and environmental fields.
 * It never receives renderer-specific draw instructions from the server.
 * "cell = WATER" is valid.  "draw blue rect at 400,200" is not.
 */

/** Material IDs — must match internal/systems/materials/registry.go */
export const enum MatID {
  Empty = 0,
  Rock  = 1,
  Soil  = 2,
  Sand  = 3,
  Water = 4,
  Plant = 5,
  Fire  = 6,
  Vapor = 7,
  Smoke = 8,
  Lava  = 9,
  Ice   = 10,
  Ash   = 11,
  Oil   = 12,
  Ember = 13,
}

/** Camera describes the visible viewport into the world. */
export interface Camera {
  /** World-coordinate of the top-left pixel */
  x: number;
  y: number;
  /** Viewport dimensions in canvas pixels */
  width: number;
  height: number;
  /** Zoom factor (1.0 = one cell per pixel) */
  zoom: number;
}

/** A full viewport snapshot received from the server. */
export interface WorldSnapshot {
  tick: number;
  /** World coordinate of snapshot origin */
  x: number;
  y: number;
  width: number;
  height: number;
  /** Flat array of material IDs, row-major: index = (y-origin)*width + (x-origin) */
  data: Uint8Array;
}

/** A dirty-chunk delta received from the server. */
export interface ChunkUpdate {
  /** Chunk grid coordinates */
  cx: number;
  cy: number;
  tick: number;
  /** Flat material array covering the chunk region */
  data: Uint8Array;
}

/** Configuration passed to initialize(). */
export interface RendererConfig {
  canvas: HTMLCanvasElement;
  worldW: number;
  worldH: number;
  /** Cells per chunk edge — must match the server ChunkSize */
  chunkSize: number;
  /** Enable/disable optional visual layers */
  layers?: {
    environment?: boolean;
    particles?:   boolean;
    lighting?:    boolean;
    debug?:       boolean;
  };
  /** Quality preset: LOW | MEDIUM | HIGH | ULTRA | AUTO */
  quality?: "LOW" | "MEDIUM" | "HIGH" | "ULTRA" | "AUTO";
}

/**
 * IWorldRenderer is the single contract all rendering backends must satisfy.
 *
 * Implementations:
 *  - Canvas2DRenderer  (canvas/Canvas2DRenderer.ts)  — fallback / debug
 *  - WebGL2Renderer    (webgl2/WebGL2Renderer.ts)    — baseline accelerated
 *  - WebGPURenderer    (webgpu/WebGPURenderer.ts)    — optional, modern GPU
 */
export interface IWorldRenderer {
  /** Backend name, exposed in diagnostics UI. */
  readonly name: string;

  /** Async setup: allocate GPU resources, compile shaders, etc. */
  initialize(config: RendererConfig): Promise<void>;

  /** Apply a full viewport snapshot (join or reconnect). */
  applySnapshot(snapshot: WorldSnapshot): void;

  /** Apply a single dirty-chunk delta. */
  applyChunk(update: ChunkUpdate): void;

  /** Update the visible camera region. */
  setCamera(camera: Camera): void;

  /** Called on canvas/window resize. */
  resize(width: number, height: number): void;

  /**
   * Render one frame.
   * @param time  DOMHighResTimeStamp from requestAnimationFrame
   */
  render(time: number): void;

  /** Release GPU resources. */
  dispose(): void;
}

/**
 * selectRenderer detects GPU capabilities at runtime and returns the
 * appropriate renderer backend.
 *
 * This is the only place that contains feature-detection logic.
 * All other code imports and uses IWorldRenderer.
 */
export async function selectRenderer(): Promise<IWorldRenderer> {
  // Try WebGPU first
  if ("gpu" in navigator) {
    try {
      const adapter = await (navigator as any).gpu.requestAdapter();
      if (adapter) {
        const { WebGPURenderer } = await import("./webgpu/WebGPURenderer.js");
        console.info("[renderer] selected: WebGPU");
        return new WebGPURenderer();
      }
    } catch {
      console.warn("[renderer] WebGPU available but failed to initialize, falling back");
    }
  }

  // Try WebGL2
  const probe = document.createElement("canvas");
  if (probe.getContext("webgl2")) {
    const { WebGL2Renderer } = await import("./webgl2/WebGL2Renderer.js");
    console.info("[renderer] selected: WebGL2");
    return new WebGL2Renderer();
  }

  // Canvas2D fallback
  console.warn("[renderer] GPU not available — using Canvas2D fallback");
  const { Canvas2DRenderer } = await import("./canvas/Canvas2DRenderer.js");
  return new Canvas2DRenderer();
}
