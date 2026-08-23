package tests

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/worldweaver/worldweaver/internal/game"
)

// Contribution ledger.
//
// The ledger exists to make cooperation visible: it records what each player put
// into a world and pays a bounded bonus when two players work the same ground at
// the same time. Most of the risk lives in that bonus, so most of these tests are
// about the edges of it — the radius, the window, and the guard that stops a pair
// farming the multiplier by poking one cell forever.

// contribAt returns a fixed base time. Every test drives the clock explicitly
// rather than sleeping: the assist window is 20 seconds, and a test suite that
// waits that out is a test suite nobody runs.
func contribAt() time.Time {
	return time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
}

// contribApprox compares scores, which are float64 products of a weight and a
// multiplier.
func contribApprox(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// contribPlace books a material placement at an explicit time.
func contribPlace(l *game.ContributionLedger, world string, player uint32, cat game.ContributionCategory, x, y, cells int, at time.Time) game.ContributionResult {
	return l.Record(world, game.ContributionAction{
		PlayerID: player,
		Category: cat,
		X:        x,
		Y:        y,
		Cells:    cells,
		At:       at,
	})
}

// TestContributionSoloScoreIsCellsTimesCategoryWeight is the base case everything else
// is measured against: one player, nobody nearby, no multiplier.
func TestContributionSoloScoreIsCellsTimesCategoryWeight(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	r1 := contribPlace(l, "w", 1, game.ContribTerrain, 100, 100, 10, base)
	r2 := contribPlace(l, "w", 1, game.ContribWater, 100, 100, 10, base.Add(time.Second))
	r3 := contribPlace(l, "w", 1, game.ContribLife, 100, 100, 10, base.Add(2*time.Second))

	if r1.Multiplier != 1 || r2.Multiplier != 1 || r3.Multiplier != 1 {
		t.Fatalf("solo work was multiplied: %v %v %v", r1.Multiplier, r2.Multiplier, r3.Multiplier)
	}

	want := 10*game.CategoryWeight(game.ContribTerrain) +
		10*game.CategoryWeight(game.ContribWater) +
		10*game.CategoryWeight(game.ContribLife)

	pc, ok := l.Player("w", 1)
	if !ok {
		t.Fatal("the player who just built has no ledger entry")
	}
	if !contribApprox(pc.Score, want) {
		t.Errorf("score %v, want %v", pc.Score, want)
	}
	if pc.Actions != 3 {
		t.Errorf("actions %d, want 3", pc.Actions)
	}
	if pc.Cells != 30 {
		t.Errorf("cells %d, want 30", pc.Cells)
	}
	if pc.CellsByCategory[game.ContribWater] != 10 || pc.CellsByCategory[game.ContribLife] != 10 {
		t.Errorf("cells were not split by category: %v", pc.CellsByCategory)
	}
	if pc.Assists != 0 {
		t.Errorf("assists %d, want 0 — nobody else was in the world", pc.Assists)
	}
}

// TestContributionSpanRecordsFirstAndLastSeen: the ledger is also the record of
// who was around when a world was built, so the span must widen with each action
// and never move backwards.
func TestContributionSpanRecordsFirstAndLastSeen(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 10, 10, 5, base)
	contribPlace(l, "w", 1, game.ContribTerrain, 10, 10, 5, base.Add(90*time.Second))
	// An out-of-order replay must not rewind the span.
	contribPlace(l, "w", 1, game.ContribTerrain, 10, 10, 5, base.Add(30*time.Second))

	pc, _ := l.Player("w", 1)
	if !pc.FirstSeen.Equal(base) {
		t.Errorf("firstSeen %v, want %v", pc.FirstSeen, base)
	}
	if !pc.LastSeen.Equal(base.Add(90 * time.Second)) {
		t.Errorf("lastSeen %v, want the latest action at %v", pc.LastSeen, base.Add(90*time.Second))
	}
}

