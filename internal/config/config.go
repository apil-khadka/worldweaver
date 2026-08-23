// Package config centralizes all runtime configuration for WorldWeaver.
//
// Hard-coded magic numbers anywhere else in the codebase are a code smell.
// All tunable values live here and flow inward via dependency injection.
//
// # Source priority (highest to lowest)
//  1. CLI flags (parsed in cmd/server/main.go)
//  2. Environment variables (future)
//  3. Config file (future)
//  4. Defaults defined in this file
package config

import "time"

// ServerConfig holds all server-side runtime parameters.
type ServerConfig struct {
	// HTTP
	Addr string // e.g. ":8080"

	// World
	WorldWidth  int
	WorldHeight int
	WorldSeed   int64
	ChunkSize   int // cells per chunk edge (32 or 64)

	// Simulation
	TickRate    int           // simulation ticks per second (target)
	NetworkRate int           // world-update broadcast frequency (Hz)

	// Persistence
	SnapshotDir      string
	SnapshotInterval time.Duration

	// Multiplayer
	MaxPlayers int

	// Features — disable to isolate systems during testing
	SimPlants    bool
	SimFire      bool
	SimVapor     bool
	SimWind      bool
	NetCompress  bool
	DebugMetrics bool
}

// Default returns the recommended default configuration.
// These values are tuned for a 1024×512 world on a single-core laptop.
// Adjust after profiling for your deployment.
func Default() ServerConfig {
	return ServerConfig{
		Addr:             ":8080",
		// A wide, shallow world in the Terraria mould: traversed mostly sideways,
		// with room for vertical strata. Measured at 2.99 ms/tick for 2.1M cells
		// on an M5 Pro, so 1.57M cells leaves ample headroom against the 16.67 ms
		// budget for 60 TPS. See docs/performance/benchmark-results.md.
		WorldWidth:       2048,
		WorldHeight:      768,
		WorldSeed:        20260823,
		ChunkSize:        64,
		TickRate:         60,
		NetworkRate:      20,
		SnapshotDir:      ".",
		SnapshotInterval: 5 * time.Minute,
		MaxPlayers:       16,
		SimPlants:        true,
		SimFire:          true,
		SimVapor:         true,
		SimWind:          true,
		NetCompress:      false,
		DebugMetrics:     true,
	}
}

// ClientConfig represents the configuration delivered to the browser client
// via the WELCOME message so that the client can adapt its behaviour.
type ClientConfig struct {
	WorldW    int    `json:"worldW"`
	WorldH    int    `json:"worldH"`
	ChunkSize int    `json:"chunkSize"`
	Seed      int64  `json:"seed"`
}
