// WorldWeaver server entry point.
//
// # Architecture
//
// The process runs three concurrent pipelines:
//
//  1. Simulation goroutine — advances the world on a fixed 60 TPS timestep.
//  2. Broadcast goroutine  — streams dirty chunks to all WebSocket clients
//     at a lower frequency (~20 Hz) to reduce bandwidth.
//  3. HTTP server          — serves the static client and accepts WebSocket
//     upgrades.  Built on chi for clean route composition.
//
// # Server authority
//
// Clients never mutate world state directly.  They send PlayerAction requests
// which are validated and enqueued here, then processed inside the simulation
// tick on the next iteration.
//
// # Usage
//
//	go run ./cmd/server [flags]
//
// Flags:
//
//	-addr       HTTP listen address (default :8080)
//	-size       World size preset: small, medium, large, huge
//	-width      World width  in cells (default 2048)
//	-height     World height in cells (default 768)
//	-seed       World generation seed (default 20260823)
//	-snapdir    Directory for world snapshots (default .)
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/network"
	"github.com/worldweaver/worldweaver/internal/persistence"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

func main() {
	addr    := flag.String("addr", ":8080", "HTTP/WebSocket listen address")
	sizeName := flag.String("size", "", "World size preset: small, medium, large, huge (overrides -width/-height)")
	worldW  := flag.Int("width", 2048, "World width in cells")
	worldH  := flag.Int("height", 768, "World height in cells")
	seed    := flag.Int64("seed", 20260823, "World generation seed")
	snapDir := flag.String("snapdir", ".", "Directory for world snapshots")
	flag.Parse()

	// A named preset is the friendlier way to pick a size; explicit dimensions
	// remain available for benchmarking odd shapes.
	if *sizeName != "" {
		p := game.LookupPreset(*sizeName)
		*worldW, *worldH = p.Width, p.Height
		log.Printf("World size preset %q → %dx%d (%s)", p.Name, p.Width, p.Height, p.Description)
	}

	log.Printf("WorldWeaver — world %dx%d (%.1fM cells) seed=%d addr=%s",
		*worldW, *worldH, float64(*worldW**worldH)/1e6, *seed, *addr)

	// ── World ────────────────────────────────────────────────────────────────
	w := world.New(*worldW, *worldH, *seed)

	if err := persistence.Load(*snapDir, w); err != nil {
		log.Printf("No snapshot found (%v) — generating fresh world", err)
		w.Generate()
	} else {
		log.Printf("Restored world from snapshot at tick %d", w.Tick)
	}

	// ── Metrics ──────────────────────────────────────────────────────────────
	m := metrics.New()

	// ── Simulation ───────────────────────────────────────────────────────────
	eng := simulation.NewEngine(w, m)

	// ── Network ──────────────────────────────────────────────────────────────
	sb := game.NewScoreboard()
	auth := game.NewAuthManager()
	worldMgr := game.NewWorldManager(*seed, *worldW, *worldH)
	hub := network.NewHub(w, eng, m, sb, "genesis", auth, worldMgr)

	// Static file system — serves the built frontend from web/dist/
	staticFS := http.Dir("web/dist")
	router := network.NewRouter(hub, w, m, staticFS)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // WebSocket connections are long-lived
		IdleTimeout:  60 * time.Second,
	}

	// ── Broadcast pipeline ───────────────────────────────────────────────────
	// Runs independently of the simulation loop.  Frequency is lower than TPS
	// so bandwidth stays bounded even at high cell counts.
	go func() {
		ticker := time.NewTicker(time.Second / 20) // 20 Hz network updates
		defer ticker.Stop()
		for range ticker.C {
			hub.BroadcastChunkUpdates(w)
		}
	}()

	// Metrics broadcast — every second
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			snap := m.Snapshot()
			stability := game.Compute(w)
			hub.BroadcastMetrics(snap, stability.Overall, w.Tick)
			// Regenerate influence for all connected players
			hub.RegenerateAllInfluence()
			// Tick cooperative goals (check progress, rotate if due)
			hub.TickGoals(stability.Overall)
		}
	}()

	// Periodic persistence — every 5 minutes
	cancelSnap := persistence.SavePeriodic(*snapDir, w, 5*time.Minute)

	// ── Start ────────────────────────────────────────────────────────────────
	eng.Start()

	go func() {
		log.Printf("Listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// ── Shutdown ─────────────────────────────────────────────────────────────
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
	<-sigC

	log.Println("Shutting down…")
	cancelSnap()
	eng.Stop()

	if err := persistence.Save(*snapDir, w); err != nil {
		log.Printf("Final snapshot save failed: %v", err)
	}

	log.Println("Goodbye.")
}