// TestContributionAssistCreditsBothPlayersForNearbyWork is the collaboration mechanic
// itself: two players on the same hillside each end up recorded as the other's
// partner, and the second one's action is worth more than it would have been
// alone.
func TestContributionAssistCreditsBothPlayersForNearbyWork(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 100, 100, 10, base)
	joint := contribPlace(l, "w", 2, game.ContribTerrain, 120, 100, 10, base.Add(3*time.Second))

	if len(joint.Assisted) != 1 || joint.Assisted[0] != 1 {
		t.Fatalf("assisted %v, want [1]", joint.Assisted)
	}
	if joint.Multiplier <= 1 {
		t.Errorf("multiplier %v, want more than 1 for assisted work", joint.Multiplier)
	}
	if !contribApprox(joint.Awarded, joint.BaseScore*joint.Multiplier) {
		t.Errorf("awarded %v does not equal base %v times multiplier %v",
			joint.Awarded, joint.BaseScore, joint.Multiplier)
	}

	// The credit is mutual: the player who was already there gets the assist too,
	// otherwise arriving second would be the only way to be recognised.
	first, _ := l.Player("w", 1)
	second, _ := l.Player("w", 2)
	if first.Assists != 1 || first.Partners[2] != 1 {
		t.Errorf("player 1 assists=%d partners=%v, want one assist with player 2", first.Assists, first.Partners)
	}
	if second.Assists != 1 || second.Partners[1] != 1 {
		t.Errorf("player 2 assists=%d partners=%v, want one assist with player 1", second.Assists, second.Partners)
	}
	if second.AssistedActions != 1 {
		t.Errorf("assistedActions %d, want 1", second.AssistedActions)
	}
}

// TestContributionAssistedWorkOutscoresIdenticalSoloWork states the product requirement in
// numbers: the same effort must pay more when it is shared. Without this the
// motto is decoration.
func TestContributionAssistedWorkOutscoresIdenticalSoloWork(t *testing.T) {
	base := contribAt()

	solo := game.NewContributionLedger()
	contribPlace(solo, "w", 1, game.ContribTerrain, 100, 100, 20, base)
	alone, _ := solo.Player("w", 1)

	together := game.NewContributionLedger()
	contribPlace(together, "w", 9, game.ContribTerrain, 100, 100, 1, base)
	contribPlace(together, "w", 1, game.ContribTerrain, 100, 100, 20, base.Add(time.Second))
	helped, _ := together.Player("w", 1)

	if helped.Score <= alone.Score {
		t.Errorf("assisted score %v is not above solo score %v for identical work", helped.Score, alone.Score)
	}
}

// TestContributionAssistDeniedOutsideRadius: proximity is what makes it collaboration.
// Two players on opposite ends of a 4096-wide world are not helping each other,
// however close together their clicks fall in time.
func TestContributionAssistDeniedOutsideRadius(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 100, 100, 10, base)
	far := contribPlace(l, "w", 2, game.ContribTerrain, 100+game.AssistRadius+1, 100, 10, base.Add(time.Second))

	if len(far.Assisted) != 0 {
		t.Errorf("assisted %v, want none just outside the %d-cell radius", far.Assisted, game.AssistRadius)
	}
	if far.Multiplier != 1 {
		t.Errorf("multiplier %v, want 1", far.Multiplier)
	}
	if far.Throttled {
		t.Error("throttled reported for distant work; the guard must only fire on a genuine nearby pair")
	}
}

// TestContributionAssistDeniedOutsideTimeWindow: a player who shaped this spot and left half an
// hour ago must stop earning assists from whoever builds here next, or squatting
// would pay better than building.
func TestContributionAssistDeniedOutsideTimeWindow(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 100, 100, 10, base)
	late := contribPlace(l, "w", 2, game.ContribTerrain, 100, 100, 10, base.Add(game.AssistWindow+time.Second))

	if len(late.Assisted) != 0 {
		t.Errorf("assisted %v, want none outside the %v window", late.Assisted, game.AssistWindow)
	}
	if late.Multiplier != 1 {
		t.Errorf("multiplier %v, want 1", late.Multiplier)
	}
}

