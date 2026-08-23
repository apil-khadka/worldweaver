package game

import (
	"sort"
	"sync"
	"time"
)

// Contribution ledger.
//
// The Scoreboard answers "who is winning". That is the wrong question for a game
// whose whole premise is that a world is built by its population together: it
// rewards whoever sprayed the most cells and says nothing about who they were
// working with. Two players terraforming the same valley score exactly as if they
// had never met.
//
// The ledger records the same actions with a different emphasis. It keeps what
// each player actually contributed — which kinds of matter, how much, over what
// span of time — and it credits ASSISTS: when players work the same ground at
// roughly the same moment, each is recorded as having helped the other, and the
// acting player's score is multiplied. Building next to someone is worth more
// than building alone, which is the only way the motto shows up in the numbers
// rather than in the marketing.
//
// The ledger is deliberately standalone: plain integers for material, power and
// player identifiers, no import of the simulation, world or network packages. It
// observes actions that have already happened and has no authority over them, so
// it can be tested on its own and wired in wherever the action is dispatched.

// ── Tunables ─────────────────────────────────────────────────────────────────

const (
	// AssistRadius is how close, in cells, two players' actions must be for each
	// to be credited with assisting the other.
	//
	// 48 sits between MaxToolRadius (32) and MaxRadius (64) — the reach of a
	// brush. It is the smallest distance at which the two players could plausibly
	// have touched the same cells. Requiring exact overlap would credit almost
	// nobody, since people building together stand beside each other rather than
	// on top of each other; a much larger radius would hand out assists for
	// unrelated work happening elsewhere on a 4096-wide world.
	AssistRadius = 48

	// AssistWindow is how recently the other player must have acted for the two
	// actions to count as joint work.
	//
	// 20s is long enough to cover taking turns — one player raises a ridge, the
	// other plants it — and short enough that someone who left the area a minute
	// ago stops collecting credit for what is now somebody else's build.
	AssistWindow = 20 * time.Second

	// AssistTile is the side of the grid square used to decide whether a pair of
	// players is still working "the same patch" for anti-farming purposes.
	//
	// One tile is the diameter of the assist radius: anything inside it could
	// have been reached without moving, so it is treated as one worksite.
	AssistTile = 2 * AssistRadius

	// AssistCooldown is the period over which a pair's assist budget for one tile
	// is counted, after which it refills.
	//
	// Longer than AssistWindow on purpose: while the window is still open the two
	// are mid-collaboration, and further clicks are the same collaboration rather
	// than a new one worth crediting again.
	AssistCooldown = 30 * time.Second

	// MaxPairAssistsPerTile bounds how often one pair of players can earn assist
	// credit for the same tile within one AssistCooldown.
	//
	// This is the anti-farming guard. Without it, two players could park on a
	// single cell and alternate clicks forever, each click assisted by the other,
	// multiplying a stream of near-zero-effort actions indefinitely. Three lets a
	// genuine back-and-forth over one spot be recognised; the fourth and later
	// pokes score as solo work. Earning more means moving somewhere new, which is
	// what actually building something together looks like.
	MaxPairAssistsPerTile = 3

	// AssistBonus is added to the score multiplier for each distinct player
	// credited on one action.
	AssistBonus = 0.15

	// MaxCollaborationMultiplier caps the reward for joint work.
	//
	// 1.75 is reached at five simultaneous partners. The cap exists in both
	// directions: collaboration must be clearly better than working alone, but
	// not so much better that a solo builder's contribution stops registering,
	// and the ceiling means no arrangement of players can turn the multiplier
	// into an unbounded score source.
	MaxCollaborationMultiplier = 1.75
)

