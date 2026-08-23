/**
 * identity.ts — keypair identity for WorldWeaver.
 *
 * There is no sign-up form and no password. The browser generates an Ed25519
 * keypair the first time you open the game, keeps the private key locally, and
 * proves who you are by signing a nonce the server hands out. The public key is
 * the identity: it is what world ownership is recorded against, so it survives a
 * server restart and cannot be claimed by someone who simply picks your name.
 *
 * What replaced what: login used to POST a nickname and get a token back, with
 * no verification of any kind. Ownership was then a nickname comparison, so
 * taking over another player's worlds needed nothing more than typing their name.
 *
 * # Losing the key
 *
 * The key never leaves this device, and the server has no way to reissue it, so
 * clearing site data means losing the identity and everything owned by it. That
 * is why `exportIdentity` exists: the player can copy their key somewhere safe
 * and restore it here or on another machine.
 */

const STORAGE_KEY = "ww_identity_v1";

/** Bytes of an Ed25519 public key. */
const PUBLIC_KEY_BYTES = 32;

export interface Identity {
  /** base64url public key — the durable identity, safe to publish. */
  publicKey: string;

  /** Short fingerprint for display. Never used for authorization. */
  fingerprint: string;

  privateKey: CryptoKey;
}

export interface Session {
  token: string;
  playerID: number;
  nickname: string;
  keyId: string;
}

/** Raised when the browser has no Ed25519 implementation. */
export class UnsupportedCryptoError extends Error {
  constructor() {
    super("This browser cannot do Ed25519 signatures. Please update it, or use Chrome 137+, Safari 17+ or Firefox 130+.");
    this.name = "UnsupportedCryptoError";
  }
}

interface StoredIdentity {
  publicKey: string;
  privateJwk: JsonWebKey;
}

let cached: Identity | null = null;

// ── Encoding helpers ────────────────────────────────────────────────────────
//
// The wire format is base64url without padding, matching Go's
// base64.RawURLEncoding on the server. Standard base64 would not round-trip:
// '+' and '/' are not URL-safe and '=' padding is rejected by the decoder.

function toBase64Url(bytes: Uint8Array): string {
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function fromBase64Url(text: string): Uint8Array {
  const padded = text.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i);
  return out;
}

/** Reports whether this browser can generate and use Ed25519 keys. */
export async function cryptoAvailable(): Promise<boolean> {
  if (!globalThis.crypto?.subtle) return false;
  try {
    await crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]);
    return true;
  } catch {
    return false;
  }
}

/**
 * Returns this device's identity, generating one on first use.
 *
 * Keys are exportable on purpose. A non-extractable key would be safer against
 * script-level theft, but it could never be backed up, and an identity that
 * cannot be backed up means one cleared browser profile destroys every world the
 * player owns — with no account and no email, nobody can restore it for them.
 */
export async function loadIdentity(): Promise<Identity> {
  if (cached) return cached;

  const stored = readStored();
  if (stored) {
    try {
      cached = await importStored(stored);
      return cached;
    } catch (err) {
      // A key we cannot import is worse than none: it would fail every login
      // silently. Discard it and start again rather than wedging the player out.
      console.warn("stored identity unusable, generating a new one:", err);
      localStorage.removeItem(STORAGE_KEY);
    }
  }

  cached = await generateIdentity();
  return cached;
}

/** Discards the local identity. The next load generates a fresh one. */
export function forgetIdentity(): void {
  localStorage.removeItem(STORAGE_KEY);
  cached = null;
}

/**
 * Serialises the identity so the player can keep a copy.
 *
 * This is the private key. Anyone holding it is the player, so the UI presenting
 * it has to say so plainly.
 */
export function exportIdentity(): string | null {
  return localStorage.getItem(STORAGE_KEY);
}

/**
 * Restores an identity previously produced by exportIdentity.
 *
 * The blob is validated by importing it before it is stored, so a mistyped paste
 * cannot overwrite a working identity with something unusable.
 */
export async function importIdentity(blob: string): Promise<Identity> {
  let parsed: StoredIdentity;
  try {
    parsed = JSON.parse(blob) as StoredIdentity;
  } catch {
    throw new Error("That does not look like an identity key.");
  }
  if (!parsed?.publicKey || !parsed?.privateJwk) {
    throw new Error("That identity key is missing part of the pair.");
  }
  if (fromBase64Url(parsed.publicKey).length !== PUBLIC_KEY_BYTES) {
    throw new Error("That public key is the wrong length.");
  }

  const identity = await importStored(parsed);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(parsed));
  cached = identity;
  return identity;
}

