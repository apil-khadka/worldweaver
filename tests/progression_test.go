package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/game"
)

// cellsInRadius approximates the cell count a brush of the given radius covers,
// matching how the server counts affected cells.
func cellsInRadius(r int) int {
	n := 0
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				n++
			}
		}
	}
	return n
}

// The curve must grow super-linearly. The old one was five levels at
// [0, 100, 500, 2000, 10000] — about 5x per step over five levels — against scoring
// that grew as the square of the brush radius, so it was overwhelmed immediately.
func TestLevelCurveGrowsExponentially(t *testing.T) {
	if len(game.LevelThresholds) < 20 {
		t.Fatalf("only %d levels defined; the curve needs a long tail to climb",
			len(game.LevelThresholds))
	}

	// Each gap must be strictly larger than the one before it.
	for i := 3; i < len(game.LevelThresholds); i++ {
		prevGap := game.LevelThresholds[i-1].Score - game.LevelThresholds[i-2].Score
		gap := game.LevelThresholds[i].Score - game.LevelThresholds[i-1].Score
		if gap <= prevGap {
			t.Errorf("level %d needs %d score but level %d needed %d: the curve is "+
				"not accelerating", i+1, gap, i, prevGap)
		}
	}
}

// Level 2 must be close enough to feel responsive. A progression whose first step
// takes an hour reads as broken just as much as one that takes a second.
func TestLevelTwoIsReachableQuickly(t *testing.T) {
	l2 := game.LevelThresholds[1].Score
	if l2 > 200 {
		t.Errorf("level 2 costs %d score, which is too far for a first step", l2)
	}
	if l2 < 10 {
		t.Errorf("level 2 costs %d score, which is close enough to free that it "+
			"teaches the player nothing", l2)
	}
}

// The headline defect: holding one power on one spot reached max level in about two
// seconds. This asserts the fix by simulating exactly that abuse.
func TestHoldingOnePowerOnOneSpotDoesNotRaceUpLevels(t *testing.T) {
	sb := game.NewScoreboard()
	const world = "t"
	const pid = uint32(1)

	// One minute of holding the button at the client's 8 applications/second, all at
	// the same coordinate, with a radius-8 brush.
	cells := cellsInRadius(8)
	for i := 0; i < 8*60; i++ {
		sb.RecordPowerActionAt(world, pid, game.PowerRain, cells, 0.03, 500, 300)
	}

	score := sb.ScoreOf(world, pid)
	level := game.LevelForScore(score)

	t.Logf("one minute of holding one spot: score=%d level=%d", score, level)

	if level > 3 {
		t.Errorf("reached level %d (score %d) by holding one power on one spot for "+
			"a minute; the repetition and rate rules are not damping it", level, score)
	}
}

// Brush radius must not be a score multiplier. Before the fix a radius-24 brush
// scored 9x a radius-8 one per application, because reward was per cell.
func TestBrushRadiusDoesNotMultiplyScore(t *testing.T) {
	scoreFor := func(radius int) int {
		sb := game.NewScoreboard()
		cells := cellsInRadius(radius)
		// Spread out so the repetition rule does not confound the comparison.
		for i := 0; i < 20; i++ {
			sb.RecordPowerActionAt("t", 1, game.PowerRain, cells, 0.03, i*100, 0)
		}
		return sb.ScoreOf("t", 1)
	}

	small := scoreFor(4)
	large := scoreFor(24)

	if small == 0 {
		t.Fatal("a small brush earned nothing at all")
	}

	ratio := float64(large) / float64(small)
	t.Logf("radius 4 → %d, radius 24 → %d (ratio %.2fx)", small, large, ratio)

	// Area ratio between radius 4 and 24 is 36x. A sqrt area term should keep the
	// score ratio near 6x at the very most; anything approaching the area ratio
	// means reward is still per-cell.
	if ratio > 4.0 {
		t.Errorf("a radius-24 brush scores %.2fx a radius-4 brush; reward is still "+
			"scaling with area rather than with the action", ratio)
	}
}

