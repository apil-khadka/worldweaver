/**
 * Typed access to the world-listing endpoint exposed by the Go server.
 *
 * The landing page is public and unauthenticated, so this module only ever
 * reads. Every value that reaches the UI is normalised here: the server is
 * trusted to be well-behaved, but a stale deployment or a proxy returning HTML
 * must never be able to crash the page or inject a non-string into JSX.
 */

/** Shape of one entry in `GET /api/worlds` (mirrors game.WorldInfo). */
export interface WorldSummary {
  id: string;
  name: string;
  seed: number;
  width: number;
  height: number;
  creatorName: string;
  createdAt: string;
  playerCount: number;
  maxPlayers: number;
  size: string;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

/**
 * Converts one raw JSON entry into a `WorldSummary`, or `null` when the entry
 * is unusable. A world without an id cannot be linked to, so it is dropped
 * rather than rendered as a dead card.
 */
function normalizeWorld(raw: unknown): WorldSummary | null {
  if (typeof raw !== "object" || raw === null) return null;
  const r = raw as Record<string, unknown>;

  const id = asString(r["id"]).trim();
  if (id === "") return null;

  return {
    id,
    name: asString(r["name"]).trim() || "Untitled world",
    seed: asNumber(r["seed"]),
    width: asNumber(r["width"]),
    height: asNumber(r["height"]),
    creatorName: asString(r["creatorName"]).trim() || "anonymous",
    createdAt: asString(r["createdAt"]),
    playerCount: Math.max(0, asNumber(r["playerCount"])),
    maxPlayers: Math.max(0, asNumber(r["maxPlayers"])),
    size: asString(r["size"]).trim(),
  };
}

/**
 * Fetches the public world list.
 *
 * Resolves with an empty array when the server reports no worlds. Rejects when
 * the server is unreachable, returns a non-2xx status, or returns something
 * that is not a JSON array — callers are expected to render an error state
 * rather than an empty one in that case.
 */
export async function fetchWorlds(signal?: AbortSignal): Promise<WorldSummary[]> {
  const resp = await fetch("/api/worlds", {
    signal,
    headers: { Accept: "application/json" },
  });

  if (!resp.ok) {
    throw new Error(`Server responded with ${resp.status}`);
  }

  const payload: unknown = await resp.json();
  if (!Array.isArray(payload)) {
    throw new Error("Unexpected response from the world server");
  }

  return payload
    .map(normalizeWorld)
    .filter((w): w is WorldSummary => w !== null);
}

/** True when a world has no free slots. */
export function isFull(world: WorldSummary): boolean {
  return world.maxPlayers > 0 && world.playerCount >= world.maxPlayers;
}

/** Deep link into the client for a specific world. */
export function playUrl(worldId: string): string {
  return `/play?world=${encodeURIComponent(worldId)}`;
}
