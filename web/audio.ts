/**
 * audio.ts — Procedural sound effects for WorldWeaver
 *
 * All sounds are generated via Web Audio API (no external audio files).
 * Sounds are subtle and togglable. AudioContext is initialized only after
 * the first user interaction (browser autoplay policy compliance).
 */

// ── Types ────────────────────────────────────────────────────────────────────
type PowerID = 0 | 1 | 2 | 3; // Rain, Heat, Wind, Growth

// ── AudioEngine singleton ────────────────────────────────────────────────────
export class AudioEngine {
  private static instance: AudioEngine | null = null;

  private ctx: AudioContext | null = null;
  private masterGain: GainNode | null = null;
  private muted = false;
  private initialized = false;

  // Ambient state
  private fireNode: { source: AudioBufferSourceNode; gain: GainNode } | null = null;
  private waterNode: { source: AudioBufferSourceNode; gain: GainNode } | null = null;
  private fireTarget = 0;
  private waterTarget = 0;
  private ambientRaf: number | null = null;

  private constructor() {}

  static getInstance(): AudioEngine {
    if (!AudioEngine.instance) {
      AudioEngine.instance = new AudioEngine();
    }
    return AudioEngine.instance;
  }

  /** Call on first user click/interaction to unlock AudioContext. */
  init(): void {
    if (this.initialized) return;
    this.initialized = true;

    try {
      this.ctx = new AudioContext();
      this.masterGain = this.ctx.createGain();
      this.masterGain.gain.value = 1.0;
      this.masterGain.connect(this.ctx.destination);
      console.info("[audio] initialized");
    } catch (e) {
      console.warn("[audio] Web Audio API not available:", e);
    }
  }

  /** Resume context if suspended (e.g. after tab switch). */
  private resume(): void {
    if (this.ctx?.state === "suspended") {
      this.ctx.resume();
    }
  }

  get isMuted(): boolean {
    return this.muted;
  }

  toggleMute(): boolean {
    this.muted = !this.muted;
    if (this.masterGain) {
      this.masterGain.gain.setTargetAtTime(
        this.muted ? 0 : 1.0,
        this.ctx!.currentTime,
        0.05
      );
    }
    return this.muted;
  }

  // ═══════════════════════════════════════════════════════════════════════════
  // POWER APPLICATION SOUNDS
  // ═══════════════════════════════════════════════════════════════════════════

  /** Play a subtle power-application sound (called on each click/drag apply). */
  playPower(power: PowerID): void {
    if (!this.ctx || this.muted) return;
    this.resume();

    switch (power) {
      case 0: this.playRainDrip(); break;
      case 1: this.playHeatCrackle(); break;
      case 2: this.playWindWhoosh(); break;
      case 3: this.playGrowthChime(); break;
    }
  }

  /** Rain: water drip — quick high→low frequency sweep */
  private playRainDrip(): void {
    const ctx = this.ctx!;
    const t = ctx.currentTime;

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    osc.frequency.setValueAtTime(600, t);
    osc.frequency.exponentialRampToValueAtTime(150, t + 0.08);

    gain.gain.setValueAtTime(0.1, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.1);

    osc.connect(gain).connect(this.masterGain!);
    osc.start(t);
    osc.stop(t + 0.12);
  }

  /** Heat: crackle — short burst of shaped noise */
  private playHeatCrackle(): void {
    const ctx = this.ctx!;
    const t = ctx.currentTime;
    const duration = 0.08;

    const bufferSize = Math.ceil(ctx.sampleRate * duration);
    const buffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate);
    const data = buffer.getChannelData(0);

    // Crackle: random pops with decay
    for (let i = 0; i < bufferSize; i++) {
      const env = 1 - i / bufferSize;
      data[i] = (Math.random() * 2 - 1) * env * (Math.random() > 0.7 ? 1 : 0.2);
    }

    const source = ctx.createBufferSource();
    source.buffer = buffer;

    const gain = ctx.createGain();
    gain.gain.setValueAtTime(0.1, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + duration);

    const hp = ctx.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.value = 2000;