// Memory bounds. The ledger is a long-lived in-process map fed by every action
// in every world, so each dimension it grows along has a ceiling. Nothing here
// is authoritative game state: dropping the coldest entry loses some history,
// never correctness.
const (
	// MaxTrackedWorlds bounds how many worlds hold ledger state at once. Worlds
	// are created freely by players and only sometimes deleted, so recording into
	// a new world beyond this limit evicts the least recently active one.
	MaxTrackedWorlds = 256

	// MaxContributorsPerWorld bounds the roster of one world. A world seats
	// MaxPlayers at a time but sees many more over its life; past this point the
	// contributor who acted longest ago is dropped.
	MaxContributorsPerWorld = 256

	// recentActionsPerWorld is the size of the per-world ring of actions scanned
	// for assist partners. It only needs to hold what a full world can produce
	// inside one AssistWindow.
	recentActionsPerWorld = 64

	// maxPairBudgets bounds the per-world anti-farm bookkeeping.
	maxPairBudgets = 1024
)

// ── Categories ───────────────────────────────────────────────────────────────

// ContributionCategory groups what a player added to the world, so a roster can
// show that someone dug the mountains while someone else filled the sea.
type ContributionCategory uint8

const (
	// ContribTerrain is solid ground: rock, soil, sand, ash, ice.
	ContribTerrain ContributionCategory = iota

	// ContribWater is water and the ice-free part of the water cycle.
	ContribWater

	// ContribLife is plants and creatures.
	ContribLife

	// ContribFire is combustion: fire, lava, embers, oil.
	ContribFire

	// ContribAir is the atmosphere: vapour, smoke, cloud.
	ContribAir

	// ContribHazard is destructive matter: void, radiation, plasma.
	ContribHazard

	// ContribOther covers anything unrecognised, so an unknown id is still
	// counted rather than silently dropped.
	ContribOther

	contribCategoryCount
)

// String names a category for display and for test failure messages.
func (c ContributionCategory) String() string {
	switch c {
	case ContribTerrain:
		return "terrain"
	case ContribWater:
		return "water"
	case ContribLife:
		return "life"
	case ContribFire:
		return "fire"
	case ContribAir:
		return "air"
	case ContribHazard:
		return "hazard"
	default:
		return "other"
	}
}

// contributionWeight is the score earned per cell in each category.
//
// Weights express what the world needs rather than what is easiest to spam.
// Life is the hardest thing to establish and the easiest to lose, so it is worth
// most; hazards are scored at all only because clearing and re-shaping with them
// is legitimate play, but at a quarter, so nobody out-contributes a gardener by
// flooding a world with void.
var contributionWeight = [contribCategoryCount]float64{
	ContribTerrain: 1.0,
	ContribWater:   1.5,
	ContribLife:    2.0,
	ContribFire:    0.5,
	ContribAir:     0.75,
	ContribHazard:  0.25,
	ContribOther:   1.0,
}

// CategoryWeight returns the score earned per cell in a category.
func CategoryWeight(c ContributionCategory) float64 {
	if c >= contribCategoryCount {
		return contributionWeight[ContribOther]
	}
	return contributionWeight[c]
}

// CategoryForMaterial maps a material id to a contribution category.
func CategoryForMaterial(m uint8) ContributionCategory {
	switch m {
	case matRock, matSoil, matSand, matAsh, matIce:
		return ContribTerrain
	case matWater:
		return ContribWater
	case matPlant, matHerbivore, matPredator:
		return ContribLife
	case matFire, matLava, matEmber, matOil:
		return ContribFire
	case matVapor, matSmoke, matCloud:
		return ContribAir
	case matVoid, matRadiation, matPlasma:
		return ContribHazard
	default:
		return ContribOther
	}
}

// CategoryForPower maps an elemental power id to a contribution category.
func CategoryForPower(p uint8) ContributionCategory {
	switch p {
	case PowerRain:
		return ContribWater
	case PowerHeat:
		return ContribFire
	case PowerWind:
		return ContribAir
	case PowerGrowth, PowerLife:
		return ContribLife
	default:
		return ContribOther
	}
}

