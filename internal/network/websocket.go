package network

import (
	"encoding/json"
	"log"
	"net/http"

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

	// Static frontend
	r.Handle("/*", http.FileServer(staticFS))

	// Root API info
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
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

	// ── Worlds ───────────────────────────────────────────────────────────────
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
		Name   string `json:"name"`
		Seed   int64  `json:"seed"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	info, err := h.worldMgr.CreateWorld(req.Name, req.Seed, req.Width, req.Height, creatorName)
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

	// Send initial full snapshot of the viewport
	SendFullSnapshot(c, h.world)

	ctx := r.Context()
	go c.writePump(ctx)
	c.readPump(ctx) // blocks until disconnect
}
