package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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
	r.Post("/api/login", hub.handleLogin)
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

	return r
}

// ── Auth Handler ─────────────────────────────────────────────────────────────

func (h *Hub) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Nickname == "" {
		req.Nickname = "Anonymous"
	}
	if len(req.Nickname) > 20 {
		req.Nickname = req.Nickname[:20]
	}

	sess := h.auth.Login(req.Nickname)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":    sess.Token,
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
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	sess := h.auth.Validate(token)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"playerID": sess.PlayerID,
		"nickname": sess.Nickname,
	})
}

// ── World Handlers ───────────────────────────────────────────────────────────

func (h *Hub) handleListWorlds(w http.ResponseWriter, r *http.Request) {
	// Update player counts from live hub
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()
	h.worldMgr.SetPlayerCount("genesis", count)

	worlds := h.worldMgr.ListWorlds()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(worlds)
}

func (h *Hub) handleCreateWorld(w http.ResponseWriter, r *http.Request) {
	// Auth check (optional — anonymous can create for hackathon)
	token := r.Header.Get("Authorization")
	sess := h.auth.Validate(token)
	creatorName := "anonymous"
	if sess != nil {
		creatorName = sess.Nickname
	}

	var req struct {
		Name       string `json:"name"`
		Seed       int64  `json:"seed"`
		MaxPlayers int    `json:"maxPlayers"`
		Size       string `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if req.MaxPlayers < 1 || req.MaxPlayers > game.MaxPlayers {
		req.MaxPlayers = game.MaxPlayers
	}

	info, err := h.worldMgr.CreateWorld(req.Name, req.Seed, req.MaxPlayers, creatorName, req.Size)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(info)
}

func (h *Hub) handleDeleteWorld(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Auth check — required for deletion
	token := r.Header.Get("Authorization")
	sess := h.auth.Validate(token)
	if sess == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := h.worldMgr.DeleteWorld(id, sess.Nickname); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "only the creator can delete this world" {
			status = http.StatusForbidden
		}
		http.Error(w, `{"error":"`+err.Error()+`"}`, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ── WebSocket Handler ────────────────────────────────────────────────────────

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

	// Extract token and world from query params
	token := r.URL.Query().Get("token")
	worldID := r.URL.Query().Get("world")
	if worldID == "" {
		worldID = "genesis"
	}

	// Validate world exists
	if !h.worldMgr.Exists(worldID) {
		http.Error(w, `{"error":"world not found"}`, http.StatusNotFound)
		return
	}

	// Check player cap — reject if world is full
	if h.worldMgr.IsFull(worldID) {
		maxP := h.worldMgr.GetMaxPlayers(worldID)
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		msg := fmt.Sprintf(`{"type":"error","message":"World full (%d/%d players)"}`, maxP, maxP)
		conn.Write(r.Context(), websocket.MessageText, []byte(msg))
		conn.Close(4001, "world full")
		return
	}

	// Resolve player identity from token
	var playerNickname string
	var playerID uint32
	if token != "" {
		sess := h.auth.Validate(token)
		if sess != nil {
			playerNickname = sess.Nickname
			playerID = sess.PlayerID
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}

	var c *Client
	if playerID > 0 {
		c = newClientWithIdentity(h, conn, playerID, playerNickname, worldID)
	} else {
		c = newClient(h, conn)
		c.WorldID = worldID
	}

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
