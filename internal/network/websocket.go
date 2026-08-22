package network

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/world"
)

// NewRouter builds and returns the chi HTTP router.
//
// Routes:
//
//	GET  /             → serves index.html (embedded)
//	GET  /ws           → WebSocket upgrade endpoint
//	GET  /api/metrics  → JSON snapshot of current metrics
//	GET  /debug/pprof/ → Go profiling (development only, guarded by build tag)
func NewRouter(hub *Hub, w *world.World, m *metrics.Metrics, staticFS http.FileSystem) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static frontend
	r.Handle("/*", http.FileServer(staticFS))

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

	return r
}

// handleWebSocket upgrades an HTTP connection to WebSocket and registers
// the resulting client with the hub.
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow all origins during development / demo.
		// Restrict to the deployment domain in production.
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}

	c := newClient(h, conn)
	h.register(c)

	log.Printf("player %d connected from %s", c.Player.ID, r.RemoteAddr)

	// Send welcome
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
