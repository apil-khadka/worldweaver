package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// WorldInfo holds metadata about a running world instance.
type WorldInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Seed        int64     `json:"seed"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CreatorName string    `json:"creatorName"`
	CreatedAt   time.Time `json:"createdAt"`
	PlayerCount int       `json:"playerCount"`
}

// WorldInstance holds a world's metadata plus a reference to its hub ID.
// The actual simulation/hub state is managed externally — this is just the registry.
type WorldInstance struct {
	Info WorldInfo
}

// WorldManager manages multiple world instances.
// Thread-safe for concurrent access from HTTP handlers.
type WorldManager struct {
	mu     sync.RWMutex
	worlds map[string]*WorldInstance
}

// NewWorldManager creates a new world manager with the default 'genesis' world pre-registered.
func NewWorldManager(defaultSeed int64, defaultW, defaultH int) *WorldManager {
	wm := &WorldManager{
		worlds: make(map[string]*WorldInstance),
	}

	// Register the default world
	wm.worlds["genesis"] = &WorldInstance{
		Info: WorldInfo{
			ID:          "genesis",
			Name:        "Genesis",
			Seed:        defaultSeed,
			Width:       defaultW,
			Height:      defaultH,
			CreatorName: "system",
			CreatedAt:   time.Now(),
		},
	}

	return wm
}

// CreateWorld creates a new world and returns its info.
// Returns an error if the name is empty or already exists.
func (wm *WorldManager) CreateWorld(name string, seed int64, width, height int, creatorName string) (*WorldInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("world name cannot be empty")
	}
	if width <= 0 || width > 4096 {
		width = 1024
	}
	if height <= 0 || height > 2048 {
		height = 512
	}

	id := generateWorldID()

	wm.mu.Lock()
	defer wm.mu.Unlock()

	instance := &WorldInstance{
		Info: WorldInfo{
			ID:          id,
			Name:        name,
			Seed:        seed,
			Width:       width,
			Height:      height,
			CreatorName: creatorName,
			CreatedAt:   time.Now(),
		},
	}
	wm.worlds[id] = instance
	return &instance.Info, nil
}

// DeleteWorld removes a world by ID. Only the creator (or system) can delete.
// Returns an error if not found or unauthorized.
func (wm *WorldManager) DeleteWorld(id string, requesterName string) error {
	if id == "genesis" {
		return fmt.Errorf("cannot delete the default world")
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	w, ok := wm.worlds[id]
	if !ok {
		return fmt.Errorf("world not found")
	}
	if w.Info.CreatorName != requesterName {
		return fmt.Errorf("only the creator can delete this world")
	}

	delete(wm.worlds, id)
	return nil
}

// ListWorlds returns info on all running worlds.
func (wm *WorldManager) ListWorlds() []WorldInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	list := make([]WorldInfo, 0, len(wm.worlds))
	for _, w := range wm.worlds {
		list = append(list, w.Info)
	}
	return list
}

// GetWorld returns info for a specific world, or nil if not found.
func (wm *WorldManager) GetWorld(id string) *WorldInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if w, ok := wm.worlds[id]; ok {
		info := w.Info
		return &info
	}
	return nil
}

// SetPlayerCount updates the player count for a world.
func (wm *WorldManager) SetPlayerCount(id string, count int) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if w, ok := wm.worlds[id]; ok {
		w.Info.PlayerCount = count
	}
}

// Exists checks if a world ID is registered.
func (wm *WorldManager) Exists(id string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	_, ok := wm.worlds[id]
	return ok
}

func generateWorldID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("world-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