// ── Records ──────────────────────────────────────────────────────────────────

// ContributionAction is one completed act of world-building, as observed after
// the fact. The caller reports what actually landed, not what was requested.
type ContributionAction struct {
	PlayerID uint32
	Category ContributionCategory

	// X, Y is the centre of the affected area, used for assist proximity.
	X, Y int

	// Cells is how many cells the action changed.
	Cells int

	// At is when the action happened. Zero means now, which is what live callers
	// want; tests set it explicitly to drive the time window.
	At time.Time
}

// ContributionResult reports what one recorded action earned, so a caller can
// tell the player they were credited for helping someone.
type ContributionResult struct {
	PlayerID uint32
	Category ContributionCategory
	Cells    int

	// BaseScore is what the action was worth before collaboration.
	BaseScore float64

	// Multiplier is 1.0 for solo work, up to MaxCollaborationMultiplier.
	Multiplier float64

	// Awarded is BaseScore * Multiplier.
	Awarded float64

	// TotalScore is the player's running contribution score after this action.
	TotalScore float64

	// Assisted lists the players credited on this action, ascending.
	Assisted []uint32

	// Throttled is true when a nearby player was found but the pair had already
	// spent its assist budget for that patch of world. It distinguishes "nobody
	// was there" from "you two have been poking the same cell".
	Throttled bool
}