async function generateIdentity(): Promise<Identity> {
  let pair: CryptoKeyPair;
  try {
    pair = (await crypto.subtle.generateKey({ name: "Ed25519" }, true, [
      "sign",
      "verify",
    ])) as CryptoKeyPair;
  } catch {
    throw new UnsupportedCryptoError();
  }

  const rawPublic = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
  const privateJwk = await crypto.subtle.exportKey("jwk", pair.privateKey);
  const publicKey = toBase64Url(rawPublic);

  localStorage.setItem(STORAGE_KEY, JSON.stringify({ publicKey, privateJwk }));

  return {
    publicKey,
    fingerprint: await fingerprintOf(rawPublic),
    privateKey: pair.privateKey,
  };
}

function readStored(): StoredIdentity | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as StoredIdentity;
    return parsed?.publicKey && parsed?.privateJwk ? parsed : null;
  } catch {
    return null;
  }
}

async function importStored(stored: StoredIdentity): Promise<Identity> {
  const privateKey = await crypto.subtle.importKey(
    "jwk",
    stored.privateJwk,
    { name: "Ed25519" },
    true,
    ["sign"],
  );
  return {
    publicKey: stored.publicKey,
    fingerprint: await fingerprintOf(fromBase64Url(stored.publicKey)),
    privateKey,
  };
}

/** fingerprintOf mirrors the server's Fingerprint: the first 4 bytes of SHA-256. */
async function fingerprintOf(rawPublic: Uint8Array): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", rawPublic as BufferSource));
  return Array.from(digest.slice(0, 4))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

// ── Handshake ───────────────────────────────────────────────────────────────

/**
 * Signs in: asks for a challenge, signs it, and exchanges the signature for a
 * session token.
 *
 * Two round-trips rather than one, because a signature is only proof if the
 * server chose what was signed. A client-generated token would be replayable by
 * anyone who saw it once.
 */
export async function signIn(nickname: string): Promise<Session> {
  const identity = await loadIdentity();

  const challengeResp = await fetch("/api/challenge", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ publicKey: identity.publicKey }),
  });
  if (!challengeResp.ok) {
    throw new Error(await errorText(challengeResp, "Could not start sign-in."));
  }
  const { challenge } = (await challengeResp.json()) as { challenge: string };
  if (!challenge) throw new Error("The server sent an empty challenge.");

  const signature = new Uint8Array(
    await crypto.subtle.sign(
      { name: "Ed25519" },
      identity.privateKey,
      fromBase64Url(challenge) as BufferSource,
    ),
  );

  const loginResp = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      publicKey: identity.publicKey,
      signature: toBase64Url(signature),
      nickname,
    }),
  });
  if (!loginResp.ok) {
    throw new Error(await errorText(loginResp, "Sign-in was refused."));
  }
  return (await loginResp.json()) as Session;
}

/** Confirms a stored token is still live, returning the session it belongs to. */
export async function resumeSession(token: string): Promise<Session | null> {
  try {
    const resp = await fetch("/api/session", { headers: authHeader(token) });
    if (!resp.ok) return null;
    const data = (await resp.json()) as Session;
    return { ...data, token };
  } catch {
    return null;
  }
}

/** Ends the session server-side so the token cannot be reused. */
export async function signOut(token: string): Promise<void> {
  if (!token) return;
  try {
    await fetch("/api/logout", { method: "POST", headers: authHeader(token) });
  } catch {
    // A failed logout is not worth blocking the UI over; the token expires anyway.
  }
}

/** Changes the display name. The identity behind it is unaffected. */
export async function rename(token: string, nickname: string): Promise<string> {
  const resp = await fetch("/api/rename", {
    method: "POST",
    headers: { "Content-Type": "application/json", ...authHeader(token) },
    body: JSON.stringify({ nickname }),
  });
  if (!resp.ok) throw new Error(await errorText(resp, "Could not change your name."));
  const data = (await resp.json()) as { nickname: string };
  return data.nickname;
}

/** authHeader builds the bearer header used by every authenticated call. */
export function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

/** errorText prefers the server's message, falling back to a readable default. */
async function errorText(resp: Response, fallback: string): Promise<string> {
  try {
    const body = (await resp.json()) as { error?: string };
    return body.error || fallback;
  } catch {
    return fallback;
  }
}