// Working across the world must beat drilling one hole, so the incentive points at
// interesting play rather than at the most repetitive possible input.
func TestMovingAroundOutscoresRepeatingOneSpot(t *testing.T) {
	cells := cellsInRadius(8)

	still := game.NewScoreboard()
	for i := 0; i < 60; i++ {
		still.RecordPowerActionAt("t", 1, game.PowerRain, cells, 0.03, 500, 300)
	}

	moving := game.NewScoreboard()
	for i := 0; i < 60; i++ {
		moving.RecordPowerActionAt("t", 1, game.PowerRain, cells, 0.03, i*80, 300)
	}

	stillScore := still.ScoreOf("t", 1)
	movingScore := moving.ScoreOf("t", 1)

	t.Logf("same spot → %d, moving around → %d", stillScore, movingScore)

	if movingScore <= stillScore {
		t.Errorf("moving around scored %d but repeating one spot scored %d: the "+
			"incentive is pointing at the most boring possible input",
			movingScore, stillScore)
	}
}

// The rate rule must bound the ceiling regardless of technique, so no input pattern
// can earn without limit.
func TestSustainedSpammingIsRateLimited(t *testing.T) {
	sb := game.NewScoreboard()
	cells := cellsInRadius(8)

	// 300 well-spread actions inside one minute — far beyond human play, and beyond
	// the 60-action window where full and half rates apply.
	for i := 0; i < 300; i++ {
		sb.RecordPowerActionAt("t", 1, game.PowerRain, cells, 0.03, (i*37)%1000, (i*53)%600)
	}
	spammed := sb.ScoreOf("t", 1)

	// The same count spread over enough windows that damping does not apply.
	reference := game.NewScoreboard()
	for i := 0; i < 30; i++ {
		reference.RecordPowerActionAt("t", 1, game.PowerRain, cells, 0.03, (i*37)%1000, (i*53)%600)
	}
	fair := reference.ScoreOf("t", 1)

	t.Logf("300 actions in one minute → %d; 30 actions → %d", spammed, fair)

	// 10x the actions must not produce anywhere near 10x the score.
	if float64(spammed) > float64(fair)*4 {
		t.Errorf("300 actions scored %d against %d for 30: the rate rule is not "+
			"bounding sustained spam", spammed, fair)
	}
}

// Every level must be reachable, and the unlock schedule must move forward rather
// than regress at any step.
func TestUnlocksNeverRegress(t *testing.T) {
	for i := 1; i < len(game.LevelThresholds); i++ {
		prev, cur := game.LevelThresholds[i-1], game.LevelThresholds[i]
		if cur.PowerRadius < prev.PowerRadius {
			t.Errorf("level %d has a smaller power radius (%d) than level %d (%d)",
				i+1, cur.PowerRadius, i, prev.PowerRadius)
		}
		if cur.MaxInfluence < prev.MaxInfluence {
			t.Errorf("level %d has less max influence (%.0f) than level %d (%.0f)",
				i+1, cur.MaxInfluence, i, prev.MaxInfluence)
		}
		if cur.Score <= prev.Score {
			t.Errorf("level %d threshold %d is not above level %d's %d",
				i+1, cur.Score, i, prev.Score)
		}
	}
}

// The Life power gate at level 4 is referenced by the client, so the level it sits
// at is a contract rather than a tuning value.
func TestLifePowerGateStaysAtLevelFour(t *testing.T) {
	if game.LevelThresholds[3].Score == 0 {
		t.Fatal("level 4 has no threshold, so the Life power would be free")
	}
	l4 := game.LevelThresholds[3].Score
	if l4 < 100 || l4 > 5000 {
		t.Errorf("level 4 costs %d score; the Life power unlock should be an "+
			"early-game goal, not trivial or remote", l4)
	}
}
