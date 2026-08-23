package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ── Dynamic World Scaling ────────────────────────────────────────────────────

const (
	// BaseWidth is the horizontal room allotted per player slot.
	BaseWidth = 256

	// MinWidth keeps even a two-player world wider than it is tall.
	MinWidth = 1024

	// WorldDepth is fixed for every world regardless of capacity. Depth is a
	// property of the world's vertical strata — sky, surface, underground,
	// cavern, underworld — so it must not shrink with player count. Scaling it
	// down left small worlds too shallow to hold distinct layers.
	WorldDepth = 768
	MaxPlayers = 8
)

// ── World size presets ───────────────────────────────────────────────────────

// WorldSizeName identifies a world size preset chosen at creation time.
type WorldSizeName string

const (
	SizeSmall  WorldSizeName = "small"
	SizeMedium WorldSizeName = "medium"
	SizeLarge  WorldSizeName = "large"
	SizeHuge   WorldSizeName = "huge"
)

// WorldPreset describes the dimensions behind a size name.
//
// Depth grows more slowly than width: every world needs enough height for its
// vertical strata, but a longer world is what makes exploration interesting.
// Cell counts are kept within the measured simulation budget — 2.1M cells runs
// at 2.99 ms/tick against a 16.67 ms allowance for 60 TPS.
type WorldPreset struct {
	Name        WorldSizeName
	Width       int
	Height      int
	Description string
}

// WorldPresets lists the selectable sizes in ascending order.
var WorldPresets = []WorldPreset{
	{SizeSmall, 1024, 640, "Compact — quick to fill, easy on older machines"},
	{SizeMedium, 2048, 768, "Balanced — the default"},
	{SizeLarge, 3072, 896, "Long — Terraria-like proportions"},
	{SizeHuge, 4096, 1024, "Vast — best with a fast machine"},
}

// LookupPreset resolves a size name, falling back to medium for anything
// unrecognised so a malformed request cannot fail world creation.
func LookupPreset(name string) WorldPreset {
	for _, p := range WorldPresets {
		if string(p.Name) == name {
			return p
		}
	}
	return WorldPresets[1] // medium
}

// WorldSize calculates world dimensions for a given player capacity.
//
// Used when no explicit size preset is given. Only the width scales: more players
// need more ground to spread out over, while the vertical strata stay the same
// depth in every world.
// 1–4 players → 1024x768, 6 → 1536x768, 8 → 2048x768.
func WorldSize(playerCap int) (width, height int) {
	if playerCap < 1 {
		playerCap = 1
	}
	if playerCap > MaxPlayers {
		playerCap = MaxPlayers
	}
	width = BaseWidth * playerCap
	if width < MinWidth {
		width = MinWidth
	}
	return width, WorldDepth
}

// ── Cooperative Goals ────────────────────────────────────────────────────────

// GoalType identifies a cooperative goal.
type GoalType int

const (
	GoalStability GoalType = iota
	GoalGrowPlants
	GoalExtinguishFires
	GoalCreateLake
	GoalGrowGrass
	GoalRaiseHerd
	GoalEstablishPredators
	GoalPlantForest
	GoalClearVoid
	GoalClearRadiation
	GoalCoolWorld
)

// GoalDirection says whether a goal is met by reaching a figure or by getting
// below one. Completion used to be a switch with a case per goal type, which
// meant every new objective needed its comparison written out again.
type GoalDirection int

const (
	// AtLeast completes when progress reaches or exceeds the target.
	AtLeast GoalDirection = iota
	// AtMost completes when progress falls to or below the target.
	AtMost
)

// GoalDefinition describes a cooperative goal template.
type GoalDefinition struct {
	Type      GoalType
	Text      string
	Target    int
	Direction GoalDirection
}

// Satisfied reports whether a measurement meets this goal.
func (d GoalDefinition) Satisfied(progress int) bool {
	if d.Direction == AtMost {
		return progress <= d.Target
	}
	return progress >= d.Target
}

// GoalRotation is the set of goals that rotate every 5 minutes.
//
// Objectives that ask the players to clean something up are only offered when
// there is something to clean up: the rotation skips any goal that is already
// satisfied when it comes round, so "extinguish all fires" is not handed out and
// immediately completed on a world that is not burning.
var GoalRotation = []GoalDefinition{
	{Type: GoalStability, Text: "Raise world stability above 80%", Target: 80, Direction: AtLeast},
	{Type: GoalGrowGrass, Text: "Cover the land with 3000 grass", Target: 3000, Direction: AtLeast},
	{Type: GoalRaiseHerd, Text: "Raise a herd of 60 grazing animals", Target: 60, Direction: AtLeast},
	{Type: GoalPlantForest, Text: "Grow a forest of 1200 trees", Target: 1200, Direction: AtLeast},
	{Type: GoalCreateLake, Text: "Fill a lake of 1000 connected water", Target: 1000, Direction: AtLeast},
	{Type: GoalEstablishPredators, Text: "Establish 15 predators", Target: 15, Direction: AtLeast},
	{Type: GoalExtinguishFires, Text: "Put out every fire", Target: 0, Direction: AtMost},
	{Type: GoalClearVoid, Text: "Seal every void", Target: 0, Direction: AtMost},
	{Type: GoalClearRadiation, Text: "Clear all radiation", Target: 0, Direction: AtMost},
	{Type: GoalCoolWorld, Text: "Cool the world: fewer than 200 lava cells", Target: 200, Direction: AtMost},
	{Type: GoalGrowPlants, Text: "Grow 500 plants", Target: 500, Direction: AtLeast},
}

