/**
 * constants.ts — Canonical protocol and domain constants.
 * Material IDs MUST match internal/systems/materials/registry.go
 * Message types MUST match internal/network/protocol.go
 * Power IDs MUST match internal/game/influence.go
 */
export const MatID = {
  Empty: 0, Rock: 1, Soil: 2, Sand: 3, Water: 4,
  Plant: 5, Fire: 6, Vapor: 7, Smoke: 8,
  Lava: 9, Ice: 10, Ash: 11, Oil: 12, Ember: 13,
} as const;

export const MsgType = {
  Hello: "hello", PowerInput: "power", Viewport: "viewport", Ping: "ping",
  Welcome: "welcome", WorldSnapshot: "world_snapshot",
  ChunkUpdate: "chunk_update", PlayerState: "player_state",
  WorldMetrics: "world_metrics", Error: "error", Pong: "pong",
} as const;

export const PowerID = { Rain: 0, Heat: 1, Wind: 2, Growth: 3 } as const;

export const PowerMeta = [
  { id: 0, name: "Rain",   icon: "\u{1F327}", shortcut: "1", costPerSec: 2 },
  { id: 1, name: "Heat",   icon: "\u{1F525}", shortcut: "2", costPerSec: 3 },
  { id: 2, name: "Wind",   icon: "\u{1F4A8}", shortcut: "3", costPerSec: 1 },
  { id: 3, name: "Growth", icon: "\u{1F331}", shortcut: "4", costPerSec: 4 },
] as const;

export const PROTOCOL_VERSION = 1;
export const CHUNK_SIZE = 64;
