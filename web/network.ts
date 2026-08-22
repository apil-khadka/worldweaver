/**
 * network.ts — WebSocket client for WorldWeaver
 *
 * Handles:
 *  - Connection lifecycle (connect, disconnect, auto-reconnect)
 *  - Inbound message dispatch (snapshot, chunk updates, metrics, player state)
 *  - Outbound message construction (hello, power input, viewport)
 *
 * This module is the only part of the client that communicates with the server.
 * It exposes callbacks so other modules (renderer, UI) can react to server events.
 *
 * Protocol reference: internal/network/protocol.go
 */

import type { ChunkUpdate, FullSnapshot } from "./renderer.js";

/** Renderer interface consumed by WorldNetwork — both Canvas2D and WebGL2 satisfy this. */
export interface IGameRenderer {
  viewX: number;
  viewY: number;
  worldW: number;
  worldH: number;
  chunkSize: number;
  zoom: number;
  readonly visibleW: number;
  readonly visibleH: number;
  initWorld(w: number, h: number): void;
  applySnapshot(snap: FullSnapshot): void;
  applyChunkUpdates(updates: ChunkUpdate[]): void;
  getMaterialCache(): Uint8Array | null;
  drawImmediate(): void;
  onResize(): void;
  stepZoom(direction: 1 | -1): number;
}

const RECONNECT_DELAY_MS = 2000;

export interface MetricsData {
  tps:          number;
  tickP95Ms:    number;
  activeCells:  number;
  activeChunks: number;
  playerCount:  number;
  outboundBPS:  number;
  stability:    number;
}

export interface PlayerState {
  playerID:     number;
  influence:    number;
  maxInfluence: number;
}

export interface RemoteCursor {
  playerID: number;
  x:        number;
  y:        number;
  power:    number;
  lastSeen: number;
}

/** Callbacks that other modules register to receive network events. */
export interface NetworkCallbacks {
  onConnected?():          void;
  onDisconnected?():       void;
  onMetrics?(m: MetricsData): void;
  onPlayerState?(s: PlayerState): void;
  onError?(msg: string):   void;
  onCursorUpdate?(cursor: RemoteCursor): void;
  onPlayerJoin?(playerID: number): void;
  onPlayerLeave?(playerID: number): void;
  onChat?(playerID: number, nickname: string, text: string, x: number, y: number): void;
  onPingLocation?(playerID: number, x: number, y: number): void;
  onEmote?(playerID: number, emote: string, x: number, y: number): void;
  onCombo?(playerIDs: number[], powers: number[], x: number, y: number): void;
}

export class WorldNetwork {
  private ws: WebSocket | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  /** Set by the UI and input handlers. */
  callbacks: NetworkCallbacks = {};

  /** Player state received from server. */
  playerID        = 0;
  worldW          = 0;
  worldH          = 0;
  influence       = 100;
  maxInfluence    = 100;
  activePower     = 0;

  /** RTT tracking */
  private pingSentAt = 0;
  lastRttMs = 0;

  constructor(
    private readonly wsUrl: string,
    private renderer: IGameRenderer,
  ) {}

  /** Hot-swap the renderer backend (used by view mode toggle). */
  swapRenderer(r: IGameRenderer): void {
    this.renderer = r;
  }

  connect(): void {
    if (this.ws) return;
    console.info("[network] connecting to", this.wsUrl);
    this.ws = new WebSocket(this.wsUrl);

    this.ws.binaryType = "arraybuffer";

    this.ws.addEventListener("open",    () => this.onOpen());
    this.ws.addEventListener("message", (e) => this.onMessage(e));
    this.ws.addEventListener("close",   () => this.onClose());
    this.ws.addEventListener("error",   () => this.onClose());
  }

  private onOpen(): void {
    console.info("[network] connected");
    this.send({ type: "hello", viewW: window.innerWidth, viewH: window.innerHeight });
    this.callbacks.onConnected?.();
    this.startPing();
  }

  private onClose(): void {
    console.warn("[network] disconnected — retrying in", RECONNECT_DELAY_MS, "ms");
    this.ws = null;
    this.callbacks.onDisconnected?.();
    this.reconnectTimer = setTimeout(() => this.connect(), RECONNECT_DELAY_MS);
  }