// TestContributionRepeatedPokingOfOneCellCannotFarmTheMultiplier is the exploit this design
// has to survive: two players sitting on a single cell, alternating clicks, each
// click assisted by the other, multiplying an endless stream of one-cell actions.
// The pair's budget for a patch of world runs out, and everything after that
// scores as solo work.
func TestContributionRepeatedPokingOfOneCellCannotFarmTheMultiplier(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	const pokes = 12
	multiplied := 0
	throttledLate := 0

	for i := 0; i < pokes; i++ {
		player := uint32(1 + i%2)
		r := contribPlace(l, "w", player, game.ContribTerrain, 10, 10, 1, base.Add(time.Duration(i)*time.Second))
		if r.Multiplier > 1 {
			multiplied++
		}
		if i >= 2*game.MaxPairAssistsPerTile && r.Throttled && r.Multiplier == 1 {
			throttledLate++
		}
	}

	if multiplied > game.MaxPairAssistsPerTile {
		t.Errorf("%d of %d pokes were multiplied; the pair budget of %d was not enforced",
			multiplied, pokes, game.MaxPairAssistsPerTile)
	}
	if throttledLate == 0 {
		t.Error("no late poke was reported as throttled; the pair can keep collecting credit on one cell")
	}

	// The total must stay near what the same clicks were worth solo, not scale
	// with how long the two are willing to sit there.
	total := l.World("w").TotalScore
	soloWorth := float64(pokes) * game.CategoryWeight(game.ContribTerrain)
	ceiling := soloWorth + float64(game.MaxPairAssistsPerTile)*game.CategoryWeight(game.ContribTerrain)*game.MaxCollaborationMultiplier
	if total > ceiling {
		t.Errorf("total score %v exceeds the bounded ceiling %v for %d one-cell pokes", total, ceiling, pokes)
	}
}

// TestContributionMovingToFreshGroundEarnsAssistAgain guards the other side of the
// anti-farm rule: it must throttle a pair squatting on one spot without punishing
// the pair who spent the afternoon building across the map together.
func TestContributionMovingToFreshGroundEarnsAssistAgain(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	// Burn the budget for the first worksite.
	for i := 0; i < 2*game.MaxPairAssistsPerTile+4; i++ {
		contribPlace(l, "w", uint32(1+i%2), game.ContribTerrain, 10, 10, 1, base.Add(time.Duration(i)*time.Second))
	}
	exhausted := contribPlace(l, "w", 1, game.ContribTerrain, 10, 10, 1, base.Add(20*time.Second))
	if exhausted.Multiplier > 1 {
		t.Fatalf("multiplier %v at the exhausted worksite; test premise is wrong", exhausted.Multiplier)
	}

	// Same pair, several tiles away.
	fresh := 5 * game.AssistTile
	contribPlace(l, "w", 1, game.ContribTerrain, fresh, 10, 10, base.Add(21*time.Second))
	moved := contribPlace(l, "w", 2, game.ContribTerrain, fresh+20, 10, 10, base.Add(22*time.Second))

	if len(moved.Assisted) != 1 || moved.Assisted[0] != 1 {
		t.Errorf("assisted %v, want [1]: moving to new ground must earn credit again", moved.Assisted)
	}
	if moved.Multiplier <= 1 {
		t.Errorf("multiplier %v at a fresh worksite, want more than 1", moved.Multiplier)
	}
}

// TestContributionCollaborationMultiplierIsCapped: with a full world crowded onto one spot
// the multiplier must stop climbing, so no arrangement of players turns it into
// an unbounded score source.
func TestContributionCollaborationMultiplierIsCapped(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	const crowd = 8
	for i := 0; i < crowd; i++ {
		contribPlace(l, "w", uint32(10+i), game.ContribTerrain, 200+i, 200, 1, base.Add(time.Duration(i)*time.Second))
	}
	last := contribPlace(l, "w", 99, game.ContribTerrain, 200, 200, 10, base.Add(crowd*time.Second))

	if len(last.Assisted) != crowd {
		t.Fatalf("assisted %d players, want %d", len(last.Assisted), crowd)
	}
	if last.Multiplier != game.MaxCollaborationMultiplier {
		t.Errorf("multiplier %v, want the cap %v", last.Multiplier, game.MaxCollaborationMultiplier)
	}
	if uncapped := 1 + game.AssistBonus*float64(crowd); last.Multiplier >= uncapped {
		t.Errorf("multiplier %v was not capped below the uncapped %v", last.Multiplier, uncapped)
	}
}

// TestContributionRosterIsOrderedByScore: the roster is what a client renders, so the order
// must come from the ledger rather than from Go's randomised map iteration.
func TestContributionRosterIsOrderedByScore(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	// Kept far apart so scores come purely from cells, with no assists in play.
	contribPlace(l, "w", 1, game.ContribTerrain, 0, 0, 10, base)
	contribPlace(l, "w", 2, game.ContribTerrain, 1000, 0, 50, base.Add(time.Second))
	contribPlace(l, "w", 3, game.ContribTerrain, 2000, 0, 30, base.Add(2*time.Second))

	roster := l.Roster("w")
	if len(roster) != 3 {
		t.Fatalf("roster has %d entries, want 3", len(roster))
	}
	want := []uint32{2, 3, 1}
	for i, id := range want {
		if roster[i].PlayerID != id {
			t.Fatalf("roster order %v, want %v", []uint32{roster[0].PlayerID, roster[1].PlayerID, roster[2].PlayerID}, want)
		}
	}

	// Repeated calls must agree; map iteration order would make them differ.
	for i := 0; i < 20; i++ {
		if l.Roster("w")[0].PlayerID != 2 {
			t.Fatal("roster order is not stable between calls")
		}
	}
}

