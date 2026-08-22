/**
 * pixi-renderer.ts — PixiJS 8 World Renderer
 *
 * Replaces raw WebGL2 management with PixiJS for context handling, batching,
 * and state management. Uses a data-texture + custom shader approach for the
 * material grid, with separate containers for creatures, particles, and UI.
 *
 * Architecture:
 *  - Material grid: PIXI.Sprite + dynamic Texture from Uint8Array, custom Filter
 *    maps material IDs → palette colors in a fragment shader
 *  - Creature layer: PIXI.Container with small colored sprites (Herbivore/Predator)
 *  - Particle layer: PIXI.Container for cosmetic effects
 *  - UI overlay layer: Cursor indicators, power radius
 *  - Zoom: stage.scale
 *  - Smooth camera: stage.position with lerp
 */

import {
  Application,
  Container,
  Sprite,
  Texture,
  Graphics,
  Filter,
  GlProgram,
  TextureSource,
} from "pixi.js";
import type { FullSnapshot, ChunkUpdate, IWorldRenderer } from "./renderer.js";

// ── Palette (same as renderer.ts / webgl2-renderer.ts) ─────────────────────

const PALETTE_DATA = new Uint8Array([
  /* 0  Empty */     13,  13,  16,  255,
  /* 1  Rock  */     80,  75,  70,  255,
  /* 2  Soil  */     101, 67,  33,  255,
  /* 3  Sand  */     210, 180, 100, 255,
  /* 4  Water */     30,  100, 200, 220,
  /* 5  Plant */     50,  140, 50,  255,
  /* 6  Fire  */     240, 100, 20,  255,
  /* 7  Vapor */     180, 210, 240, 120,
  /* 8  Smoke */     80,  80,  80,  160,
  /* 9  Lava  */     255, 80,  0,   255,
  /* 10 Ice   */     180, 230, 255, 255,
  /* 11 Ash   */     50,  45,  40,  200,
  /* 12 Oil   */     80,  60,  20,  255,
  /* 13 Ember */     200, 50,  10,  180,
  /* 14 Herbivore */ 100, 200, 60,  255,
  /* 15 Predator  */ 220, 50,  50,  255,
  /* 16 Cloud */     200, 200, 220, 180,
]);

// ── Shader for material ID → color mapping ─────────────────────────────────

const MATERIAL_VERT = /* glsl */ `
in vec2 aPosition;
out vec2 vTextureCoord;

uniform vec4 uInputSize;
uniform vec4 uOutputFrame;
uniform vec4 uOutputTexture;

vec4 filterVertexPosition(void) {
  vec2 position = aPosition * uOutputFrame.zw + uOutputFrame.xy;
  position.x = position.x * (2.0 / uOutputTexture.x) - 1.0;
  position.y = position.y * (2.0*uOutputTexture.z / uOutputTexture.y) - uOutputTexture.z;
  return vec4(position, 0.0, 1.0);
}

vec2 filterTextureCoord(void) {
  return aPosition * (uOutputFrame.zw * uInputSize.zw);
}

void main(void) {
  gl_Position = filterVertexPosition();
  vTextureCoord = filterTextureCoord();
}
`;

const MATERIAL_FRAG = /* glsl */ `
in vec2 vTextureCoord;
out vec4 finalColor;

uniform sampler2D uTexture;
uniform sampler2D uPalette;
uniform float uTime;

void main(void) {
  // Sample material ID from R channel (stored as normalized float: id/255)
  float matNorm = texture(uTexture, vTextureCoord).r;
  float matID = floor(matNorm * 255.0 + 0.5);

  // Lookup palette: palette is 256x1 texture
  vec4 col = texture(uPalette, vec2((matID + 0.5) / 256.0, 0.5));

  // Water shimmer
  if (matID == 4.0) {
    float shimmer = sin(uTime * 2.0 + vTextureCoord.x * 40.0 + vTextureCoord.y * 30.0) * 0.05;
    col.rgb += shimmer;
  }

  // Fire flicker
  if (matID == 6.0 || matID == 13.0) {
    float flicker = sin(uTime * 8.0 + vTextureCoord.x * 20.0) * 0.08;
    col.r += flicker;
    col.g -= flicker * 0.5;
  }

  finalColor = col;
}
`;

// ── Zoom levels ────────────────────────────────────────────────────────────

const ZOOM_LEVELS = [0.25, 0.5, 1, 2, 4];

// ── Camera lerp factor ─────────────────────────────────────────────────────

const CAMERA_LERP = 0.12;

// ── PixiJS Renderer ────────────────────────────────────────────────────────

export class PixiWorldRenderer implements IWorldRenderer {
  private app: Application;
  private initialized = false;

  // Layers
  private worldContainer!: Container;
  private materialSprite!: Sprite;
  private creatureContainer!: Container;
  private particleContainer!: Container;
  private uiContainer!: Container;

