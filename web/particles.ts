/**
 * particles.ts — Cosmetic particle effects for WorldWeaver
 *
 * A lightweight Canvas2D particle system that runs independently of the server
 * tick rate. Adds visual polish: rain streaks, fire embers, smoke wisps,
 * water splashes, and leaf drift.
 *
 * Design:
 *  - Object pool (max 500) to avoid GC pressure
 *  - requestAnimationFrame loop, decoupled from simulation
 *  - Reads renderer.getMaterialCache() to detect fire/water/plants/smoke
 *  - Renders on a dedicated overlay canvas (pointer-events: none)
 */

import type { IWorldRenderer } from "./renderer.js";

// ── Material IDs (must match renderer.ts / materials.go) ──────────────────
const Mat = {
  Water: 4,
  Plant: 5,
  Fire:  6,
  Smoke: 8,
} as const;

// ── Particle types ────────────────────────────────────────────────────────
const enum PType {
  Rain   = 0,
  Ember  = 1,
  Smoke  = 2,
  Splash = 3,
  Leaf   = 4,
}

// ── Configuration ─────────────────────────────────────────────────────────
const MAX_PARTICLES = 500;
const RAIN_COUNT    = 80;       // target active rain particles
const SCAN_INTERVAL = 500;      // ms between material cache scans
const SPLASH_BURST  = 4;        // particles per water landing

interface Particle {
  active: boolean;
  type:   PType;
  x:      number;
  y:      number;
  vx:     number;
  vy:     number;
  life:   number;  // remaining life in seconds
  maxLife: number;
  r:      number;
  g:      number;
  b:      number;
  size:   number;
}

export class ParticleSystem {
  private readonly ctx: CanvasRenderingContext2D;
  private readonly pool: Particle[] = [];
  private width  = 0;
  private height = 0;
  private lastTime = 0;
  private lastScan = 0;
  private running  = false;
  private rainActive = false;
  private windX = 0.3; // slight wind offset for rain/leaves

  // Track previous water positions for splash detection
  private prevWaterSet: Set<number> = new Set();

  constructor(
    private readonly canvas: HTMLCanvasElement,
    private readonly renderer: IWorldRenderer,
    private readonly getActivePower: () => number,
  ) {
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Particle canvas 2D not supported");
    this.ctx = ctx;

    // Pre-allocate pool
    for (let i = 0; i < MAX_PARTICLES; i++) {
      this.pool.push({
        active: false, type: PType.Rain,
        x: 0, y: 0, vx: 0, vy: 0,
        life: 0, maxLife: 1,
        r: 255, g: 255, b: 255, size: 1,
      });
    }
  }

  resize(w: number, h: number): void {
    this.canvas.width  = w;
    this.canvas.height = h;
    this.width  = w;
    this.height = h;
  }

  start(): void {
    if (this.running) return;
    this.running  = true;
    this.lastTime = performance.now();
    this.tick(this.lastTime);
  }

  stop(): void {
    this.running = false;
  }

  // ── Main loop ─────────────────────────────────────────────────────────

  private tick = (now: number): void => {
    if (!this.running) return;
    const dt = Math.min((now - this.lastTime) / 1000, 0.05); // cap to avoid spiral
    this.lastTime = now;

    // Periodic material scan for embers/smoke/leaves/splashes
    if (now - this.lastScan > SCAN_INTERVAL) {
      this.lastScan = now;
      this.scanMaterials();
    }

    // Rain spawning (when Rain power is selected — index 0)
    this.rainActive = this.getActivePower() === 0;
    if (this.rainActive) {
      this.spawnRain();
    }

    // Update & render
    this.update(dt);
    this.render();

    requestAnimationFrame(this.tick);
  };

  // ── Spawners ──────────────────────────────────────────────────────────

  private spawnRain(): void {
    let activeRain = 0;
    for (const p of this.pool) {
      if (p.active && p.type === PType.Rain) activeRain++;
    }
    const need = RAIN_COUNT - activeRain;
    for (let i = 0; i < need; i++) {
      const p = this.acquire();
      if (!p) break;
      p.type = PType.Rain;
      p.x = Math.random() * this.width;
      p.y = -Math.random() * 40; // start above viewport
      p.vx = 20 + Math.random() * 30; // wind drift
      p.vy = 400 + Math.random() * 200; // fast fall
      p.life = 1.5 + Math.random() * 0.5;
      p.maxLife = p.life;
      p.r = 100; p.g = 160; p.b = 255;
      p.size = 1.5;
    }
  }

  private spawnEmber(sx: number, sy: number): void {
    const p = this.acquire();
    if (!p) return;
    p.type = PType.Ember;
    p.x = sx + (Math.random() - 0.5) * 4;
    p.y = sy + (Math.random() - 0.5) * 4;
    p.vx = (Math.random() - 0.5) * 30;
    p.vy = -(40 + Math.random() * 60); // float up
    p.life = 0.8 + Math.random() * 0.6;
    p.maxLife = p.life;
    // Orange/yellow gradient
    p.r = 220 + Math.floor(Math.random() * 35);
    p.g = 80 + Math.floor(Math.random() * 100);
    p.b = 0;
    p.size = 1.5 + Math.random() * 1.5;
  }