// GoalRotationInterval is how often goals rotate.
const GoalRotationInterval = 5 * time.Minute

// GoalState tracks the current cooperative goal for a world.
type GoalState struct {
	Definition GoalDefinition `json:"definition"`
	Progress   int            `json:"progress"`
	Completed  bool           `json:"completed"`
	StartedAt  time.Time      `json:"startedAt"`
	Index      int            `json:"index"` // position in GoalRotation
}

// ── World Info & Instance ────────────────────────────────────────────────────

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
	MaxPlayers  int       `json:"maxPlayers"`

	// Size is the preset the world was created from, for display in the lobby.
	Size WorldSizeName `json:"size,omitempty"`
}

// WorldInstance holds a world's metadata plus cooperative goal state.
type WorldInstance struct {
	Info WorldInfo
	Goal GoalState
}

// ── World Manager ────────────────────────────────────────────────────────────

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
			MaxPlayers:  MaxPlayers,
		},
		Goal: GoalState{
			Definition: GoalRotation[0],
			StartedAt:  time.Now(),
			Index:      0,
		},
	}

	return wm
}

// CreateWorld creates a new world and returns its info.
//
// playerCap is clamped to [1, MaxPlayers]. Dimensions come from the named size
// preset; an empty or unrecognised size falls back to deriving them from player
// capacity, which is how worlds were sized before presets existed.
func (wm *WorldManager) CreateWorld(name string, seed int64, playerCap int, creatorName string, size string) (*WorldInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("world name cannot be empty")
	}
	if playerCap < 1 {
		playerCap = MaxPlayers
	}
	if playerCap > MaxPlayers {
		playerCap = MaxPlayers
	}

	var width, height int
	var sizeName WorldSizeName
	if size == "" {
		width, height = WorldSize(playerCap)
		sizeName = SizeMedium
	} else {
		p := LookupPreset(size)
		width, height, sizeName = p.Width, p.Height, p.Name
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
			MaxPlayers:  playerCap,
			Size:        sizeName,
		},
		Goal: GoalState{
			Definition: GoalRotation[0],
			StartedAt:  time.Now(),
			Index:      0,
		},
	}
	wm.worlds[id] = instance
	return &instance.Info, nil
}

// DeleteWorldByOwner removes a world by ID.
//
// Authorization is NOT performed here: the caller must already have established
// ownership through the AccessRegistry, which records the owner's public key.
// The previous DeleteWorld compared the requester's display name against
// CreatorName, which anyone could satisfy by picking that name at login.
func (wm *WorldManager) DeleteWorldByOwner(id string) error {
	if id == "genesis" {
		return fmt.Errorf("cannot delete the default world")
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, ok := wm.worlds[id]; !ok {
		return fmt.Errorf("world not found")
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

// IsFull returns true if a world has reached its player cap.
func (wm *WorldManager) IsFull(id string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if w, ok := wm.worlds[id]; ok {
		return w.Info.PlayerCount >= w.Info.MaxPlayers
	}
	return false
}

// GetMaxPlayers returns the player cap for a world (0 if not found).
func (wm *WorldManager) GetMaxPlayers(id string) int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if w, ok := wm.worlds[id]; ok {
		return w.Info.MaxPlayers
	}
	return 0
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

// ── Goal Management ──────────────────────────────────────────────────────────

// GetGoalState returns the current goal state for a world.
func (wm *WorldManager) GetGoalState(id string) *GoalState {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if w, ok := wm.worlds[id]; ok {
		g := w.Goal
		return &g
	}
	return nil
}

// UpdateGoalProgress sets the progress for the current goal.
// Returns true if the goal was JUST completed (transition to completed).
func (wm *WorldManager) UpdateGoalProgress(id string, progress int) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	w, ok := wm.worlds[id]
	if !ok {
		return false
	}
	if w.Goal.Completed {
		return false
	}
	w.Goal.Progress = progress

	if w.Goal.Definition.Satisfied(progress) {
		w.Goal.Completed = true
		return true
	}
	return false
}

// SkipCurrentGoal advances to the next objective without awarding a bonus.
//
// Used when a freshly rotated goal turns out to be satisfied already, which
// happens with clean-up objectives on a world that has nothing to clean up.
func (wm *WorldManager) SkipCurrentGoal(id string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	w, ok := wm.worlds[id]
	if !ok {
		return
	}
	w.Goal.Index = (w.Goal.Index + 1) % len(GoalRotation)
	w.Goal.Definition = GoalRotation[w.Goal.Index]
	w.Goal.Progress = 0
	w.Goal.Completed = false
	w.Goal.StartedAt = time.Now()
}

// GoalAge reports how long the current goal has been active.
func (wm *WorldManager) GoalAge(id string) time.Duration {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	w, ok := wm.worlds[id]
	if !ok {
		return 0
	}
	return time.Since(w.Goal.StartedAt)
}

// RotateGoalIfDue checks if the current goal has expired and rotates.
// Returns true if a rotation happened.
func (wm *WorldManager) RotateGoalIfDue(id string) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	w, ok := wm.worlds[id]
	if !ok {
		return false
	}

	if time.Since(w.Goal.StartedAt) < GoalRotationInterval {
		return false
	}

	// Rotate to next goal
	nextIdx := (w.Goal.Index + 1) % len(GoalRotation)
	w.Goal = GoalState{
		Definition: GoalRotation[nextIdx],
		StartedAt:  time.Now(),
		Index:      nextIdx,
	}
	return true
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func generateWorldID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("world-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