// PlayerContribution is everything one player has added to one world.
type PlayerContribution struct {
	PlayerID uint32 `json:"playerId"`

	// Actions is how many recorded acts of building this player performed.
	Actions int `json:"actions"`

	// Cells is the total cells changed across all categories.
	Cells int `json:"cells"`

	// CellsByCategory breaks that total down by what was added.
	CellsByCategory map[ContributionCategory]int `json:"cellsByCategory"`

	// Assists is how many times this player and another were credited together.
	// It counts both directions: helping and being helped are the same act seen
	// from two sides, and separating them would only invite arguments about who
	// arrived first.
	Assists int `json:"assists"`

	// Partners counts assists per collaborator, so "you build a lot with Ana"
	// is answerable.
	Partners map[uint32]int `json:"partners"`

	// AssistedActions is how many of this player's own actions were multiplied.
	AssistedActions int `json:"assistedActions"`

	// Score is the running contribution score, collaboration multiplier included.
	Score float64 `json:"score"`

	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// clone returns a deep copy, so callers can neither read a map while the ledger
// writes it nor mutate ledger state through a returned value.
func (pc *PlayerContribution) clone() PlayerContribution {
	out := *pc
	out.CellsByCategory = make(map[ContributionCategory]int, len(pc.CellsByCategory))
	for k, v := range pc.CellsByCategory {
		out.CellsByCategory[k] = v
	}
	out.Partners = make(map[uint32]int, len(pc.Partners))
	for k, v := range pc.Partners {
		out.Partners[k] = v
	}
	return out
}

// WorldContribution is the whole population's work on one world.
type WorldContribution struct {
	WorldID string `json:"worldId"`

	TotalActions int     `json:"totalActions"`
	TotalCells   int     `json:"totalCells"`
	TotalScore   float64 `json:"totalScore"`
	TotalAssists int     `json:"totalAssists"`

	// Contributors is the roster ordered by score, highest first.
	Contributors []PlayerContribution `json:"contributors"`

	// MostCollaborative is the player with the most assists, or 0 if no
	// collaboration has been recorded yet.
	MostCollaborative uint32 `json:"mostCollaborative"`
}

// MilestoneProgress reports a shared target that the world advances together.
//
// Progress is the SUM of every contributor's score, not the best individual
// figure: a milestone is something the population reaches, so a hundred small
// contributions must move it as far as one large one.
type MilestoneProgress struct {
	Target       float64 `json:"target"`
	Progress     float64 `json:"progress"`
	Remaining    float64 `json:"remaining"`
	Fraction     float64 `json:"fraction"`
	Complete     bool    `json:"complete"`
	Contributors int     `json:"contributors"`
}

// ── Ledger ───────────────────────────────────────────────────────────────────

// recentAction is a compact note of where and when somebody acted, kept only
// long enough to answer "was anyone else just here".
type recentAction struct {
	playerID uint32
	x, y     int
	at       time.Time
}

// pairTile identifies a pair of players working one tile. The player ids are
// stored low-first so the pair is the same key whichever of them acts.
type pairTile struct {
	lo, hi uint32
	tx, ty int
}

// pairBudget tracks how much assist credit a pair has drawn on one tile since
// the budget last refilled.
type pairBudget struct {
	granted int
	since   time.Time
}

// worldLedger is the per-world state.
type worldLedger struct {
	players map[uint32]*PlayerContribution

	// recent is a fixed-size ring: assists only ever look a short way back, so
	// old entries are overwritten rather than accumulated.
	recent   []recentAction
	nextSlot int

	pairs map[pairTile]pairBudget

	lastActive time.Time
}

// ContributionLedger records contributions and assists for every world.
//
// Safe for concurrent use: HTTP handlers, the websocket read pumps and the
// simulation loop all touch this.
type ContributionLedger struct {
	mu     sync.RWMutex
	worlds map[string]*worldLedger
}

// NewContributionLedger creates an empty ledger.
func NewContributionLedger() *ContributionLedger {
	return &ContributionLedger{
		worlds: make(map[string]*worldLedger),
	}
}

// Record books one completed action and returns what it earned.
func (l *ContributionLedger) Record(worldID string, act ContributionAction) ContributionResult {
	if act.At.IsZero() {
		act.At = time.Now()
	}
	if act.Cells < 0 {
		act.Cells = 0
	}
	if act.Category >= contribCategoryCount {
		act.Category = ContribOther
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	w := l.worldFor(worldID, act.At)
	pc := w.contributorFor(act.PlayerID, act.At)

	partners, throttled := w.creditAssists(act)

	multiplier := 1.0 + AssistBonus*float64(len(partners))
	if multiplier > MaxCollaborationMultiplier {
		multiplier = MaxCollaborationMultiplier
	}

	base := float64(act.Cells) * CategoryWeight(act.Category)
	awarded := base * multiplier

	pc.Actions++
	pc.Cells += act.Cells
	pc.CellsByCategory[act.Category] += act.Cells
	pc.Score += awarded
	if act.At.After(pc.LastSeen) {
		pc.LastSeen = act.At
	}
	if len(partners) > 0 {
		pc.AssistedActions++
	}

	// The assist is mutual: both players carry the record of having worked
	// together. Only the acting player's score is multiplied — the partner
	// collects their own multiplier on their next action, which by then sees
	// this one in the recent ring.
	for _, partner := range partners {
		pc.Assists++
		pc.Partners[partner]++
		if other, ok := w.players[partner]; ok {
			other.Assists++
			other.Partners[act.PlayerID]++
		}
	}

	w.remember(recentAction{playerID: act.PlayerID, x: act.X, y: act.Y, at: act.At})

	return ContributionResult{
		PlayerID:   act.PlayerID,
		Category:   act.Category,
		Cells:      act.Cells,
		BaseScore:  base,
		Multiplier: multiplier,
		Awarded:    awarded,
		TotalScore: pc.Score,
		Assisted:   partners,
		Throttled:  throttled,
	}
}

// RecordPower books an elemental power application, deriving the category from
// the power id.
func (l *ContributionLedger) RecordPower(worldID string, playerID uint32, power uint8, x, y, cells int) ContributionResult {
	return l.Record(worldID, ContributionAction{
		PlayerID: playerID,
		Category: CategoryForPower(power),
		X:        x,
		Y:        y,
		Cells:    cells,
	})
}

// RecordPlacement books a direct material edit, deriving the category from the
// material id.
func (l *ContributionLedger) RecordPlacement(worldID string, playerID uint32, material uint8, x, y, cells int) ContributionResult {
	return l.Record(worldID, ContributionAction{
		PlayerID: playerID,
		Category: CategoryForMaterial(material),
		X:        x,
		Y:        y,
		Cells:    cells,
	})
}

// Player returns one player's contribution to one world.
func (l *ContributionLedger) Player(worldID string, playerID uint32) (PlayerContribution, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	w, ok := l.worlds[worldID]
	if !ok {
		return PlayerContribution{}, false
	}
	pc, ok := w.players[playerID]
	if !ok {
		return PlayerContribution{}, false
	}
	return pc.clone(), true
}

// Roster returns the world's contributors ordered by score, highest first.
//
// Ties break towards whoever started contributing earlier, then by player id, so
// the order is stable rather than dependent on map iteration.
func (l *ContributionLedger) Roster(worldID string) []PlayerContribution {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rosterLocked(worldID)
}

// rosterLocked builds the ordered roster. Caller must hold l.mu.
func (l *ContributionLedger) rosterLocked(worldID string) []PlayerContribution {
	w, ok := l.worlds[worldID]
	if !ok {
		return nil
	}

	out := make([]PlayerContribution, 0, len(w.players))
	for _, pc := range w.players {
		out = append(out, pc.clone())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if !out[i].FirstSeen.Equal(out[j].FirstSeen) {
			return out[i].FirstSeen.Before(out[j].FirstSeen)
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out
}

// World returns the aggregate for one world. A world with no recorded actions
// yields a zero aggregate rather than an error: callers render it either way.
func (l *ContributionLedger) World(worldID string) WorldContribution {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := WorldContribution{WorldID: worldID}
	roster := l.rosterLocked(worldID)
	out.Contributors = roster
	for i := range roster {
		out.TotalActions += roster[i].Actions
		out.TotalCells += roster[i].Cells
		out.TotalScore += roster[i].Score
		out.TotalAssists += roster[i].Assists
	}
	out.MostCollaborative, _, _ = l.mostCollaborativeLocked(worldID)
	return out
}

// MostCollaborative returns the player with the highest assist count.
//
// ok is false when no assists have been recorded, which is different from a tie
// at zero: before anyone has worked together there is no answer to give.
func (l *ContributionLedger) MostCollaborative(worldID string) (playerID uint32, assists int, ok bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.mostCollaborativeLocked(worldID)
}

// mostCollaborativeLocked picks the top assister. Caller must hold l.mu.
func (l *ContributionLedger) mostCollaborativeLocked(worldID string) (uint32, int, bool) {
	w, ok := l.worlds[worldID]
	if !ok {
		return 0, 0, false
	}

	var bestID uint32
	bestAssists := 0
	bestScore := 0.0
	for id, pc := range w.players {
		if pc.Assists == 0 {
			continue
		}
		// Ties go to the higher score, then the lower id, so the answer does not
		// wander between calls.
		better := pc.Assists > bestAssists ||
			(pc.Assists == bestAssists && pc.Score > bestScore) ||
			(pc.Assists == bestAssists && pc.Score == bestScore && (bestID == 0 || id < bestID))
		if better {
			bestID, bestAssists, bestScore = id, pc.Assists, pc.Score
		}
	}
	if bestAssists == 0 {
		return 0, 0, false
	}
	return bestID, bestAssists, true
}

// Milestone reports progress towards a shared target for one world.
func (l *ContributionLedger) Milestone(worldID string, target float64) MilestoneProgress {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := MilestoneProgress{Target: target}
	w, ok := l.worlds[worldID]
	if ok {
		for _, pc := range w.players {
			out.Progress += pc.Score
			out.Contributors++
		}
	}

	if target <= 0 {
		// A milestone with no target is already met; reporting an infinite or
		// negative fraction would only push the nonsense into the UI.
		out.Fraction = 1
		out.Complete = true
		return out
	}

	out.Remaining = target - out.Progress
	if out.Remaining < 0 {
		out.Remaining = 0
	}
	out.Fraction = out.Progress / target
	if out.Fraction > 1 {
		out.Fraction = 1
	}
	out.Complete = out.Progress >= target
	return out
}

// ForgetWorld drops all ledger state for a world. Call this when a world is
// deleted, otherwise its contributors sit in memory for the life of the process.
func (l *ContributionLedger) ForgetWorld(worldID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.worlds, worldID)
}

// PruneIdle drops worlds that have seen no contribution for longer than idle,
// returning how many went. Intended to be called periodically alongside the
// other housekeeping sweeps.
func (l *ContributionLedger) PruneIdle(idle time.Duration) int {
	if idle <= 0 {
		return 0
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	removed := 0
	for id, w := range l.worlds {
		if now.Sub(w.lastActive) > idle {
			delete(l.worlds, id)
			removed++
		}
	}
	return removed
}

// WorldCount reports how many worlds hold ledger state, for metrics and tests.
func (l *ContributionLedger) WorldCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.worlds)
}

// ContributorCount reports how many players have contributed to a world.
func (l *ContributionLedger) ContributorCount(worldID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if w, ok := l.worlds[worldID]; ok {
		return len(w.players)
	}
	return 0
}

// ── Internals ────────────────────────────────────────────────────────────────

// worldFor returns the per-world ledger, creating it if needed and evicting the
// coldest world if that would exceed MaxTrackedWorlds.
// Caller must hold l.mu for writing.
func (l *ContributionLedger) worldFor(worldID string, now time.Time) *worldLedger {
	if w, ok := l.worlds[worldID]; ok {
		if now.After(w.lastActive) {
			w.lastActive = now
		}
		return w
	}

	if len(l.worlds) >= MaxTrackedWorlds {
		l.evictColdestWorld()
	}

	w := &worldLedger{
		players:    make(map[uint32]*PlayerContribution),
		recent:     make([]recentAction, 0, recentActionsPerWorld),
		pairs:      make(map[pairTile]pairBudget),
		lastActive: now,
	}
	l.worlds[worldID] = w
	return w
}

// evictColdestWorld removes the world whose last contribution is oldest.
// Caller must hold l.mu for writing.
func (l *ContributionLedger) evictColdestWorld() {
	var coldestID string
	var coldest time.Time
	for id, w := range l.worlds {
		if coldestID == "" || w.lastActive.Before(coldest) {
			coldestID, coldest = id, w.lastActive
		}
	}
	if coldestID != "" {
		delete(l.worlds, coldestID)
	}
}

// contributorFor returns a player's entry, creating it if needed and evicting
// the coldest contributor if that would exceed MaxContributorsPerWorld.
// Caller must hold l.mu for writing.
func (w *worldLedger) contributorFor(playerID uint32, now time.Time) *PlayerContribution {
	if pc, ok := w.players[playerID]; ok {
		return pc
	}

	if len(w.players) >= MaxContributorsPerWorld {
		w.evictColdestContributor()
	}

	pc := &PlayerContribution{
		PlayerID:        playerID,
		CellsByCategory: make(map[ContributionCategory]int),
		Partners:        make(map[uint32]int),
		FirstSeen:       now,
		LastSeen:        now,
	}
	w.players[playerID] = pc
	return pc
}

// evictColdestContributor removes the player who acted longest ago, and scrubs
// them from everyone else's partner tallies so those maps stay bounded by the
// roster size rather than growing with every player the world has ever seen.
// Caller must hold l.mu for writing.
func (w *worldLedger) evictColdestContributor() {
	var coldestID uint32
	var coldest time.Time
	found := false
	for id, pc := range w.players {
		if !found || pc.LastSeen.Before(coldest) {
			coldestID, coldest, found = id, pc.LastSeen, true
		}
	}
	if !found {
		return
	}
	delete(w.players, coldestID)
	for _, pc := range w.players {
		delete(pc.Partners, coldestID)
	}
}

// creditAssists finds the players who were just working the same ground and
// spends their shared assist budget.
//
// Returns the credited partner ids ascending, and whether at least one nearby
// player was refused because the pair had already drawn its budget for this
// tile. Caller must hold l.mu for writing.
func (w *worldLedger) creditAssists(act ContributionAction) ([]uint32, bool) {
	tx, ty := tileOf(act.X), tileOf(act.Y)

	var partners []uint32
	throttled := false
	seen := make(map[uint32]bool)

	for i := range w.recent {
		e := w.recent[i]
		if e.playerID == 0 && e.at.IsZero() {
			continue // unused ring slot
		}
		if e.playerID == act.PlayerID || seen[e.playerID] {
			continue
		}
		if gap := act.At.Sub(e.at); gap < 0 || gap > AssistWindow {
			// A negative gap means the ring holds an action from after this one,
			// which only happens if a caller replays out of order. Treating it as
			// out of window keeps the credit rule honest either way.
			continue
		}
		dx, dy := act.X-e.x, act.Y-e.y
		if dx*dx+dy*dy > AssistRadius*AssistRadius {
			continue
		}

		seen[e.playerID] = true
		if !w.spendPairBudget(act.PlayerID, e.playerID, tx, ty, act.At) {
			throttled = true
			continue
		}
		partners = append(partners, e.playerID)
	}

	sort.Slice(partners, func(i, j int) bool { return partners[i] < partners[j] })
	return partners, throttled
}

// spendPairBudget draws one assist credit for a pair on one tile, reporting
// whether any was left. Caller must hold l.mu for writing.
func (w *worldLedger) spendPairBudget(a, b uint32, tx, ty int, now time.Time) bool {
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	key := pairTile{lo: lo, hi: hi, tx: tx, ty: ty}

	budget, ok := w.pairs[key]
	if !ok || now.Sub(budget.since) > AssistCooldown {
		// First visit, or the cooldown has elapsed and the budget refills.
		w.pruneStalePairs(now)
		w.pairs[key] = pairBudget{granted: 1, since: now}
		return true
	}
	if budget.granted >= MaxPairAssistsPerTile {
		return false
	}
	budget.granted++
	w.pairs[key] = budget
	return true
}

// pruneStalePairs keeps the anti-farm bookkeeping bounded. Caller must hold
// l.mu for writing.
func (w *worldLedger) pruneStalePairs(now time.Time) {
	if len(w.pairs) < maxPairBudgets {
		return
	}
	for key, budget := range w.pairs {
		if now.Sub(budget.since) > AssistCooldown {
			delete(w.pairs, key)
		}
	}
	if len(w.pairs) >= maxPairBudgets {
		// Every budget is still live, which takes more concurrent pairs than a
		// world can seat. Start over rather than grow: the guard re-arms on the
		// next action, so the worst case is a handful of extra assists.
		w.pairs = make(map[pairTile]pairBudget)
	}
}

// remember adds an action to the ring, overwriting the oldest entry once full.
// Caller must hold l.mu for writing.
func (w *worldLedger) remember(a recentAction) {
	if len(w.recent) < recentActionsPerWorld {
		w.recent = append(w.recent, a)
		return
	}
	w.recent[w.nextSlot] = a
	w.nextSlot = (w.nextSlot + 1) % recentActionsPerWorld
}

// tileOf maps a coordinate to its assist tile, flooring towards negative
// infinity so coordinates either side of zero do not share a tile.
func tileOf(v int) int {
	if v < 0 {
		return -((-v + AssistTile - 1) / AssistTile)
	}
	return v / AssistTile
}