    source.connect(hp).connect(gain).connect(this.masterGain!);
    source.start(t);
  }

  /** Wind: filtered noise whoosh */
  private playWindWhoosh(): void {
    const ctx = this.ctx!;
    const t = ctx.currentTime;
    const duration = 0.15;

    const bufferSize = Math.ceil(ctx.sampleRate * duration);
    const buffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate);
    const data = buffer.getChannelData(0);

    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }

    const source = ctx.createBufferSource();
    source.buffer = buffer;

    const gain = ctx.createGain();
    gain.gain.setValueAtTime(0, t);
    gain.gain.linearRampToValueAtTime(0.08, t + 0.03);
    gain.gain.linearRampToValueAtTime(0, t + duration);

    const bp = ctx.createBiquadFilter();
    bp.type = "bandpass";
    bp.frequency.setValueAtTime(800, t);
    bp.frequency.linearRampToValueAtTime(300, t + duration);
    bp.Q.value = 1.5;

    source.connect(bp).connect(gain).connect(this.masterGain!);
    source.start(t);
  }

  /** Growth: soft two-note chime */
  private playGrowthChime(): void {
    const ctx = this.ctx!;
    const t = ctx.currentTime;

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    // C6 → E6 quick arpeggio
    osc.frequency.setValueAtTime(1047, t);
    osc.frequency.setValueAtTime(1319, t + 0.05);

    gain.gain.setValueAtTime(0.08, t);
    gain.gain.setValueAtTime(0.08, t + 0.05);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.15);

    osc.connect(gain).connect(this.masterGain!);
    osc.start(t);
    osc.stop(t + 0.16);
  }

  // ═══════════════════════════════════════════════════════════════════════════
  // AMBIENT SOUNDS (fire crackling, water stream)
  // ═══════════════════════════════════════════════════════════════════════════

  /**
   * Update ambient sound volumes based on visible material counts.
   * Call this every frame or every few frames from the render loop.
   * @param fireRatio  0..1 ratio of visible fire cells
   * @param waterRatio 0..1 ratio of visible water cells
   */
  updateAmbient(fireRatio: number, waterRatio: number): void {
    if (!this.ctx || this.muted) return;

    this.fireTarget = Math.min(1, fireRatio * 10) * 0.03;  // Max 0.03
    this.waterTarget = Math.min(1, waterRatio * 8) * 0.02;  // Max 0.02

    // Start ambient loops if needed
    if (this.fireTarget > 0.001 && !this.fireNode) this.startFireAmbient();
    if (this.waterTarget > 0.001 && !this.waterNode) this.startWaterAmbient();

    // Smooth volume transitions
    if (this.fireNode) {
      this.fireNode.gain.gain.setTargetAtTime(this.fireTarget, this.ctx.currentTime, 0.3);
      if (this.fireTarget < 0.001) {
        this.fireNode.source.stop(this.ctx.currentTime + 0.5);
        this.fireNode = null;
      }
    }
    if (this.waterNode) {
      this.waterNode.gain.gain.setTargetAtTime(this.waterTarget, this.ctx.currentTime, 0.3);
      if (this.waterTarget < 0.001) {
        this.waterNode.source.stop(this.ctx.currentTime + 0.5);
        this.waterNode = null;
      }
    }
  }

  /** Brown noise loop for fire crackling. */
  private startFireAmbient(): void {
    const ctx = this.ctx!;
    const sampleRate = ctx.sampleRate;
    const length = sampleRate * 2; // 2-second loop
    const buffer = ctx.createBuffer(1, length, sampleRate);
    const data = buffer.getChannelData(0);

    // Brown noise: integrated white noise
    let last = 0;
    for (let i = 0; i < length; i++) {
      const white = Math.random() * 2 - 1;
      last = (last + 0.02 * white) / 1.02;
      // Add occasional pops for crackle character
      data[i] = last + (Math.random() > 0.995 ? (Math.random() * 0.3 - 0.15) : 0);
    }

    const source = ctx.createBufferSource();
    source.buffer = buffer;
    source.loop = true;

    const gain = ctx.createGain();
    gain.gain.value = 0;

    const hp = ctx.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.value = 200;

    const lp = ctx.createBiquadFilter();
    lp.type = "lowpass";
    lp.frequency.value = 3000;

    source.connect(hp).connect(lp).connect(gain).connect(this.masterGain!);
    source.start();

    this.fireNode = { source, gain };
  }

  /** Filtered noise for water stream ambient. */
  private startWaterAmbient(): void {
    const ctx = this.ctx!;
    const sampleRate = ctx.sampleRate;
    const length = sampleRate * 2;
    const buffer = ctx.createBuffer(1, length, sampleRate);
    const data = buffer.getChannelData(0);

    for (let i = 0; i < length; i++) {
      data[i] = Math.random() * 2 - 1;
    }

    const source = ctx.createBufferSource();
    source.buffer = buffer;
    source.loop = true;

    const gain = ctx.createGain();
    gain.gain.value = 0;

    const lp = ctx.createBiquadFilter();
    lp.type = "lowpass";
    lp.frequency.value = 600;
    lp.Q.value = 1.0;

    source.connect(lp).connect(gain).connect(this.masterGain!);
    source.start();

    this.waterNode = { source, gain };
  }

  // ═══════════════════════════════════════════════════════════════════════════
  // NOTIFICATION SOUNDS
  // ═══════════════════════════════════════════════════════════════════════════

  /** Player joined: ascending C5→E5 chime */
  playJoin(): void {
    if (!this.ctx || this.muted) return;
    this.resume();

    const ctx = this.ctx;
    const t = ctx.currentTime;

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    // C5 = 523Hz, E5 = 659Hz
    osc.frequency.setValueAtTime(523, t);
    osc.frequency.setValueAtTime(659, t + 0.06);

    gain.gain.setValueAtTime(0.08, t);
    gain.gain.setValueAtTime(0.08, t + 0.06);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.16);

    osc.connect(gain).connect(this.masterGain!);
    osc.start(t);
    osc.stop(t + 0.18);
  }

  /** Player left: descending E5→C5 chime */
  playLeave(): void {
    if (!this.ctx || this.muted) return;
    this.resume();

    const ctx = this.ctx;
    const t = ctx.currentTime;

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    // E5 = 659Hz, C5 = 523Hz
    osc.frequency.setValueAtTime(659, t);
    osc.frequency.setValueAtTime(523, t + 0.06);

    gain.gain.setValueAtTime(0.08, t);
    gain.gain.setValueAtTime(0.06, t + 0.06);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.16);

    osc.connect(gain).connect(this.masterGain!);
    osc.start(t);
    osc.stop(t + 0.18);
  }

  /** Score increase: satisfying 'ding' */
  playScoreDing(): void {
    if (!this.ctx || this.muted) return;
    this.resume();

    const ctx = this.ctx;
    const t = ctx.currentTime;

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = "sine";
    osc.frequency.setValueAtTime(880, t);

    gain.gain.setValueAtTime(0.12, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.12);

    osc.connect(gain).connect(this.masterGain!);
    osc.start(t);
    osc.stop(t + 0.13);
  }

  /** Level up: triumphant ascending arpeggio */
  playLevelUp(): void {
    if (!this.ctx || this.muted) return;
    this.resume();

    const ctx = this.ctx;
    const t = ctx.currentTime;
    const notes = [523.25, 659.25, 783.99, 1046.5]; // C5, E5, G5, C6

    notes.forEach((freq, i) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();

      osc.type = "sine";
      osc.frequency.setValueAtTime(freq, t + i * 0.1);

      gain.gain.setValueAtTime(0, t + i * 0.1);
      gain.gain.linearRampToValueAtTime(0.2, t + i * 0.1 + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.001, t + i * 0.1 + 0.4);

      osc.connect(gain).connect(this.masterGain!);
      osc.start(t + i * 0.1);
      osc.stop(t + i * 0.1 + 0.45);
    });
  }

  /** Clean up all audio nodes. */
  dispose(): void {
    if (this.fireNode) {
      this.fireNode.source.stop();
      this.fireNode = null;
    }
    if (this.waterNode) {
      this.waterNode.source.stop();
      this.waterNode = null;
    }
    if (this.ambientRaf) {
      cancelAnimationFrame(this.ambientRaf);
      this.ambientRaf = null;
    }
    this.ctx?.close();
    this.ctx = null;
    this.initialized = false;
  }
}
