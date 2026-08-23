package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/world"
)

// NewRouter builds and returns the chi HTTP router.
//
// Routes:
//
//	GET    /              → serves index.html (static)
//	GET    /ws            → WebSocket upgrade (?token=...&world=...)
//	POST   /api/login     → login with nickname
//	GET    /api/worlds    → list available worlds
//	POST   /api/worlds    → create a new world
//	DELETE /api/worlds/{id} → delete a world (creator only)
//	GET    /api/metrics   → JSON snapshot of current metrics
//	GET    /api/scores    → Top 10 player scores per world
func NewRouter(hub *Hub, w *world.World, m *metrics.Metrics, staticFS http.FileSystem) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static frontend — serves built files from web/dist/
	//
	// Vite emits content-hashed filenames under /assets, so those are safe to
	// cache indefinitely. HTML entry points must never be cached or clients
	// would keep loading stale bundles after a deploy.
	fileServer := http.FileServer(staticFS)
	r.Handle("/*", staticCacheHeaders(fileServer))

	// SPA-style route: /play serves play.html (landing page links here)
	r.Get("/play", func(w http.ResponseWriter, r *http.Request) {
		f, err := staticFS.Open("play.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			http.Error(w, "cannot stat play.html", http.StatusInternalServerError)
			return
		}
		seeker, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "play.html is not seekable", http.StatusInternalServerError)
			return
		}
		// HTML shells must always revalidate so a new build is picked up.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "play.html", stat.ModTime(), seeker)
	})

	// API info endpoint (moved from / to /api so index.html can be served at root)
	r.Get("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"name":    "WorldWeaver API",
			"version": "0.2.0",
			"status":  "running",
			"docs":    "Multiplayer emergent world simulation server",
			"endpoints": map[string]string{
				"ws":      "/ws",
				"health":  "/health",
				"metrics": "/api/metrics",
				"login":   "/api/login",
				"worlds":  "/api/worlds",
				"scores":  "/api/scores",
			},
		})
	})

	// ── Auth ─────────────────────────────────────────────────────────────────
	// Two-step keypair handshake: request a nonce, return a signature over it.
	r.Post("/api/challenge", hub.handleChallenge)
	r.Post("/api/login", hub.handleLogin)
	r.Post("/api/logout", hub.handleLogout)
	r.Post("/api/rename", hub.handleRename)
	r.Get("/api/session", hub.handleSession)

	// ── Worlds ───────────────────────────────────────────────────────────────
	// Presets are served rather than duplicated in the client, so the lobby and
	// the server cannot disagree about what sizes exist.
	r.Get("/api/world-sizes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(game.WorldPresets)
	})
	r.Get("/api/worlds", hub.handleListWorlds)
	r.Post("/api/worlds", hub.handleCreateWorld)
	r.Delete("/api/worlds/{id}", hub.handleDeleteWorld)
	r.Put("/api/worlds/{id}/visibility", hub.handleSetVisibility)

	// ── Invites ──────────────────────────────────────────────────────────────
	// Sharing a world is a code you hand to someone, not a permission list you
	// administer: that is what makes bringing a friend in a single step.
	r.Post("/api/invites", hub.handleCreateInvite)
	r.Post("/api/invites/redeem", hub.handleRedeemInvite)
	r.Get("/api/worlds/{id}/invites", hub.handleListInvites)
	r.Delete("/api/invites/{code}", hub.handleRevokeInvite)

	// WebSocket endpoint
	r.Get("/ws", hub.handleWebSocket)

	// Health check — used by Docker/Dokploy for readiness
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Metrics API — JSON, no auth required for hackathon scope
	r.Get("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap := m.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(snap)
	})

	// Scores API — top 10 players per world
	r.Get("/api/scores", game.ScoresAPIHandler(hub.Scoreboard))

	// Collaboration API — who built this world, who they built it with, and how
	// far the population has carried its shared milestone.
	r.Get("/api/contributions", hub.handleContributions)

	return r
}

// ── Auth Handlers ────────────────────────────────────────────────────────────