// TestContributionMostCollaborativeIsTheBiggestAssister names the player the world should
// thank: the one who worked alongside others most, not the one who covered the
// most cells.
func TestContributionMostCollaborativeIsTheBiggestAssister(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	// Player 2 builds with player 1 in one place and with player 3 in another.
	// Players 1 and 3 never meet.
	contribPlace(l, "w", 1, game.ContribTerrain, 10, 10, 1, base)
	contribPlace(l, "w", 2, game.ContribTerrain, 30, 10, 1, base.Add(time.Second))
	contribPlace(l, "w", 3, game.ContribTerrain, 2000, 500, 1, base.Add(2*time.Second))
	contribPlace(l, "w", 2, game.ContribTerrain, 2020, 500, 1, base.Add(3*time.Second))

	// A loner who out-builds everyone must not win this particular award.
	contribPlace(l, "w", 4, game.ContribTerrain, 3500, 900, 500, base.Add(4*time.Second))

	id, assists, ok := l.MostCollaborative("w")
	if !ok {
		t.Fatal("no most-collaborative player after two separate collaborations")
	}
	if id != 2 {
		t.Errorf("most collaborative is player %d with %d assists, want player 2", id, assists)
	}
	if assists != 2 {
		t.Errorf("assists %d, want 2", assists)
	}
	if agg := l.World("w"); agg.MostCollaborative != 2 {
		t.Errorf("aggregate reports %d, want 2", agg.MostCollaborative)
	}
	if top := l.Roster("w")[0].PlayerID; top != 4 {
		t.Errorf("roster leader is %d, want the loner 4 — the two awards must be independent", top)
	}
}

// TestContributionMostCollaborativeAbsentBeforeAnyoneCooperates: a world where everyone has
// worked alone has no answer, and reporting player 0 as the winner would put a
// nonexistent player on screen.
func TestContributionMostCollaborativeAbsentBeforeAnyoneCooperates(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 0, 0, 10, base)
	contribPlace(l, "w", 2, game.ContribTerrain, 3000, 900, 10, base.Add(time.Second))

	if id, assists, ok := l.MostCollaborative("w"); ok {
		t.Errorf("reported player %d with %d assists in a world with no collaboration", id, assists)
	}
}

// TestContributionSharedMilestoneSumsEveryContributor: a milestone belongs to the world's
// population, so a hundred small contributions must advance it exactly as far as
// the same total from one player.
func TestContributionSharedMilestoneSumsEveryContributor(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "w", 1, game.ContribTerrain, 0, 0, 100, base)   // 100
	contribPlace(l, "w", 2, game.ContribWater, 1500, 0, 100, base)  // 150
	contribPlace(l, "w", 3, game.ContribLife, 3000, 900, 100, base) // 200

	var wantTotal float64
	for _, pc := range l.Roster("w") {
		wantTotal += pc.Score
	}

	half := l.Milestone("w", wantTotal*2)
	if !contribApprox(half.Progress, wantTotal) {
		t.Errorf("progress %v, want the sum of all contributors %v", half.Progress, wantTotal)
	}
	if half.Complete {
		t.Error("milestone reported complete at half progress")
	}
	if !contribApprox(half.Fraction, 0.5) {
		t.Errorf("fraction %v, want 0.5", half.Fraction)
	}
	if !contribApprox(half.Remaining, wantTotal) {
		t.Errorf("remaining %v, want %v", half.Remaining, wantTotal)
	}
	if half.Contributors != 3 {
		t.Errorf("contributors %d, want 3", half.Contributors)
	}

	reached := l.Milestone("w", wantTotal)
	if !reached.Complete {
		t.Error("milestone not complete when the population's total reached the target")
	}
	if reached.Remaining != 0 {
		t.Errorf("remaining %v, want 0 once complete", reached.Remaining)
	}
	if reached.Fraction > 1 {
		t.Errorf("fraction %v, want it clamped to 1", reached.Fraction)
	}

	// No player alone reaches it: the point of a shared target.
	if l.Roster("w")[0].Score >= wantTotal {
		t.Error("test premise is wrong: one player already covers the whole target")
	}
}