  private onMessage(event: MessageEvent): void {
    let msg: Record<string, unknown>;
    try {
      msg = JSON.parse(event.data as string);
    } catch {
      console.error("[network] non-JSON message received");
      return;
    }

    switch (msg["type"]) {
      case "welcome":
        this.playerID = msg["playerID"] as number;
        this.worldW   = msg["worldW"]   as number;
        this.worldH   = msg["worldH"]   as number;
        this.renderer.initWorld(this.worldW, this.worldH);
        break;

      case "world_snapshot": {
        const raw  = atob(msg["data"] as string);
        const data = new Uint8Array(raw.length);
        for (let i = 0; i < raw.length; i++) data[i] = raw.charCodeAt(i);
        const snap: FullSnapshot = {
          tick: msg["tick"] as number,
          x:    msg["x"]    as number,
          y:    msg["y"]    as number,
          w:    msg["w"]    as number,
          h:    msg["h"]    as number,
          data,
        };
        this.renderer.applySnapshot(snap);
        break;
      }

      case "chunk_update": {
        const rawChunks = msg["chunks"] as Array<{
          cx: number; cy: number; tick: number; data: string;
        }>;
        const updates: ChunkUpdate[] = rawChunks.map((c) => {
          const raw  = atob(c.data);
          const data = new Uint8Array(raw.length);
          for (let i = 0; i < raw.length; i++) data[i] = raw.charCodeAt(i);
          return { cx: c.cx, cy: c.cy, tick: c.tick, data };
        });
        this.renderer.applyChunkUpdates(updates);
        break;
      }

      case "player_state": {
        const s: PlayerState = {
          playerID:     msg["playerID"]     as number,
          influence:    msg["influence"]    as number,
          maxInfluence: msg["maxInfluence"] as number,
        };
        this.influence    = s.influence;
        this.maxInfluence = s.maxInfluence;
        this.callbacks.onPlayerState?.(s);
        break;
      }

      case "world_metrics":
        this.callbacks.onMetrics?.({
          tps:          msg["tps"]          as number,
          tickP95Ms:    msg["tickP95Ms"]    as number,
          activeCells:  msg["activeCells"]  as number,
          activeChunks: msg["activeChunks"] as number,
          playerCount:  msg["playerCount"]  as number,
          outboundBPS:  msg["outboundBPS"]  as number,
          stability:    msg["stability"]    as number,
        });
        break;

      case "pong":
        this.lastRttMs = Date.now() - this.pingSentAt;
        break;

      case "cursor_update": {
        const cursor: RemoteCursor = {
          playerID: msg["playerID"] as number,
          x:        msg["x"] as number,
          y:        msg["y"] as number,
          power:    msg["power"] as number,
          lastSeen: Date.now(),
        };
        this.callbacks.onCursorUpdate?.(cursor);
        break;
      }

      case "player_join":
        this.callbacks.onPlayerJoin?.(msg["playerID"] as number);
        break;

      case "player_leave":
        this.callbacks.onPlayerLeave?.(msg["playerID"] as number);
        break;

      case "chat":
        this.callbacks.onChat?.(
          msg["playerID"] as number,
          msg["nickname"] as string,
          msg["text"] as string,
          msg["x"] as number,
          msg["y"] as number,
        );
        break;

      case "ping_location":
        this.callbacks.onPingLocation?.(
          msg["playerID"] as number,
          msg["x"] as number,
          msg["y"] as number,
        );
        break;

      case "emote":
        this.callbacks.onEmote?.(
          msg["playerID"] as number,
          msg["emote"] as string,
          msg["x"] as number,
          msg["y"] as number,
        );
        break;

      case "combo":
        this.callbacks.onCombo?.(
          msg["playerIDs"] as number[],
          msg["powers"] as number[],
          msg["x"] as number,
          msg["y"] as number,
        );
        break;

      case "error":
        console.warn("[network] server error:", msg["message"]);
        this.callbacks.onError?.(msg["message"] as string);
        break;
    }
  }

  /**
   * Send a power application request to the server.
   * @param power  0=Rain, 1=Heat, 2=Wind, 3=Growth
   * @param x      World X coordinate
   * @param y      World Y coordinate
   * @param radius Influence radius in cells
   */
  sendPower(power: number, x: number, y: number, radius = 24): void {
    this.send({
      type:      "power",
      power,
      x:         Math.round(x),
      y:         Math.round(y),
      radius,
      intensity: 0.8,
    });
  }

  /** Notify the server of a camera move so it streams the right chunks. */
  sendViewport(x: number, y: number, w: number, h: number): void {
    this.send({ type: "viewport", x, y, w, h });
  }

  /** Send cursor position to server for multiplayer presence (throttled externally). */
  sendCursor(x: number, y: number, power: number): void {
    this.send({ type: "cursor", x: Math.round(x), y: Math.round(y), power });
  }

  /** Send a chat message. */
  sendChat(text: string): void {
    this.send({ type: "chat", text: text.slice(0, 50) });
  }

  /** Send a location ping. */
  sendPingLocation(x: number, y: number): void {
    this.send({ type: "ping_location", x: Math.round(x), y: Math.round(y) });
  }

  /** Send an emote. */
  sendEmote(emote: string): void {
    this.send({ type: "emote", emote });
  }

  private send(msg: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private startPing(): void {
    setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.pingSentAt = Date.now();
        this.send({ type: "ping" });
      }
    }, 3000);
  }
}