// writeJSON sends a JSON body with the given status.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// writeError sends a JSON error body.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// bearerToken extracts the session token from a request.
//
// The header is the only supported location. An earlier version also accepted
// ?token=, which wrote credentials into access logs and browser history.
func bearerToken(r *http.Request) string {
	tok := r.Header.Get("Authorization")
	return strings.TrimPrefix(tok, "Bearer ")
}

// requireSession resolves the caller's session, or writes 401 and returns nil.
func (h *Hub) requireSession(w http.ResponseWriter, r *http.Request) *game.AuthSession {
	sess := h.auth.Validate(bearerToken(r))
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return nil
	}
	return sess
}

// handleChallenge issues a nonce for a public key to sign.
//
// This is the first half of the handshake that replaced nickname-only login.
// Requesting a challenge proves nothing and grants nothing, so it needs no auth.
func (h *Hub) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"publicKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	_, keyID, err := game.DecodePublicKey(req.PublicKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	nonce, expires, err := h.auth.NewChallenge(keyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue a challenge")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"challenge": nonce,
		"expiresAt": expires.UTC(),
	})
}

// handleLogin completes the handshake: a signature over the outstanding
// challenge proves possession of the private key.
func (h *Hub) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"publicKey"`
		Signature string `json:"signature"`
		Nickname  string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	sess, err := h.auth.VerifyAndLogin(req.PublicKey, req.Signature, req.Nickname)
	if err != nil {
		// Deliberately uniform: distinguishing "no such challenge" from "bad
		// signature" would tell a prober which keys have login attempts pending.
		log.Printf("login rejected: %v", err)
		writeError(w, http.StatusUnauthorized, "could not verify that key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":    sess.Token,
		"playerID": sess.PlayerID,
		"nickname": sess.Nickname,
		"keyId":    sess.KeyID,
	})
}

// handleLogout ends a session. Previously there was no route for this at all, so
// a token stayed valid until the process exited.
func (h *Hub) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.auth.Logout(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "signed out"})
}

// handleRename changes the caller's display name without touching their identity.
func (h *Hub) handleRename(w http.ResponseWriter, r *http.Request) {
	if h.requireSession(w, r) == nil {
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	sess, err := h.auth.Rename(bearerToken(r), req.Nickname)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"playerID": sess.PlayerID,
		"nickname": sess.Nickname,
	})
}

// staticCacheHeaders sets Cache-Control appropriate to the asset being served.
//
// Fingerprinted bundles under /assets never change for a given URL, so they get
// a long immutable TTL. Everything else (HTML shells in particular) is revalidated
// on each load so a new deploy is picked up immediately.
func staticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case path == "/" || strings.HasSuffix(path, ".html"):
			w.Header().Set("Cache-Control", "no-cache")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}

// handleSession reports whether a bearer token is still valid.
//
// Sessions live in memory, so a server restart invalidates every token. The
// client uses this endpoint on load to decide between resuming silently and
// re-authenticating, instead of always forcing the player back to the lobby.
func (h *Hub) handleSession(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"playerID": sess.PlayerID,
		"nickname": sess.Nickname,
		"keyId":    sess.KeyID,
	})
}

// ── World Handlers ───────────────────────────────────────────────────────────

// worldListing is a world as the lobby sees it: the metadata plus the caller's
// relationship to it, so the client does not have to guess which controls to show.
type worldListing struct {
	game.WorldInfo
	Visibility game.Visibility `json:"visibility"`
	Owned      bool            `json:"owned"`
}

// handleListWorlds returns the worlds this caller is allowed to see.
//
// Listing is filtered per caller: unlisted and private worlds are omitted unless
// the caller owns them or has been invited. Previously every world was returned
// to everybody, so a "private" world was private in name only — its ID was on
// the front page for anyone to connect to.
//
// The endpoint stays readable without a session so the landing page can show the
// public worlds to a visitor who has not signed in yet.
func (h *Hub) handleListWorlds(w http.ResponseWriter, r *http.Request) {
	// Update player counts from live hub
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()
	h.worldMgr.SetPlayerCount("genesis", count)

	keyID := ""
	if sess := h.auth.Validate(bearerToken(r)); sess != nil {
		keyID = sess.KeyID
	}

	out := make([]worldListing, 0)
	for _, info := range h.worldMgr.ListWorlds() {
		if !h.Access.Visible(info.ID, keyID) {
			continue
		}
		_, vis, ok := h.Access.Get(info.ID)
		if !ok {
			vis = game.VisibilityPublic
		}
		out = append(out, worldListing{
			WorldInfo:  info,
			Visibility: vis,
			Owned:      keyID != "" && h.Access.IsOwner(info.ID, keyID),
		})
	}

	writeJSON(w, http.StatusOK, out)
}

// handleCreateWorld creates a world owned by the caller's key.
//
// A verified session is now required. Previously a nil session was tolerated and
// the world was recorded as owned by the literal name "anonymous", which meant
// any other anonymous caller satisfied the ownership check and could delete it.
func (h *Hub) handleCreateWorld(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	var req struct {
		Name       string `json:"name"`
		Seed       int64  `json:"seed"`
		MaxPlayers int    `json:"maxPlayers"`
		Size       string `json:"size"`
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if game.SanitiseNickname(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.MaxPlayers < 1 || req.MaxPlayers > game.MaxPlayers {
		req.MaxPlayers = game.MaxPlayers
	}

	info, err := h.worldMgr.CreateWorld(req.Name, req.Seed, req.MaxPlayers, sess.Nickname, req.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Ownership is recorded against the key, not the display name.
	vis := game.ParseVisibility(req.Visibility)
	h.Access.Register(info.ID, sess.KeyID, vis)

	writeJSON(w, http.StatusCreated, map[string]any{
		"world":      info,
		"visibility": vis,
		"owned":      true,
	})
}

// handleDeleteWorld removes a world the caller owns.
func (h *Hub) handleDeleteWorld(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}
	id := chi.URLParam(r, "id")

	if id == "genesis" {
		writeError(w, http.StatusForbidden, "the founding world cannot be deleted")
		return
	}
	if !h.Access.IsOwner(id, sess.KeyID) {
		// Same response whether the world is missing or owned by someone else, so
		// this cannot be used to discover which private world IDs exist.
		writeError(w, http.StatusForbidden, "only the owner can delete this world")
		return
	}

	if err := h.worldMgr.DeleteWorldByOwner(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.ForgetWorldState(id)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleContributions reports who has built a world and who they built it with.
//
// Readable without a session for public worlds so the landing page can show a
// world's collaborators, but a hidden world's roster is only for the people who
// can reach the world itself — the roster would otherwise leak both its existence
// and who is inside it.
func (h *Hub) handleContributions(w http.ResponseWriter, r *http.Request) {
	worldID := r.URL.Query().Get("world")
	if worldID == "" {
		worldID = h.WorldName
	}

	keyID := ""
	if sess := h.auth.Validate(bearerToken(r)); sess != nil {
		keyID = sess.KeyID
	}
	if !h.Access.Visible(worldID, keyID) {
		writeError(w, http.StatusNotFound, "world not found")
		return
	}

	writeJSON(w, http.StatusOK, h.CollaborationFor(worldID))
}

// ── Invites and membership ───────────────────────────────────────────────────

// handleCreateInvite issues an invite code for a world the caller owns.
func (h *Hub) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	var req struct {
		WorldID   string `json:"worldId"`
		MaxUses   int    `json:"maxUses"`
		ExpiresIn string `json:"expiresIn"` // Go duration string, e.g. "24h"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	var ttl time.Duration
	if req.ExpiresIn != "" {
		parsed, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expiresIn must be a duration such as 24h")
			return
		}
		ttl = parsed
	}

	inv, err := h.Access.CreateInvite(req.WorldID, sess.KeyID, req.MaxUses, ttl)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

// handleRedeemInvite exchanges a code for membership of the world behind it.
func (h *Hub) handleRedeemInvite(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	worldID, err := h.Access.RedeemInvite(req.Code, sess.KeyID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info := h.worldMgr.GetWorld(worldID)
	writeJSON(w, http.StatusOK, map[string]any{
		"worldId": worldID,
		"world":   info,
	})
}

// handleListInvites returns the live codes for a world the caller owns.
func (h *Hub) handleListInvites(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	invites, err := h.Access.ListInvites(chi.URLParam(r, "id"), sess.KeyID)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

// handleRevokeInvite deletes a code.
func (h *Hub) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}
	if err := h.Access.RevokeInvite(chi.URLParam(r, "code"), sess.KeyID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// handleSetVisibility changes who can find and join a world.
func (h *Hub) handleSetVisibility(w http.ResponseWriter, r *http.Request) {
	sess := h.requireSession(w, r)
	if sess == nil {
		return
	}

	var req struct {
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	vis := game.ParseVisibility(req.Visibility)
	if err := h.Access.SetVisibility(chi.URLParam(r, "id"), sess.KeyID, vis); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"visibility": vis})
}

// ── WebSocket Handler ────────────────────────────────────────────────────────

// acceptOptions builds the upgrade options, enforcing an origin check.
//
// The library rejects a cross-origin handshake by default, comparing Origin
// against the request Host. The previous code set InsecureSkipVerify, which
// disabled that check entirely and let any page on the internet open an
// authenticated socket using the visitor's session — a cross-site WebSocket
// hijack. Additional origins for split frontend/backend deployments come from
// Hub.AllowedOrigins instead.
func (h *Hub) acceptOptions() *websocket.AcceptOptions {
	return &websocket.AcceptOptions{
		OriginPatterns: h.AllowedOrigins,
	}
}

// handleWebSocket upgrades an HTTP connection to WebSocket and registers
// the resulting client with the hub.
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Connection rate limit: max 20 new connections/minute per IP.
	ip := r.RemoteAddr
	if realIP := r.Header.Get("X-Real-Ip"); realIP != "" {
		ip = realIP
	}
	if !h.connLimiter.AllowConnection(ip) {
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}

	// The token travels in the query string here and only here. Browsers cannot
	// set headers on a WebSocket handshake, so there is nowhere else to put it.
	token := r.URL.Query().Get("token")
	worldID := r.URL.Query().Get("world")
	if worldID == "" {
		worldID = "genesis"
	}

	// A verified session is required to connect. Previously an absent or invalid
	// token was ignored and the connection proceeded as an anonymous player, which
	// made every access check below trivially bypassable: no token, no key, no
	// membership test to fail.
	sess := h.auth.Validate(token)
	if sess == nil {
		http.Error(w, `{"error":"sign in first"}`, http.StatusUnauthorized)
		return
	}

	// Validate world exists
	if !h.worldMgr.Exists(worldID) {
		http.Error(w, `{"error":"world not found"}`, http.StatusNotFound)
		return
	}

	// Private worlds admit their owner and invited members only. Refused before
	// the upgrade so a rejected player gets a real status code rather than a
	// socket that opens and immediately closes.
	if !h.Access.CanJoin(worldID, sess.KeyID) {
		http.Error(w, `{"error":"you need an invite to enter this world"}`, http.StatusForbidden)
		return
	}

	// Check player cap — reject if world is full
	if h.worldMgr.IsFull(worldID) {
		maxP := h.worldMgr.GetMaxPlayers(worldID)
		conn, err := websocket.Accept(w, r, h.acceptOptions())
		if err != nil {
			return
		}
		msg := fmt.Sprintf(`{"type":"error","message":"World full (%d/%d players)"}`, maxP, maxP)
		conn.Write(r.Context(), websocket.MessageText, []byte(msg))
		conn.Close(4001, "world full")
		return
	}

	conn, err := websocket.Accept(w, r, h.acceptOptions())
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}

	c := newClientWithIdentity(h, conn, sess.PlayerID, sess.Nickname, worldID)

	h.register(c)

	log.Printf("player %d (%s) connected to world %q from %s",
		c.Player.ID, c.Player.Nickname, worldID, r.RemoteAddr)

	// Send welcome with nickname
	c.sendJSON(WelcomeMsg{
		Type:     MsgWelcome,
		PlayerID: c.Player.ID,
		WorldW:   h.world.Width,
		WorldH:   h.world.Height,
		Tick:     h.world.Tick,
	})

	// The snapshot is deliberately not sent here. The client's visible region is
	// still unknown at this point (its HELLO/VIEWPORT message is handled by the
	// read pump below), so a snapshot sent now would cover a 0x0 area and the
	// client would have nothing to render. The read pump sends it as soon as the
	// viewport is reported.

	ctx := r.Context()
	go c.writePump(ctx)
	c.readPump(ctx) // blocks until disconnect
}