// TestContributionMilestoneOnUnknownWorldIsEmpty: a client can ask about a world that has
// seen no building yet, and that must read as zero progress rather than panic or
// divide by nothing.
func TestContributionMilestoneOnUnknownWorldIsEmpty(t *testing.T) {
	l := game.NewContributionLedger()

	m := l.Milestone("never-built", 100)
	if m.Progress != 0 || m.Complete || m.Contributors != 0 {
		t.Errorf("unknown world reported %+v, want empty progress", m)
	}
	if agg := l.World("never-built"); agg.TotalScore != 0 || len(agg.Contributors) != 0 {
		t.Errorf("unknown world aggregate %+v, want empty", agg)
	}
	if _, ok := l.Player("never-built", 1); ok {
		t.Error("unknown world returned a player entry")
	}
}

// TestContributionForgetWorldDropsAllState is the leak this needs to not have: worlds are
// created and deleted freely, and a ledger that keeps their rosters forever grows
// for the life of the process.
func TestContributionForgetWorldDropsAllState(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	contribPlace(l, "doomed", 1, game.ContribTerrain, 10, 10, 10, base)
	contribPlace(l, "doomed", 2, game.ContribTerrain, 20, 10, 10, base.Add(time.Second))
	contribPlace(l, "keeper", 1, game.ContribTerrain, 10, 10, 10, base)

	l.ForgetWorld("doomed")

	if l.WorldCount() != 1 {
		t.Errorf("worldCount %d after deleting one of two worlds, want 1", l.WorldCount())
	}
	if _, ok := l.Player("doomed", 1); ok {
		t.Error("deleted world still holds a contributor")
	}
	if l.ContributorCount("doomed") != 0 {
		t.Errorf("deleted world reports %d contributors", l.ContributorCount("doomed"))
	}
	if l.Roster("doomed") != nil {
		t.Error("deleted world still returns a roster")
	}
	if _, ok := l.Player("keeper", 1); !ok {
		t.Error("deleting one world took another world's state with it")
	}
}

// TestContributionPruneIdleKeepsActiveWorlds: the periodic sweep must only reclaim worlds
// nobody is building in, or a quiet moment would erase a live world's history.
func TestContributionPruneIdleKeepsActiveWorlds(t *testing.T) {
	l := game.NewContributionLedger()

	contribPlace(l, "live", 1, game.ContribTerrain, 10, 10, 10, time.Now())

	if removed := l.PruneIdle(time.Hour); removed != 0 {
		t.Errorf("pruned %d worlds that were just active", removed)
	}
	if l.WorldCount() != 1 {
		t.Fatalf("worldCount %d, want 1", l.WorldCount())
	}

	if removed := l.PruneIdle(time.Nanosecond); removed != 1 {
		t.Errorf("pruned %d worlds, want 1 once past the idle cutoff", removed)
	}
	if l.WorldCount() != 0 {
		t.Errorf("worldCount %d after pruning, want 0", l.WorldCount())
	}
}

// TestContributionTrackedWorldsAreBounded: players create worlds far more readily than they
// delete them, so the ledger must have a ceiling of its own rather than trusting
// every world to be forgotten explicitly.
func TestContributionTrackedWorldsAreBounded(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	const extra = 40
	for i := 0; i < game.MaxTrackedWorlds+extra; i++ {
		contribPlace(l, fmt.Sprintf("world-%d", i), 1, game.ContribTerrain, 10, 10, 1,
			base.Add(time.Duration(i)*time.Second))
	}

	if got := l.WorldCount(); got != game.MaxTrackedWorlds {
		t.Errorf("worldCount %d, want the bound %d", got, game.MaxTrackedWorlds)
	}
	// The coldest worlds are the ones that go.
	if _, ok := l.Player("world-0", 1); ok {
		t.Error("the least recently active world survived eviction")
	}
	newest := fmt.Sprintf("world-%d", game.MaxTrackedWorlds+extra-1)
	if _, ok := l.Player(newest, 1); !ok {
		t.Error("the most recent world was evicted; eviction is picking the wrong world")
	}
}

