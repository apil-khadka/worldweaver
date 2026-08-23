# Identity & Shared Worlds — Requirements

## Motto

**Collaboration is the point.** Every decision here is judged by whether it helps
a group of people shape a world together: getting in without friction, finding
each other, controlling who joins, and seeing each other's contribution.

## Measured starting point

An audit of the current system found:

| Area | Actual state |
|------|--------------|
| `POST /api/login` | Accepts any nickname, returns a token. No credential of any kind. |
| Authorization in the codebase | One check: a nickname string comparison in `DeleteWorld`. |
| `GET /ws` | An invalid or absent token is silently ignored; the connection proceeds. |
| WebSocket origin | `InsecureSkipVerify: true`, so any origin may connect. |
| `POST /api/worlds` | Tolerates a nil session; anonymous callers own worlds as `"anonymous"`. |
| `ListWorlds()` | Returns every world to every caller. |
| `WorldInfo` | No owner, visibility, invite code or member list. |
| Worlds | Metadata only. One shared `world.World`; `Client.WorldID` is never read. |
| Persistence | Terrain only. Players, sessions, worlds and scores are lost on restart. |

Two players who "join different worlds" today edit the same cells and see each
other's chat.

## Identity

Modelled on device-key identity: the client generates a keypair on first run, the
public key is the durable identity, and nicknames are cosmetic and changeable.
There is no password to create, remember or reset.

### REQ-ID-001: Keypair Identity
A client SHALL generate an Ed25519 keypair on first run and keep the private key
in local storage. The public key SHALL be the player's durable identity.

**Acceptance:** Clearing site data produces a new identity; keeping it preserves
the same identity across reloads and server restarts.

### REQ-ID-002: Challenge–Response Login
Login SHALL require proving possession of the private key. The server issues a
random challenge; the client returns a signature over it; the server verifies the
signature against the presented public key.

**Acceptance:** A request with a valid public key but an incorrect or replayed
signature is rejected with 401.

### REQ-ID-003: Single-Use Challenges
A challenge SHALL be valid once and SHALL expire. Replaying a captured
challenge-and-signature pair SHALL fail.

**Acceptance:** Submitting the same challenge and signature twice succeeds once
and then fails.

### REQ-ID-004: Stable Player Identity
A player's ID SHALL be derived from their public key so it is stable across
restarts. IDs SHALL NOT be reused between different keys.

**Acceptance:** Restarting the server and logging in with the same key yields the
same player ID; a different key yields a different ID.

### REQ-ID-005: Changeable Display Name
A player SHALL be able to change their display name at any time without
affecting their identity, ownership or progress.

**Acceptance:** Renaming preserves owned worlds and accumulated score.

### REQ-ID-006: No Anonymous Writes
Every request that mutates state — creating or deleting a world, connecting to
the socket, applying a power — SHALL require a verified session. Unauthenticated
callers SHALL be able to read public information only.

**Acceptance:** Each mutating endpoint returns 401 without a valid token.

### REQ-ID-007: Session Expiry
Sessions SHALL expire after a period of inactivity and expired sessions SHALL be
pruned, so the session store cannot grow without bound.

**Acceptance:** A session unused past its lifetime is rejected and removed.

### REQ-ID-008: Origin Checking
The WebSocket handshake SHALL verify the request origin against a configured
allow-list rather than skipping verification.

**Acceptance:** A handshake from a disallowed origin is refused.

## Worlds

### REQ-WLD-001: Visibility
A world SHALL be **public**, **unlisted** or **private**:

| Visibility | Appears in the public list | Who may join |
|------------|---------------------------|--------------|
| public | yes | anyone signed in |
| unlisted | no | anyone with the invite code |
| private | no | invited members only |

**Acceptance:** Listing as a non-member returns public worlds only.

### REQ-WLD-002: Ownership By Key
A world SHALL record its owner's public key. Only the owner may change settings,
issue invites or delete it.

**Acceptance:** A second player, whatever their display name, cannot delete a
world they do not own.

### REQ-WLD-003: Invite Codes
An owner SHALL be able to issue invite codes. Redeeming a code grants
membership. Codes SHALL support an optional use limit and expiry.

**Acceptance:** A code past its use limit or expiry is refused; a valid code adds
the redeemer as a member.

### REQ-WLD-004: Real Isolation
Players in different worlds SHALL NOT share simulation state, chat, cursors or
goals. Each world SHALL own its own simulation.

**Acceptance:** A power applied in world A leaves world B unchanged, and chat in
A does not reach B.

### REQ-WLD-005: Durable Worlds
Worlds, their membership and their owners SHALL survive a server restart.

**Acceptance:** A private world with two members is still present, still owned by
the same key, and still restricted after a restart.

### REQ-WLD-006: Capacity
A world SHALL enforce its player cap at connect time for every world, not only
the default one.

**Acceptance:** The cap is enforced on a created world, and its reported player
count reflects actual connections.

## Collaboration

### REQ-CLB-001: Contribution Tracking
Each world SHALL track what each member contributed — cells shaped, creatures
introduced, fires extinguished, goals helped complete.

**Acceptance:** Two players acting differently produce different contribution
breakdowns.

### REQ-CLB-002: Shared Goals Credit Everyone
Completing a cooperative goal SHALL reward every member who contributed, in
proportion to their contribution, rather than only the player who finished it.

**Acceptance:** A goal completed after two players contribute rewards both.

### REQ-CLB-003: Presence Roster
A world SHALL expose who is currently present, with display name and identity
colour, so members can see who they are working with.

**Acceptance:** The roster reflects joins and departures.

### REQ-CLB-004: Joint Actions
Some outcomes SHALL require more than one player acting together within a window,
and SHALL be recognisably rewarded when they do.

**Acceptance:** A joint action performed alone does not trigger; performed by two
players it does.

## Out of scope

- Federation or peer-to-peer transport; the server stays authoritative
- Email, phone numbers, OAuth or password recovery
- Payments or entitlements
- Moderation tooling beyond owner-level kick and ban

## References
- BitChat whitepaper — peers identified by cryptographic keys, signed announcements
- `.kiro/specs/god-sandbox-pivot/` — the 2D god-sandbox direction
- `.kiro/specs/world-generation-ecosystem/` — the simulation this collaborates on
