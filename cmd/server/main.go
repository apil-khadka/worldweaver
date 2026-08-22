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
//	-width      World width  in cells (default 1024)
//	-height     World height in cells (default 512)
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

	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/network"
	"github.com/worldweaver/worldweaver/internal/persistence"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

func main() {
	addr    := flag.String("addr", ":8080", "HTTP/WebSocket listen address")
	worldW  := flag.Int("width", 1024, "World width in cells")
	worldH  := flag.Int("height", 512, "World height in cells")
	seed    := flag.Int64("seed", 20260823, "World generation seed")
	snapDir := flag.String("snapdir", ".", "Directory for world snapshots")
	flag.Parse()

	log.Printf("WorldWeaver — world %dx%d seed=%d addr=%s",
		*worldW, *worldH, *seed, *addr)

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
	hub := network.NewHub(w, eng, m)

	// Static file system — serves web/ directory
	staticFS := http.Dir("web")
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
			hub.BroadcastMetrics(snap, 0, w.Tick)
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