  // Textures
  private materialTextureSource!: TextureSource;
  private materialTexture!: Texture;
  private paletteTexture!: Texture;
  private materialFilter!: Filter;

  // Material cache (same as other renderers)
  private materialCache: Uint8Array | null = null;

  // Camera state
  private _viewX = 0;
  private _viewY = 0;
  private targetViewX = 0;
  private targetViewY = 0;

  get viewX(): number { return this._viewX; }
  set viewX(v: number) {
    this._viewX = v;
    this.targetViewX = v;
  }

  get viewY(): number { return this._viewY; }
  set viewY(v: number) {
    this._viewY = v;
    this.targetViewY = v;
  }

  // World dimensions
  worldW = 0;
  worldH = 0;

  chunkSize = 64;
  zoom = 1;

  private readonly canvas: HTMLCanvasElement;
  private time = 0;

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    this.app = new Application();
  }

  /** Async initialization — must be called before use. */
  async init(): Promise<void> {
    await this.app.init({
      canvas: this.canvas,
      width: this.canvas.width,
      height: this.canvas.height,
      backgroundColor: 0x0d0d10,
      antialias: false,
      resolution: window.devicePixelRatio || 1,
      autoDensity: true,
    });

    // Create layer hierarchy
    this.worldContainer = new Container();
    this.creatureContainer = new Container();
    this.particleContainer = new Container();
    this.uiContainer = new Container();

    this.app.stage.addChild(this.worldContainer);
    this.app.stage.addChild(this.creatureContainer);
    this.app.stage.addChild(this.particleContainer);
    this.app.stage.addChild(this.uiContainer);

    // Create palette texture (256x1 RGBA)
    const paletteBuffer = new Uint8Array(256 * 4);
    paletteBuffer.set(PALETTE_DATA);
    this.paletteTexture = Texture.from({
      resource: paletteBuffer,
      width: 256,
      height: 1,
      format: "rgba8unorm",
      alphaMode: "premultiply-alpha-on-upload",
    });

    // Set up ticker for animation
    this.app.ticker.add((ticker) => {
      this.time += ticker.deltaMS / 1000;
      this.updateCamera();
      if (this.materialFilter) {
        this.materialFilter.resources.materialUniforms.uniforms.uTime = this.time;
      }
    });

    this.initialized = true;
  }

  /** Called when the server sends world dimensions. */
  initWorld(w: number, h: number): void {
    this.worldW = w;
    this.worldH = h;
    this.materialCache = new Uint8Array(w * h);

    // Create material data texture (R8 — one byte per cell = material ID)
    // We use an RGBA texture with material ID in R channel for compatibility
    const texData = new Uint8Array(w * h * 4);
    this.materialTextureSource = new TextureSource({
      resource: texData,
      width: w,
      height: h,
      format: "rgba8unorm",
      alphaMode: "no-premultiply-alpha",
      scaleMode: "nearest",
      autoGenerateMipmaps: false,
    });
    this.materialTexture = new Texture({ source: this.materialTextureSource });

    // Create the material sprite that fills the world
    this.materialSprite = new Sprite(this.materialTexture);
    this.materialSprite.width = w;
    this.materialSprite.height = h;
    this.worldContainer.addChild(this.materialSprite);

    // Custom filter that maps material IDs to colors
    this.materialFilter = new Filter({
      glProgram: GlProgram.from({
        vertex: MATERIAL_VERT,
        fragment: MATERIAL_FRAG,
      }),
      resources: {
        materialUniforms: {
          uTime: { value: 0, type: "f32" },
        },
        uPalette: this.paletteTexture.source,
      },
    });
    // No custom filter needed — we upload pre-colored RGBA directly
    // this.materialSprite.filters = [this.materialFilter];
  }

  /** Step zoom in or out. Returns new zoom level. */
  stepZoom(direction: 1 | -1): number {
    const curIdx = ZOOM_LEVELS.indexOf(this.zoom);
    const idx = curIdx === -1
      ? ZOOM_LEVELS.findIndex((z) => z >= this.zoom)
      : curIdx;
    const newIdx = Math.max(0, Math.min(ZOOM_LEVELS.length - 1, idx + direction));
    this.zoom = ZOOM_LEVELS[newIdx];
    this.applyZoom();
    return this.zoom;
  }

  private applyZoom(): void {
    this.app.stage.scale.set(this.zoom);
  }

  /** How many world cells are visible horizontally/vertically */
  get visibleW(): number {
    return Math.ceil(this.canvas.width / this.zoom);
  }
  get visibleH(): number {
    return Math.ceil(this.canvas.height / this.zoom);
  }

  /** Apply a full viewport snapshot from the server. */
  applySnapshot(snap: FullSnapshot): void {
    if (!this.materialCache) return;
    for (let row = 0; row < snap.h; row++) {
      for (let col = 0; col < snap.w; col++) {
        const wx = snap.x + col;
        const wy = snap.y + row;
        if (wx >= 0 && wx < this.worldW && wy >= 0 && wy < this.worldH) {
          this.materialCache[wy * this.worldW + wx] = snap.data[row * snap.w + col];
        }
      }
    }
    this.uploadMaterialTexture();
    this.updateCreatures();
  }

  /** Apply a list of dirty chunk updates from the server. */
  applyChunkUpdates(updates: ChunkUpdate[]): void {
    if (!this.materialCache) return;
    const cs = this.chunkSize;
    for (const u of updates) {
      const cx0 = u.cx * cs;
      const cy0 = u.cy * cs;
      const cxEnd = Math.min(cx0 + cs, this.worldW);
      const cyEnd = Math.min(cy0 + cs, this.worldH);
      let idx = 0;
      for (let y = cy0; y < cyEnd; y++) {
        for (let x = cx0; x < cxEnd; x++) {
          this.materialCache[y * this.worldW + x] = u.data[idx++];
        }
      }
    }
    this.uploadMaterialTexture();
    this.updateCreatures();
  }

  /** Upload material cache to GPU texture — maps material IDs to RGBA colors directly */
  private uploadMaterialTexture(): void {
    if (!this.materialCache || !this.materialTextureSource) return;
    const w = this.worldW;
    const h = this.worldH;
    const texData = new Uint8Array(w * h * 4);
    for (let i = 0; i < w * h; i++) {
      const mat = this.materialCache[i];
      const p = mat * 4;
      texData[i * 4]     = PALETTE_DATA[p]     ?? 13;
      texData[i * 4 + 1] = PALETTE_DATA[p + 1] ?? 13;
      texData[i * 4 + 2] = PALETTE_DATA[p + 2] ?? 16;
      texData[i * 4 + 3] = PALETTE_DATA[p + 3] ?? 255;
    }
    this.materialTextureSource.resource = texData;
    this.materialTextureSource.update();
  }

  /** Update creature sprites from material cache */
  private updateCreatures(): void {
    if (!this.materialCache) return;

    // Clear existing creature sprites
    this.creatureContainer.removeChildren();

    // Scan visible region for creatures
    const startX = Math.max(0, Math.floor(this.viewX));
    const startY = Math.max(0, Math.floor(this.viewY));
    const endX = Math.min(this.worldW, startX + this.visibleW);
    const endY = Math.min(this.worldH, startY + this.visibleH);

    for (let y = startY; y < endY; y++) {
      for (let x = startX; x < endX; x++) {
        const mat = this.materialCache[y * this.worldW + x];
        if (mat === 14 || mat === 15) {
          const g = new Graphics();
          const color = mat === 14 ? 0x64c83c : 0xdc3232;
          g.circle(0, 0, 0.4);
          g.fill(color);
          g.position.set(x + 0.5, y + 0.5);
          this.creatureContainer.addChild(g);
        }
      }
    }
  }

  /** Smooth camera interpolation */
  private updateCamera(): void {
    // Lerp toward target
    this._viewX += (this.targetViewX - this._viewX) * CAMERA_LERP;
    this._viewY += (this.targetViewY - this._viewY) * CAMERA_LERP;

    // Position stage so that viewX/viewY is at top-left of canvas
    this.app.stage.position.set(
      -this._viewX * this.zoom,
      -this._viewY * this.zoom,
    );
  }

  /** Resize internal rendering after canvas resize. */
  onResize(): void {
    if (!this.initialized) return;
    this.app.renderer.resize(this.canvas.width, this.canvas.height);
    this.applyZoom();
  }

  /** Expose material cache for minimap rendering and client-side prediction. */
  getMaterialCache(): Uint8Array | null {
    return this.materialCache;
  }

  /** Public draw trigger for client-side prediction (immediate feedback). */
  drawImmediate(): void {
    this.uploadMaterialTexture();
  }

  /** Set camera target for smooth scrolling (external callers can use this for animated pan). */
  setViewTarget(x: number, y: number): void {
    this.targetViewX = Math.max(0, Math.min(this.worldW - this.visibleW, x));
    this.targetViewY = Math.max(0, Math.min(this.worldH - this.visibleH, y));
  }

  /** Dispose of PixiJS resources */
  dispose(): void {
    this.app.destroy(true, { children: true, texture: true });
  }
}

/** Check if PixiJS can initialize (WebGL2 required) */
export function isPixiAvailable(): boolean {
  try {
    const testCanvas = document.createElement("canvas");
    const gl = testCanvas.getContext("webgl2");
    return gl !== null;
  } catch {
    return false;
  }
}