  private spawnSmoke(sx: number, sy: number): void {
    const p = this.acquire();
    if (!p) return;
    p.type = PType.Smoke;
    p.x = sx + (Math.random() - 0.5) * 6;
    p.y = sy;
    p.vx = (Math.random() - 0.5) * 15;
    p.vy = -(15 + Math.random() * 25); // drift up slowly
    p.life = 1.2 + Math.random() * 1.0;
    p.maxLife = p.life;
    p.r = 120; p.g = 120; p.b = 130;
    p.size = 2 + Math.random() * 2;
  }

  private spawnSplash(sx: number, sy: number): void {
    for (let i = 0; i < SPLASH_BURST; i++) {
      const p = this.acquire();
      if (!p) break;
      p.type = PType.Splash;
      p.x = sx;
      p.y = sy;
      const angle = Math.random() * Math.PI * 2;
      const speed = 60 + Math.random() * 80;
      p.vx = Math.cos(angle) * speed;
      p.vy = Math.sin(angle) * speed - 40; // bias upward
      p.life = 0.15 + Math.random() * 0.1; // very short
      p.maxLife = p.life;
      p.r = 80; p.g = 150; p.b = 255;
      p.size = 1.5 + Math.random();
    }
  }

  private spawnLeaf(sx: number, sy: number): void {
    const p = this.acquire();
    if (!p) return;
    p.type = PType.Leaf;
    p.x = sx + (Math.random() - 0.5) * 10;
    p.y = sy + (Math.random() - 0.5) * 10;
    p.vx = this.windX * 40 + (Math.random() - 0.5) * 20;
    p.vy = -(5 + Math.random() * 15); // gentle rise
    p.life = 2.0 + Math.random() * 2.0;
    p.maxLife = p.life;
    // Varied greens
    p.r = 30 + Math.floor(Math.random() * 40);
    p.g = 120 + Math.floor(Math.random() * 80);
    p.b = 20 + Math.floor(Math.random() * 30);
    p.size = 1 + Math.random() * 1.5;
  }

  // ── Material scanning ─────────────────────────────────────────────────

  private scanMaterials(): void {
    const cache = this.renderer.getMaterialCache();
    if (!cache) return;

    const { viewX, viewY, worldW, zoom } = this.renderer;
    const visW = Math.ceil(this.width / zoom);
    const visH = Math.ceil(this.height / zoom);

    // Sample a grid (every 8th cell to save CPU)
    const step = 8;
    const newWaterSet = new Set<number>();

    for (let gy = 0; gy < visH; gy += step) {
      for (let gx = 0; gx < visW; gx += step) {
        const wx = viewX + gx;
        const wy = viewY + gy;
        if (wx < 0 || wx >= worldW || wy < 0 || wy >= this.renderer.worldH) continue;

        const mat = cache[wy * worldW + wx];
        const sx = gx * zoom;
        const sy = gy * zoom;

        if (mat === Mat.Fire) {
          // ~1 ember per visible fire cell per second → at 500ms scan, 50% chance
          if (Math.random() < 0.5) {
            this.spawnEmber(sx, sy);
          }
        } else if (mat === Mat.Smoke) {
          if (Math.random() < 0.3) {
            this.spawnSmoke(sx, sy);
          }
        } else if (mat === Mat.Plant) {
          if (Math.random() < 0.05) { // rare leaf
            this.spawnLeaf(sx, sy);
          }
        } else if (mat === Mat.Water) {
          const key = wy * worldW + wx;
          newWaterSet.add(key);
          // Detect new water (splash)
          if (!this.prevWaterSet.has(key)) {
            if (Math.random() < 0.3) { // don't splash every cell
              this.spawnSplash(sx, sy);
            }
          }
        }
      }
    }
    this.prevWaterSet = newWaterSet;
  }

  // ── Update ────────────────────────────────────────────────────────────

  private update(dt: number): void {
    for (const p of this.pool) {
      if (!p.active) continue;

      p.life -= dt;
      if (p.life <= 0) {
        p.active = false;
        continue;
      }

      p.x += p.vx * dt;
      p.y += p.vy * dt;

      // Gravity for splashes
      if (p.type === PType.Splash) {
        p.vy += 400 * dt; // gravity pull
      }

      // Wind wobble for leaves
      if (p.type === PType.Leaf) {
        p.vx += Math.sin(this.lastTime / 600 + p.y * 0.01) * 10 * dt;
      }

      // Remove if off-screen
      if (p.x < -20 || p.x > this.width + 20 || p.y > this.height + 20) {
        p.active = false;
      }
    }
  }

  // ── Render ────────────────────────────────────────────────────────────

  private render(): void {
    const { ctx, width, height } = this;
    ctx.clearRect(0, 0, width, height);

    for (const p of this.pool) {
      if (!p.active) continue;

      const alpha = Math.max(0, p.life / p.maxLife);

      if (p.type === PType.Rain) {
        // Render as a thin streak/line
        ctx.strokeStyle = `rgba(${p.r},${p.g},${p.b},${(alpha * 0.6).toFixed(2)})`;
        ctx.lineWidth = p.size;
        ctx.beginPath();
        ctx.moveTo(p.x, p.y);
        ctx.lineTo(p.x - p.vx * 0.008, p.y - p.vy * 0.008); // short trail
        ctx.stroke();
      } else {
        // Render as filled circle
        ctx.fillStyle = `rgba(${p.r},${p.g},${p.b},${alpha.toFixed(2)})`;
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  }

  // ── Pool management ───────────────────────────────────────────────────

  private acquire(): Particle | null {
    for (const p of this.pool) {
      if (!p.active) {
        p.active = true;
        return p;
      }
    }
    return null; // pool exhausted
  }
}