// TestContributionContributorsPerWorldAreBounded: a long-lived public world sees far more
// players than it seats, and its roster must not grow with every visitor it has
// ever had.
func TestContributionContributorsPerWorldAreBounded(t *testing.T) {
	l := game.NewContributionLedger()
	base := contribAt()

	const extra = 25
	for i := 0; i < game.MaxContributorsPerWorld+extra; i++ {
		contribPlace(l, "busy", uint32(i+1), game.ContribTerrain, 10, 10, 1,
			base.Add(time.Duration(i)*time.Second))
	}

	if got := l.ContributorCount("busy"); got != game.MaxContributorsPerWorld {
		t.Errorf("contributorCount %d, want the bound %d", got, game.MaxContributorsPerWorld)
	}
	if _, ok := l.Player("busy", 1); ok {
		t.Error("the contributor who acted longest ago survived eviction")
	}
}

// TestContributionUnknownIdsStillCount: material and power ids come off the wire, and an id
// this build does not recognise must land in a category rather than vanish —
// silently dropping contributions is worse than filing them under "other".
func TestContributionUnknownIdsStillCount(t *testing.T) {
	l := game.NewContributionLedger()

	if got := game.CategoryForMaterial(233); got != game.ContribOther {
		t.Errorf("unknown material mapped to %v, want other", got)
	}
	if got := game.CategoryForPower(200); got != game.ContribOther {
		t.Errorf("unknown power mapped to %v, want other", got)
	}

	l.RecordPlacement("w", 1, 233, 10, 10, 7)
	pc, ok := l.Player("w", 1)
	if !ok || pc.Cells != 7 {
		t.Errorf("placement of an unknown material was not recorded: %+v", pc)
	}
}

// TestContributionCategoriesMatchTheirElement keeps the two entry points
// agreeing: raining on a valley and painting water into it are both water.
func TestContributionCategoriesMatchTheirElement(t *testing.T) {
	if game.CategoryForPower(game.PowerRain) != game.ContribWater {
		t.Error("rain is not counted as a water contribution")
	}
	if game.CategoryForPower(game.PowerLife) != game.ContribLife {
		t.Error("the life power is not counted as a life contribution")
	}
	if game.CategoryForPower(game.PowerHeat) != game.ContribFire {
		t.Error("heat is not counted as a fire contribution")
	}
	// Hazards are worth least: flooding a world with void must not out-score a
	// player who spent the same effort growing something.
	if game.CategoryWeight(game.ContribHazard) >= game.CategoryWeight(game.ContribLife) {
		t.Error("hazard cells are worth as much as life; destruction would be the efficient play")
	}
}

// TestContributionConcurrentUseIsRaceFree: HTTP handlers, the websocket read pumps and the
// simulation loop all touch this at once. Run with -race; the counts also catch a
// lost update that a race detector alone would miss.
func TestContributionConcurrentUseIsRaceFree(t *testing.T) {
	l := game.NewContributionLedger()

	const writers = 8
	const perWriter = 250
	const worlds = 4

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for w := 0; w < worlds; w++ {
					id := fmt.Sprintf("w%d", w)
					l.Roster(id)
					l.World(id)
					l.Milestone(id, 1000)
					l.MostCollaborative(id)
					l.Player(id, 1)
					l.ContributorCount(id)
				}
			}
		}()
	}

	var writersWG sync.WaitGroup
	for i := 0; i < writers; i++ {
		writersWG.Add(1)
		go func(player uint32) {
			defer writersWG.Done()
			world := fmt.Sprintf("w%d", int(player)%worlds)
			for n := 0; n < perWriter; n++ {
				l.RecordPower(world, player, game.PowerRain, 100+n%50, 100, 2)
			}
		}(uint32(i + 1))
	}
	writersWG.Wait()
	close(stop)
	readers.Wait()

	totalActions := 0
	totalCells := 0
	for w := 0; w < worlds; w++ {
		agg := l.World(fmt.Sprintf("w%d", w))
		totalActions += agg.TotalActions
		totalCells += agg.TotalCells
	}
	if totalActions != writers*perWriter {
		t.Errorf("recorded %d actions, want %d — an update was lost", totalActions, writers*perWriter)
	}
	if totalCells != writers*perWriter*2 {
		t.Errorf("recorded %d cells, want %d", totalCells, writers*perWriter*2)
	}
}
