/**
 * prediction.ts — Client-side prediction for power application
 *
 * Applies LOCAL visual changes immediately on click so powers feel instant.
 * The next server chunk update overwrites predictions with authoritative state.
 *
 * Material IDs (must match renderer.ts / materials.go):
 *   Empty=0, Rock=1, Soil=2, Sand=3, Water=4, Plant=5, Fire=6,
 *   Vapor=7, Smoke=8, Lava=9, Ice=10, Ash=11, Oil=12, Ember=13,
 *   Herbivore=14, Predator=15, Cloud=16
 */

import { WorldRenderer } from "./renderer.js";
import { PowerEffects } from "./effects.js";

const enum Mat {
  Empty     = 0,
  Rock      = 1,
  Soil      = 2,
  Sand      = 3,
  Water     = 4,
  Plant     = 5,
  Fire      = 6,
  Herbivore = 14,
}

export class ClientPrediction {
  constructor(
    private readonly renderer: WorldRenderer,
    private readonly effects: PowerEffects,
  ) {}

  /**
   * Apply an optimistic local prediction for a power at (wx, wy) with given radius.
   * Mutates the renderer's material cache directly and triggers an immediate redraw.
   */
  predict(power: number, wx: number, wy: number, radius: number): void {
    const cache = this.renderer.getMaterialCache();
    if (!cache) return;

    const worldW = this.renderer.worldW;
    const worldH = this.renderer.worldH;
    const r2 = radius * radius;

    const xMin = Math.max(0, wx - radius);
    const xMax = Math.min(worldW - 1, wx + radius);
    const yMin = Math.max(0, wy - radius);
    const yMax = Math.min(worldH - 1, wy + radius);

    let changed = false;

    switch (power) {
      case 0: // Rain — fill Empty cells with Water
        for (let y = yMin; y <= yMax; y++) {
          for (let x = xMin; x <= xMax; x++) {
            const dx = x - wx;
            const dy = y - wy;
            if (dx * dx + dy * dy > r2) continue;
            const idx = y * worldW + x;
            if (cache[idx] === Mat.Empty) {
              cache[idx] = Mat.Water;
              changed = true;
            }
          }
        }
        break;

      case 1: // Heat — ignite Plants into Fire
        for (let y = yMin; y <= yMax; y++) {
          for (let x = xMin; x <= xMax; x++) {
            const dx = x - wx;
            const dy = y - wy;
            if (dx * dx + dy * dy > r2) continue;
            const idx = y * worldW + x;
            if (cache[idx] === Mat.Plant) {
              cache[idx] = Mat.Fire;
              changed = true;
            }
          }
        }
        break;

      case 2: // Wind — no visual prediction (affects velocity field)
        break;

      case 3: // Growth — grow Plants near Water/Soil
        for (let y = yMin; y <= yMax; y++) {
          for (let x = xMin; x <= xMax; x++) {
            const dx = x - wx;
            const dy = y - wy;
            if (dx * dx + dy * dy > r2) continue;
            const idx = y * worldW + x;
            if (cache[idx] !== Mat.Empty && cache[idx] !== Mat.Soil) continue;
            // Check if any neighbor is Water or Soil
            if (this.hasNeighbor(cache, x, y, worldW, worldH, [Mat.Water, Mat.Soil])) {
              cache[idx] = Mat.Plant;
              changed = true;
            }
          }
        }
        break;

      case 4: // Life — spawn Herbivores in scattered cells
        for (let y = yMin; y <= yMax; y++) {
          for (let x = xMin; x <= xMax; x++) {
            const dx = x - wx;
            const dy = y - wy;
            if (dx * dx + dy * dy > r2) continue;
            const idx = y * worldW + x;
            if (cache[idx] !== Mat.Empty) continue;
            // Sparse spawn: ~5% of empty cells
            if (Math.random() < 0.05) {
              cache[idx] = Mat.Herbivore;
              changed = true;
            }
          }
        }
        break;
    }

    if (changed) {
      this.renderer.drawImmediate();
    }

    // Trigger glow effect on the overlay
    this.effects.triggerGlow(wx, wy, power, radius);
  }

  private hasNeighbor(
    cache: Uint8Array,
    x: number, y: number,
    worldW: number, worldH: number,
    mats: number[],
  ): boolean {
    for (let dy = -1; dy <= 1; dy++) {
      for (let dx = -1; dx <= 1; dx++) {
        if (dx === 0 && dy === 0) continue;
        const nx = x + dx;
        const ny = y + dy;
        if (nx < 0 || nx >= worldW || ny < 0 || ny >= worldH) continue;
        const mat = cache[ny * worldW + nx];
        if (mats.includes(mat)) return true;
      }
    }
    return false;
  }
}
