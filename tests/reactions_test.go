package tests

import (
	"testing"

	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// ── Table integrity ──────────────────────────────────────────────────────────

// The registry must be internally consistent. Every failure this catches would
// otherwise surface as inexplicable simulation behaviour rather than an error:
// an element that boils below its melting point is never liquid, and a transition
// naming an unregistered element turns cells into something with no behaviour.
func TestElementRegistryIsConsistent(t *testing.T) {
	if err := world.Validate(); err != nil {
		t.Fatalf("element registry invalid: %v", err)
	}
	if world.Count() < 40 {
		t.Errorf("only %d elements registered, expected at least 40", world.Count())
	}
	t.Logf("%d elements registered", world.Count())
}

// A reaction naming an unregistered element would transform cells into something
// with no definition. The validator also builds the lookup index, so a duplicate
// pair — where one row would silently win — fails here too.
func TestReactionTableIsConsistent(t *testing.T) {
	if err := simulation.ValidateReactions(); err != nil {
		t.Fatalf("reaction table invalid: %v", err)
	}
	rx := simulation.AllReactions()
	if len(rx) < 60 {
		t.Errorf("only %d reactions registered, expected at least 60", len(rx))
	}
	t.Logf("%d reactions registered", len(rx))
}

// Every reaction carries the real chemical equation it models. That is what lets
// the table be checked against chemistry instead of taste, and it is shown to the
// player, so a blank one is a defect rather than a cosmetic omission.
func TestEveryReactionDocumentsItsChemistry(t *testing.T) {
	for _, r := range simulation.AllReactions() {
		if r.Equation == "" {
			t.Errorf("reaction %d+%d has no equation recorded", r.A, r.B)
		}
	}
}

// Elements the player can select must be findable by symbol, because the drawer's
// search is the only practical way to navigate 44 entries.
func TestElementsHaveSearchableIdentity(t *testing.T) {
	symbols := map[string]bool{}
	for _, e := range world.All() {
		if e.Name == "" {
			t.Errorf("element %d has no name", e.ID)
		}
		if e.Symbol != "" {
			if symbols[e.Symbol] {
				t.Errorf("symbol %q is used by more than one element", e.Symbol)
			}
			symbols[e.Symbol] = true
		}
	}
}

// ── Reaction fixtures ────────────────────────────────────────────────────────

// reactionWorld builds a world of empty cells at a chosen ambient temperature.
//
// The argument is WHOLE degrees Celsius for readability; the simulation stores
// tenths, so it is converted here. Passing raw internal units by mistake would make
// every test run ten times colder than intended.
func reactionWorld(degreesC int16) *world.World {
	w := world.New(64, 64, 7)
	for i := range w.Material {
		w.Material[i] = world.MatEmpty
		w.Temperature[i] = degreesC * 10
	}
	return w
}

// degC converts whole degrees Celsius into the simulation's internal tenths unit.
func degC(d int16) int16 { return d * 10 }

// place puts a material at a coordinate and marks the cell dirty so its chunk
// stays awake.
func place(w *world.World, x, y int, mat uint8) {
	w.SetMaterial(x, y, mat)
}

// runReactions ticks the engine, keeping chunks awake so the pass is not skipped
// for sleeping. Reaction evaluation is spaced every few ticks, so callers need to
// allow more ticks than reactions expected.
func runReactions(w *world.World, ticks int) {
	tick(w, ticks)
}

// countOf reports how many cells hold a material.
func countOf(w *world.World, mat uint8) int {
	n := 0
	for _, m := range w.Material {
		if m == mat {
			n++
		}
	}
	return n
}

// anyOf reports whether a material is present at all.
func anyOf(w *world.World, mat uint8) bool {
	return countOf(w, mat) > 0
}

// ── The famous reactions ─────────────────────────────────────────────────────

// Sodium in water is the set piece of the whole element system: violently
// exothermic, needs no heat, and produces a flammable gas that its own released
// heat can then ignite. If this does not work, nothing does.
func TestSodiumExplodesInWater(t *testing.T) {
	w := reactionWorld(20)

	// A column of water with sodium pressed against it.
	for y := 30; y < 40; y++ {
		place(w, 32, y, world.MatWater)
		place(w, 31, y, world.ElSodium)
	}

	before := countOf(w, world.MatWater)
	runReactions(w, 400)

	if countOf(w, world.MatWater) >= before {
		t.Errorf("water was not consumed: %d before, %d after",
			before, countOf(w, world.MatWater))
	}
	if !anyOf(w, world.ElLye) && !anyOf(w, world.ElHydrogen) {
		t.Error("neither lye nor hydrogen produced — 2Na + 2H₂O → 2NaOH + H₂ " +
			"did not fire")
	}

	// The reaction is strongly exothermic, so the region must be hotter than the
	// 20 °C it started at.
	peak := int16(-32768)
	for _, temp := range w.Temperature {
		if temp > peak {
			peak = temp
		}
	}
	if peak <= degC(20) {
		t.Errorf("peak temperature %d °C: the reaction released no heat", peak)
	}
	t.Logf("lye=%d hydrogen=%d peak=%d °C",
		countOf(w, world.ElLye), countOf(w, world.ElHydrogen), peak)
}

// Potassium must be more violent than sodium, because it genuinely is. This
// asserts the ordering rather than an absolute, since the ordering is the part
// that comes from chemistry.
func TestPotassiumIsMoreViolentThanSodium(t *testing.T) {
	peakFor := func(metal uint8) int16 {
		w := reactionWorld(20)
		for y := 30; y < 40; y++ {
			place(w, 32, y, world.MatWater)
			place(w, 31, y, metal)
		}
		runReactions(w, 300)
		peak := int16(-32768)
		for _, temp := range w.Temperature {
			if temp > peak {
				peak = temp
			}
		}
		return peak
	}

	na := peakFor(world.ElSodium)
	k := peakFor(world.ElPotassium)

	if k < na {
		t.Errorf("potassium peaked at %d °C but sodium at %d °C: potassium is the "+
			"more reactive alkali metal and must release more heat", k, na)
	}
	t.Logf("sodium peak=%d °C, potassium peak=%d °C", na, k)
}

// Thermite is why the temperature unit had to change: it burns near 2500 °C, and
// it is self-sustaining because the heat it releases exceeds its own ignition
// threshold. That makes propagation emergent rather than coded.
func TestThermiteSelfPropagates(t *testing.T) {
	w := reactionWorld(20)

	// A thermite bar, with only the first cell brought up to ignition temperature.
	// Everything else starts cold, so any spread has to come from the reaction's own
	// released heat rather than from the igniter.
	for x := 20; x < 44; x++ {
		place(w, x, 32, world.ElThermite)
	}
	w.Temperature[w.Index(20, 32)] = degC(1600)

	before := countOf(w, world.ElThermite)
	runReactions(w, 600)
	after := countOf(w, world.ElThermite)

	if after >= before {
		t.Errorf("thermite did not burn: %d cells before, %d after", before, after)
	}
	if !anyOf(w, world.ElMoltenMet) {
		t.Error("no molten metal produced — 2Al + Fe₂O₃ → 2Fe + Al₂O₃ did not fire")
	}
	t.Logf("thermite consumed: %d → %d, molten metal=%d",
		before, after, countOf(w, world.ElMoltenMet))
}

// Magnesium burning in carbon dioxide is the counter-intuitive case that proves
// the table is grounded in real chemistry: CO₂ smothers almost every fire, but
// burning magnesium strips the oxygen straight out of it. No designer would invent
// this, so it is the strongest single check that the data is real.
func TestMagnesiumBurnsInCarbonDioxide(t *testing.T) {
	w := reactionWorld(600) // above magnesium's 473 °C ignition point

	for y := 30; y < 38; y++ {
		place(w, 32, y, world.ElMagnesium)
		place(w, 33, y, world.ElCO2)
	}

	runReactions(w, 400)

	if !anyOf(w, world.ElCarbon) {
		t.Error("no carbon produced: 2Mg + CO₂ → 2MgO + C did not fire. This is " +
			"the reaction that makes a CO₂ blanket feed a magnesium fire.")
	}
	if !anyOf(w, world.ElMagOxide) {
		t.Error("no metal oxide produced")
	}
	t.Logf("carbon=%d oxide=%d",
		countOf(w, world.ElCarbon), countOf(w, world.ElMagOxide))
}

// Nitrogen and CO₂ must starve an ordinary fire, which is the contrast that makes
// the magnesium case land. If CO₂ smothered nothing, magnesium burning in it would
// not be surprising.
func TestCarbonDioxideSmothersOrdinaryFire(t *testing.T) {
	w := reactionWorld(20)

	for y := 30; y < 40; y++ {
		place(w, 32, y, world.MatFire)
		w.Lifetime[w.Index(32, y)] = 3000 // long-lived, so decay is not the cause
		place(w, 33, y, world.ElCO2)
	}

	before := countOf(w, world.MatFire)
	runReactions(w, 300)
	after := countOf(w, world.MatFire)

	if after >= before {
		t.Errorf("fire was not starved by CO₂: %d cells before, %d after",
			before, after)
	}
	t.Logf("fire %d → %d under CO₂", before, after)
}

// Acid must dissolve reactive metals and leave noble ones alone. The asymmetry is
// the reactivity series, and it is what makes the acid interesting rather than a
// universal solvent.
func TestAcidDissolvesZincButNotGold(t *testing.T) {
	survives := func(metal uint8) bool {
		w := reactionWorld(20)
		for y := 30; y < 40; y++ {
			place(w, 32, y, world.ElAcid)
			place(w, 31, y, metal)
		}
		before := countOf(w, metal)
		runReactions(w, 500)
		return countOf(w, metal) >= before
	}

	if survives(world.ElZinc) {
		t.Error("zinc survived acid: Zn + 2HCl → ZnCl₂ + H₂ did not fire")
	}
	if !survives(world.ElGold) {
		t.Error("gold dissolved in plain acid. Gold resists HCl alone and needs " +
			"chlorine present, which is what the Catalyst field encodes.")
	}
}

// Gold must dissolve once chlorine is present — aqua regia in miniature. This is
// the only test of the catalyst mechanism, which nothing else exercises.
func TestGoldDissolvesOnlyWithChlorinePresent(t *testing.T) {
	w := reactionWorld(20)

	for y := 30; y < 40; y++ {
		place(w, 31, y, world.ElGold)
		place(w, 32, y, world.ElAcid)
		place(w, 33, y, world.ElChlorine) // catalyst, adjacent to the acid
	}

	before := countOf(w, world.ElGold)
	runReactions(w, 800)
	after := countOf(w, world.ElGold)

	if after >= before {
		t.Errorf("gold did not dissolve with chlorine present: %d → %d. The "+
			"catalyst gate may not be finding the chlorine.", before, after)
	}
	t.Logf("gold %d → %d with chlorine present", before, after)
}

// Acid and base must neutralise into salt and water. It is the reaction every
// player will try, and the products are the check that the table's chemistry is
// right rather than merely plausible.
func TestAcidAndLyeNeutralise(t *testing.T) {
	w := reactionWorld(20)

	for y := 30; y < 40; y++ {
		place(w, 32, y, world.ElAcid)
		place(w, 33, y, world.ElLye)
	}

	runReactions(w, 300)

	if !anyOf(w, world.ElSalt) && !anyOf(w, world.MatWater) {
		t.Error("neither salt nor water produced: HCl + NaOH → NaCl + H₂O did " +
			"not fire")
	}
	t.Logf("salt=%d water=%d",
		countOf(w, world.ElSalt), countOf(w, world.MatWater))
}

// Oxyhydrogen is the loudest reaction in the table and needs an ignition source,
// which distinguishes it from the alkali metals that need none.
func TestHydrogenAndOxygenNeedIgnition(t *testing.T) {
	build := func(ambient int16) *world.World {
		w := reactionWorld(ambient)
		for y := 30; y < 40; y++ {
			place(w, 32, y, world.ElHydrogen)
			place(w, 33, y, world.ElOxygen)
		}
		return w
	}

	// Cold: the mixture must sit there. Hydrogen and oxygen genuinely do not
	// react at room temperature without a spark.
	cold := build(20)
	coldBefore := countOf(cold, world.ElHydrogen)
	runReactions(cold, 200)
	if countOf(cold, world.ElHydrogen) < coldBefore/2 {
		t.Error("hydrogen and oxygen reacted at 20 °C without ignition; the " +
			"MinTemp gate is not holding")
	}

	// Hot: above the 500 °C threshold it must go.
	hot := build(700)
	hotBefore := countOf(hot, world.ElHydrogen)
	runReactions(hot, 200)
	if countOf(hot, world.ElHydrogen) >= hotBefore {
		t.Error("hydrogen and oxygen did not react at 700 °C: " +
			"2H₂ + O₂ → 2H₂O did not fire")
	}
	if !anyOf(hot, world.ElSteam) {
		t.Error("no steam produced by oxyhydrogen combustion")
	}
}

// White phosphorus ignites at 34 °C, below a warm room. Placing it in air is the
// whole demonstration, and it is the only element that needs no provocation at all.
func TestPhosphorusIgnitesSpontaneously(t *testing.T) {
	w := reactionWorld(40) // above the 34 °C ignition point

	for y := 30; y < 38; y++ {
		place(w, 32, y, world.ElPhosphorus)
		place(w, 33, y, world.ElOxygen)
	}

	runReactions(w, 300)

	if !anyOf(w, world.MatFire) {
		t.Error("phosphorus did not ignite in oxygen at 40 °C: " +
			"P₄ + 5O₂ → 2P₂O₅ did not fire")
	}
}

// The Haber process is the clearest catalyst case: nitrogen and hydrogen do not
// combine without iron present, which is historically why it took until 1909.
func TestHaberProcessRequiresIronCatalyst(t *testing.T) {
	build := func(withIron bool) *world.World {
		w := reactionWorld(500) // above the 450 °C threshold
		for y := 30; y < 40; y++ {
			place(w, 32, y, world.ElNitrogen)
			place(w, 33, y, world.ElHydrogen)
			if withIron {
				place(w, 34, y, world.ElIron)
			}
		}
		return w
	}

	without := build(false)
	runReactions(without, 600)
	if anyOf(without, world.ElAmmonia) {
		t.Error("ammonia formed without an iron catalyst; the catalyst gate is " +
			"not being enforced")
	}

	with := build(true)
	runReactions(with, 900)
	if !anyOf(with, world.ElAmmonia) {
		t.Error("ammonia did not form even with iron present: " +
			"N₂ + 3H₂ ⇌ 2NH₃ did not fire")
	}
}

// Iron must rust, and slowly. The slowness is the point — it is the one reaction
// in the table meant to be noticed over minutes rather than seconds.
func TestIronRustsInOxygen(t *testing.T) {
	w := reactionWorld(20)

	for y := 20; y < 50; y++ {
		for x := 30; x < 36; x++ {
			place(w, x, y, world.ElIron)
		}
		for x := 36; x < 40; x++ {
			place(w, x, y, world.ElOxygen)
		}
	}

	runReactions(w, 3000)

	if !anyOf(w, world.ElRust) {
		t.Error("iron did not rust in oxygen: 4Fe + 3O₂ → 2Fe₂O₃ did not fire")
	}
	t.Logf("rust cells after 3000 ticks: %d", countOf(w, world.ElRust))
}

// ── Cascade bounds ───────────────────────────────────────────────────────────

// An exothermic chain must not run away. Before the ceiling, two mutually
// exothermic reactions in contact saturated int16 and wrapped, which reads as the
// world instantly freezing and is very hard to trace to its cause.
func TestExothermicCascadeIsBounded(t *testing.T) {
	w := reactionWorld(20)

	// A large block of thermite, fully ignited, is the worst case for runaway.
	for y := 20; y < 44; y++ {
		for x := 20; x < 44; x++ {
			place(w, x, y, world.ElThermite)
			w.Temperature[w.Index(x, y)] = degC(1600)
		}
	}

	runReactions(w, 900)

	for i, temp := range w.Temperature {
		if temp > 32000 {
			t.Fatalf("cell %d reached %d (internal tenths), above the 32000 cascade ceiling", i, temp)
		}
		if temp < -2730 {
			t.Fatalf("cell %d reached %d (internal tenths), below absolute zero — the temperature "+
				"almost certainly wrapped through int16", i, temp)
		}
	}
}

// Reactions must not resurrect a world that has finished burning. If temperature
// never decayed, one ignition would leave the map permanently hot.
func TestWorldCoolsAfterReactionsFinish(t *testing.T) {
	w := reactionWorld(20)

	for y := 30; y < 34; y++ {
		for x := 30; x < 34; x++ {
			place(w, x, y, world.ElThermite)
			w.Temperature[w.Index(x, y)] = degC(1600)
		}
	}

	runReactions(w, 200)
	peak := int16(-32768)
	for _, temp := range w.Temperature {
		if temp > peak {
			peak = temp
		}
	}

	// Long enough for the environment pass to carry the heat away.
	runReactions(w, 4000)
	final := int16(-32768)
	for _, temp := range w.Temperature {
		if temp > final {
			final = temp
		}
	}

	if final >= peak {
		t.Errorf("world did not cool: peaked at %d °C and is still at %d °C",
			peak, final)
	}
	t.Logf("peak %d °C cooled to %d °C", peak, final)
}

// ── Player-facing queries ────────────────────────────────────────────────────

// The drawer shows a player what an element reacts with. That query has to
// actually return something for the elements a player will click first.
func TestReactionsAreDiscoverablePerElement(t *testing.T) {
	for _, id := range []uint8{
		world.ElSodium, world.ElAcid, world.ElOxygen,
		world.ElMagnesium, world.ElIron,
	} {
		rx := simulation.ReactionsFor(id)
		if len(rx) == 0 {
			e, _ := world.Lookup(id)
			name := "unknown"
			if e != nil {
				name = e.Name
			}
			t.Errorf("%s (%d) has no discoverable reactions, so the drawer would "+
				"show a player nothing", name, id)
		}
	}
}

// Reactivity zero is the gate that keeps the pass affordable. If terrain were
// reactive the pass would walk neighbours for most of the grid.
func TestInertTerrainSkipsTheReactionPass(t *testing.T) {
	for _, id := range []uint8{world.MatRock, world.MatSoil, world.MatSand} {
		if world.ReactivityOf(id) != 0 {
			t.Errorf("%s has non-zero reactivity, so the reaction pass will walk "+
				"its neighbours across most of the world",
				world.MaterialName(id))
		}
	}
}
