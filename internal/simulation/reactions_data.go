// Reaction data — real chemistry, one row per pair.
//
// Every row carries the actual chemical equation it models. That is not decoration:
// it is how the table gets checked against reality instead of taste, and it is what
// the client shows a player in the element drawer.
//
// Reactions were chosen for being legible in a 2D grid of cells. A reaction whose
// only visible result is a colour shift is not worth a row; one that produces a gas,
// a flame, or a phase change reads immediately.
//
// The set deliberately includes counter-intuitive real cases. Magnesium burns IN
// carbon dioxide, so a CO2 blanket makes a magnesium fire worse — no designer would
// invent that, and it is the clearest evidence the table is grounded in chemistry
// rather than assembled by feel. Likewise gold resists acid alone but dissolves once
// chlorine is present, and copper does not react with plain acid at all.
//
// HeatDelta is molar enthalpy scaled into a per-cell temperature delta, not the
// enthalpy itself: 852 kJ/mol for thermite is not 852 degrees.
package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// celsius converts whole degrees Celsius into the simulation's internal
// temperature unit.
//
// The simulation stores temperature as TENTHS of a degree in an int16, a
// convention that predates this table (ice melts at 200, meaning 20.0 °C). Writing
// raw numbers here would silently mix units: a threshold of 1500 would mean 150 °C,
// not 1500 °C, so thermite would ignite in a warm room and the whole table would be
// wrong by a factor of ten in a way no test would obviously catch.
//
// Every threshold in this file therefore goes through celsius(). The design in
// .kiro/specs/atomic-elements/design.md §2.1 calls for changing the unit to whole
// degrees, which is a breaking snapshot change scheduled as its own phase; until
// then this helper is the single place the conversion happens.
//
// Note the ceiling this implies: int16 tenths caps at 3276 °C, which is why
// cascadeCeiling sits at 3200 °C rather than somewhere above tungsten's 5555 °C
// boiling point.
func celsius(deg int16) int16 {
	return deg * 10
}

// Reaction speed, as 1-in-N rolls per adjacency check.
const (
	rxInstant  = 1  // every check — reserved for detonations
	rxViolent  = 3
	rxFast     = 12
	rxModerate = 40
	rxSlow     = 200
	rxCreeping = 1200 // rusting, tarnishing: visible over minutes
)

// Heat released, as degrees Celsius added to surrounding cells. Scaled from molar
// enthalpy so the relative ordering matches reality even though the absolute values
// are game units.
// Heat released or absorbed, in the simulation's internal tenths-of-a-degree unit.
//
// Scaled from molar enthalpy rather than used directly: thermite's 852 kJ/mol is
// not 852 degrees. What is preserved is the ORDERING, which is what comes from
// chemistry — potassium in water must out-release sodium in water, and both must
// sit below thermite.
var (
	heatMild     = celsius(40)
	heatWarm     = celsius(120)
	heatHot      = celsius(350)
	heatFierce   = celsius(900)
	heatInferno  = celsius(1600)
	coolEndoMild = celsius(-60)
	coolEndoHard = celsius(-200)
)

func init() {
	reactionTable = []Reaction{
		// ── Alkali metals in water ───────────────────────────────────────────
		//
		// The set piece. No heat needed, violently exothermic, produces a
		// flammable gas that the released heat can then ignite — a two-stage
		// chain that falls straight out of the data.
		{
			A: world.ElSodium, B: world.MatWater,
			ProductA: world.ElLye, ProductB: world.ElHydrogen,
			MinTemp: world.TempNone, HeatDelta: heatFierce, Chance: rxViolent,
			Equation: "2Na + 2H₂O → 2NaOH + H₂",
		},
		{
			A: world.ElPotassium, B: world.MatWater,
			ProductA: world.ElLye, ProductB: world.ElHydrogen,
			MinTemp: world.TempNone, HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "2K + 2H₂O → 2KOH + H₂",
		},
		{
			A: world.ElCalcium, B: world.MatWater,
			ProductA: world.ElLye, ProductB: world.ElHydrogen,
			MinTemp: world.TempNone, HeatDelta: heatWarm, Chance: rxFast,
			Equation: "Ca + 2H₂O → Ca(OH)₂ + H₂",
		},
		{
			// Magnesium needs hot water or steam — it is stable in cold water,
			// which is why the threshold matters.
			A: world.ElMagnesium, B: world.MatWater,
			ProductA: world.ElMagOxide, ProductB: world.ElHydrogen,
			MinTemp: celsius(80), HeatDelta: heatHot, Chance: rxModerate,
			Equation: "Mg + 2H₂O → Mg(OH)₂ + H₂",
		},
		{
			// Ethanol is the calmer alkali reaction, and demonstrates that the
			// solvent matters, not just the metal.
			A: world.ElSodium, B: world.ElEthanol,
			ProductA: world.ElLye, ProductB: world.ElHydrogen,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxFast,
			Equation: "2Na + 2C₂H₅OH → 2NaOC₂H₅ + H₂",
		},

		// ── Combustion with oxygen ──────────────────────────────────────────
		{
			A: world.ElHydrogen, B: world.ElOxygen,
			ProductA: world.ElSteam, ProductB: world.ElSteam,
			MinTemp: celsius(500), HeatDelta: heatInferno, Chance: rxInstant,
			Equation: "2H₂ + O₂ → 2H₂O",
		},
		{
			A: world.ElCarbon, B: world.ElOxygen,
			ProductA: world.ElCO2, ProductB: world.ElCO2,
			MinTemp: celsius(700), HeatDelta: heatFierce, Chance: rxFast,
			Equation: "C + O₂ → CO₂",
		},
		{
			A: world.ElMethane, B: world.ElOxygen,
			ProductA: world.ElCO2, ProductB: world.ElSteam,
			MinTemp: celsius(580), HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "CH₄ + 2O₂ → CO₂ + 2H₂O",
		},
		{
			A: world.ElSulfur, B: world.ElOxygen,
			ProductA: world.MatFire, ProductB: world.MatSmoke,
			MinTemp: celsius(250), HeatDelta: heatHot, Chance: rxFast,
			Equation: "S + O₂ → SO₂",
		},
		{
			A: world.ElMagnesium, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatFire,
			MinTemp: celsius(473), HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "2Mg + O₂ → 2MgO",
		},
		{
			// White phosphorus ignites at 34 °C, which is below room temperature
			// on a warm day. Placing it in air is the whole demonstration.
			A: world.ElPhosphorus, B: world.ElOxygen,
			ProductA: world.MatFire, ProductB: world.MatSmoke,
			MinTemp: celsius(34), HeatDelta: heatFierce, Chance: rxViolent,
			Equation: "P₄ + 5O₂ → 2P₂O₅",
		},
		{
			A: world.ElEthanol, B: world.ElOxygen,
			ProductA: world.ElCO2, ProductB: world.ElSteam,
			MinTemp: celsius(363), HeatDelta: heatFierce, Chance: rxFast,
			Equation: "C₂H₅OH + 3O₂ → 2CO₂ + 3H₂O",
		},
		{
			A: world.MatOil, B: world.ElOxygen,
			ProductA: world.MatFire, ProductB: world.MatSmoke,
			MinTemp: celsius(280), HeatDelta: heatFierce, Chance: rxFast,
			Equation: "2C₈H₁₈ + 25O₂ → 16CO₂ + 18H₂O",
		},
		{
			A: world.ElZinc, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatSmoke,
			MinTemp: celsius(460), HeatDelta: heatHot, Chance: rxModerate,
			Equation: "2Zn + O₂ → 2ZnO",
		},
		{
			A: world.ElTitanium, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatFire,
			MinTemp: celsius(1200), HeatDelta: heatFierce, Chance: rxModerate,
			Equation: "Ti + O₂ → TiO₂",
		},
		{
			A: world.ElCalcium, B: world.ElOxygen,
			ProductA: world.ElQuicklime, ProductB: world.MatFire,
			MinTemp: celsius(842), HeatDelta: heatHot, Chance: rxModerate,
			Equation: "2Ca + O₂ → 2CaO",
		},
		{
			A: world.ElSodium, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatFire,
			MinTemp: celsius(115), HeatDelta: heatHot, Chance: rxFast,
			Equation: "2Na + O₂ → Na₂O₂",
		},
		{
			A: world.ElPotassium, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatFire,
			MinTemp: celsius(65), HeatDelta: heatHot, Chance: rxViolent,
			Equation: "K + O₂ → KO₂",
		},
		{
			A: world.ElCopper, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatEmpty,
			MinTemp: celsius(300), HeatDelta: heatMild, Chance: rxSlow,
			Equation: "2Cu + O₂ → 2CuO",
		},
		{
			A: world.ElTungsten, B: world.ElOxygen,
			ProductA: world.ElMagOxide, ProductB: world.MatEmpty,
			MinTemp: celsius(400), HeatDelta: heatWarm, Chance: rxCreeping,
			Equation: "2W + 3O₂ → 2WO₃",
		},
		{
			A: world.ElGunpowder, B: world.MatFire,
			ProductA: world.MatFire, ProductB: world.MatSmoke,
			MinTemp: celsius(300), HeatDelta: heatInferno, Chance: rxInstant,
			Equation: "2KNO₃ + S + 3C → K₂S + N₂ + 3CO₂",
		},

		// ── Rusting ─────────────────────────────────────────────────────────
		//
		// Slow on purpose: the point is that iron left in damp air degrades over
		// minutes, which is a different feel from every other reaction here.
		{
			A: world.ElIron, B: world.ElOxygen,
			ProductA: world.ElRust, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxCreeping,
			Equation: "4Fe + 3O₂ → 2Fe₂O₃",
		},
		{
			A: world.ElIron, B: world.MatWater,
			ProductA: world.ElRust, ProductB: world.MatWater,
			MinTemp: world.TempNone, HeatDelta: 0, Chance: rxCreeping,
			Equation: "4Fe + 3O₂ + 6H₂O → 4Fe(OH)₃",
		},

		// ── Thermite ────────────────────────────────────────────────────────
		//
		// The reaction the temperature-unit change was made for. Strongly
		// exothermic and self-sustaining: the heat it releases exceeds its own
		// ignition threshold, so a mass propagates without any propagation code.
		{
			A: world.ElAluminium, B: world.ElRust,
			ProductA: world.ElMoltenMet, ProductB: world.ElMagOxide,
			MinTemp: celsius(1500), HeatDelta: heatInferno, Chance: rxFast,
			Equation: "2Al + Fe₂O₃ → 2Fe + Al₂O₃",
		},
		{
			A: world.ElThermite, B: world.MatFire,
			ProductA: world.ElMoltenMet, ProductB: world.MatFire,
			MinTemp: celsius(1500), HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "2Al + Fe₂O₃ → 2Fe + Al₂O₃ (premixed)",
		},
		{
			A: world.ElThermite, B: world.MatLava,
			ProductA: world.ElMoltenMet, ProductB: world.MatLava,
			MinTemp: world.TempNone, HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "thermite ignited by contact with lava",
		},
		{
			// The reaction FRONT. Thermite is premixed fuel and oxidiser, so once
			// any of it is above the ignition threshold it consumes the mixture
			// next to it — this is how a real thermite charge burns through itself
			// rather than needing a flame held against every grain.
			//
			// Without this row, propagation depended on an external igniter staying
			// in contact, and lava flows away within a tick or two, so a charge lit
			// by lava went out immediately.
			A: world.ElThermite, B: world.ElThermite,
			ProductA: world.ElMoltenMet, ProductB: world.ElThermite,
			MinTemp: celsius(1500), HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "2Al + Fe₂O₃ → 2Fe + Al₂O₃ (reaction front)",
		},

		// ── Magnesium in carbon dioxide ─────────────────────────────────────
		//
		// The counter-intuitive one. CO2 extinguishes almost everything, but
		// burning magnesium strips the oxygen straight out of it and leaves
		// carbon behind, so smothering a magnesium fire in CO2 feeds it.
		{
			A: world.ElMagnesium, B: world.ElCO2,
			ProductA: world.ElMagOxide, ProductB: world.ElCarbon,
			MinTemp: celsius(473), HeatDelta: heatFierce, Chance: rxFast,
			Equation: "2Mg + CO₂ → 2MgO + C",
		},

		// ── Halogens ────────────────────────────────────────────────────────
		{
			A: world.ElSodium, B: world.ElChlorine,
			ProductA: world.ElSalt, ProductB: world.ElSalt,
			MinTemp: world.TempNone, HeatDelta: heatFierce, Chance: rxViolent,
			Equation: "2Na + Cl₂ → 2NaCl",
		},
		{
			A: world.ElHydrogen, B: world.ElChlorine,
			ProductA: world.ElAcid, ProductB: world.ElAcid,
			MinTemp: celsius(200), HeatDelta: heatHot, Chance: rxViolent,
			Equation: "H₂ + Cl₂ → 2HCl",
		},
		{
			A: world.ElIron, B: world.ElChlorine,
			ProductA: world.ElRust, ProductB: world.MatEmpty,
			MinTemp: celsius(200), HeatDelta: heatHot, Chance: rxModerate,
			Equation: "2Fe + 3Cl₂ → 2FeCl₃",
		},
		{
			A: world.ElCopper, B: world.ElChlorine,
			ProductA: world.ElMagOxide, ProductB: world.MatEmpty,
			MinTemp: celsius(400), HeatDelta: heatWarm, Chance: rxModerate,
			Equation: "Cu + Cl₂ → CuCl₂",
		},
		{
			A: world.ElAluminium, B: world.ElBromine,
			ProductA: world.ElMagOxide, ProductB: world.MatFire,
			MinTemp: world.TempNone, HeatDelta: heatFierce, Chance: rxFast,
			Equation: "2Al + 3Br₂ → 2AlBr₃",
		},

		// ── Acid on metals ──────────────────────────────────────────────────
		//
		// The reactivity series made visible: zinc and iron dissolve, copper and
		// gold do not. That asymmetry is real and is the reason the table
		// includes the non-reactions as absences rather than as rows.
		{
			A: world.ElAcid, B: world.ElZinc,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatWarm, Chance: rxFast,
			Equation: "Zn + 2HCl → ZnCl₂ + H₂",
		},
		{
			A: world.ElAcid, B: world.ElIron,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxModerate,
			Equation: "Fe + 2HCl → FeCl₂ + H₂",
		},
		{
			A: world.ElAcid, B: world.ElMagnesium,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatHot, Chance: rxViolent,
			Equation: "Mg + 2HCl → MgCl₂ + H₂",
		},
		{
			A: world.ElAcid, B: world.ElAluminium,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatWarm, Chance: rxModerate,
			Equation: "2Al + 6HCl → 2AlCl₃ + 3H₂",
		},
		{
			A: world.ElAcid, B: world.ElCalcium,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatWarm, Chance: rxFast,
			Equation: "Ca + 2HCl → CaCl₂ + H₂",
		},
		{
			A: world.ElAcid, B: world.ElSodium,
			ProductA: world.ElHydrogen, ProductB: world.ElSalt,
			MinTemp: world.TempNone, HeatDelta: heatInferno, Chance: rxViolent,
			Equation: "2Na + 2HCl → 2NaCl + H₂",
		},
		{
			A: world.ElAcid, B: world.ElTin,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxSlow,
			Equation: "Sn + 2HCl → SnCl₂ + H₂",
		},
		{
			A: world.ElAcid, B: world.ElLead,
			ProductA: world.ElHydrogen, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxCreeping,
			Equation: "Pb + 2HCl → PbCl₂ + H₂",
		},
		{
			// Gold dissolves ONLY with chlorine present — aqua regia in miniature.
			// The catalyst field is what makes this expressible without code.
			A: world.ElAcid, B: world.ElGold,
			ProductA: world.ElMoltenMet, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, Catalyst: world.ElChlorine,
			HeatDelta: heatMild, Chance: rxSlow,
			Equation: "2Au + 3Cl₂ + 2HCl → 2HAuCl₄",
		},

		// ── Acid and base ───────────────────────────────────────────────────
		{
			A: world.ElAcid, B: world.ElLye,
			ProductA: world.ElSalt, ProductB: world.MatWater,
			MinTemp: world.TempNone, HeatDelta: heatWarm, Chance: rxViolent,
			Equation: "HCl + NaOH → NaCl + H₂O",
		},
		{
			A: world.ElLye, B: world.MatOil,
			ProductA: world.ElSoap, ProductB: world.ElSoap,
			MinTemp: celsius(80), HeatDelta: heatMild, Chance: rxModerate,
			Equation: "NaOH + fat → glycerol + soap",
		},
		{
			A: world.ElLye, B: world.MatSand,
			ProductA: world.ElGlass, ProductB: world.MatEmpty,
			MinTemp: celsius(300), HeatDelta: heatMild, Chance: rxSlow,
			Equation: "SiO₂ + 2NaOH → Na₂SiO₃ + H₂O",
		},

		// ── Salt and water ──────────────────────────────────────────────────
		{
			// Endothermic: dissolving salt genuinely cools the water slightly.
			A: world.ElSalt, B: world.MatWater,
			ProductA: world.MatWater, ProductB: world.MatWater,
			MinTemp: world.TempNone, HeatDelta: coolEndoMild, Chance: rxModerate,
			Equation: "NaCl(s) → Na⁺(aq) + Cl⁻(aq)",
		},

		// ── Smelting and industrial chemistry ───────────────────────────────
		{
			// Endothermic: smelting consumes heat, so it needs a sustained
			// external source rather than self-sustaining like thermite.
			A: world.ElCarbon, B: world.ElRust,
			ProductA: world.ElMoltenMet, ProductB: world.ElCO2,
			MinTemp: celsius(700), HeatDelta: coolEndoHard, Chance: rxModerate,
			Equation: "2Fe₂O₃ + 3C → 4Fe + 3CO₂",
		},
		{
			A: world.ElCarbon, B: world.ElSteam,
			ProductA: world.ElHydrogen, ProductB: world.ElCO2,
			MinTemp: celsius(1000), HeatDelta: coolEndoHard, Chance: rxModerate,
			Equation: "C + H₂O → CO + H₂",
		},
		{
			A: world.ElIron, B: world.ElSteam,
			ProductA: world.ElMagnetite, ProductB: world.ElHydrogen,
			MinTemp: celsius(570), HeatDelta: coolEndoMild, Chance: rxSlow,
			Equation: "3Fe + 4H₂O → Fe₃O₄ + 4H₂",
		},
		{
			// The Haber process — the one reaction here that genuinely needs a
			// catalyst, and the reason the Catalyst field exists.
			A: world.ElNitrogen, B: world.ElHydrogen,
			ProductA: world.ElAmmonia, ProductB: world.ElAmmonia,
			MinTemp: celsius(450), Catalyst: world.ElIron,
			HeatDelta: heatWarm, Chance: rxSlow,
			Equation: "N₂ + 3H₂ ⇌ 2NH₃",
		},
		{
			A: world.ElUranium, B: world.ElSteam,
			ProductA: world.ElMagOxide, ProductB: world.ElHydrogen,
			MinTemp: celsius(300), HeatDelta: heatHot, Chance: rxSlow,
			Equation: "U + 2H₂O → UO₂ + 2H₂",
		},

		// ── Sulfides and tarnishing ─────────────────────────────────────────
		{
			A: world.ElIron, B: world.ElSulfur,
			ProductA: world.ElMagnetite, ProductB: world.MatEmpty,
			MinTemp: celsius(600), HeatDelta: heatWarm, Chance: rxModerate,
			Equation: "Fe + S → FeS",
		},
		{
			A: world.ElCopper, B: world.ElSulfur,
			ProductA: world.ElMagnetite, ProductB: world.MatEmpty,
			MinTemp: celsius(400), HeatDelta: heatMild, Chance: rxSlow,
			Equation: "Cu + S → CuS",
		},
		{
			A: world.ElSilver, B: world.ElSulfur,
			ProductA: world.ElMagnetite, ProductB: world.MatEmpty,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxCreeping,
			Equation: "2Ag + S → Ag₂S",
		},

		// ── Amalgams ────────────────────────────────────────────────────────
		//
		// Mercury and gallium both attack aluminium by breaking its protective
		// oxide layer, which is why an aluminium structure crumbles on contact
		// with either. Physical rather than strictly chemical, but too good a
		// demonstration to leave out.
		{
			A: world.ElMercury, B: world.ElAluminium,
			ProductA: world.ElMercury, ProductB: world.ElMagOxide,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxFast,
			Equation: "Hg breaks Al's oxide layer → Al₂O₃",
		},
		{
			A: world.ElGallium, B: world.ElAluminium,
			ProductA: world.ElGallium, ProductB: world.ElMagOxide,
			MinTemp: celsius(30), HeatDelta: 0, Chance: rxModerate,
			Equation: "Ga wets Al grain boundaries → structural failure",
		},
		{
			A: world.ElMercury, B: world.ElGold,
			ProductA: world.ElMercury, ProductB: world.ElMercury,
			MinTemp: world.TempNone, HeatDelta: 0, Chance: rxSlow,
			Equation: "Au + Hg → Au(Hg) amalgam",
		},

		// ── Phase and terrain interactions ──────────────────────────────────
		{
			A: world.MatSand, B: world.MatLava,
			ProductA: world.ElGlass, ProductB: world.MatLava,
			MinTemp: celsius(1713), HeatDelta: coolEndoMild, Chance: rxModerate,
			Equation: "SiO₂(s) → SiO₂(l) → glass",
		},
		{
			A: world.ElSteam, B: world.ElOxygen,
			ProductA: world.ElSteam, ProductB: world.ElOxygen,
			MinTemp: world.TempNone, HeatDelta: 0, Chance: rxCreeping,
			Equation: "H₂O(g) mixing — no net reaction",
		},
		{
			A: world.ElMoltenMet, B: world.MatWater,
			ProductA: world.ElIron, ProductB: world.ElSteam,
			MinTemp: world.TempNone, HeatDelta: heatHot, Chance: rxViolent,
			Equation: "molten metal quenched → solid + steam",
		},
		{
			A: world.ElQuicklime, B: world.MatWater,
			ProductA: world.ElLye, ProductB: world.ElSteam,
			MinTemp: world.TempNone, HeatDelta: heatHot, Chance: rxFast,
			Equation: "CaO + H₂O → Ca(OH)₂",
		},
		{
			A: world.ElPhosphorus, B: world.MatWater,
			ProductA: world.ElAcid, ProductB: world.MatWater,
			MinTemp: celsius(100), HeatDelta: heatMild, Chance: rxSlow,
			Equation: "P₄O₁₀ + 6H₂O → 4H₃PO₄",
		},
		{
			A: world.ElHydrogen, B: world.MatFire,
			ProductA: world.ElSteam, ProductB: world.MatFire,
			MinTemp: world.TempNone, HeatDelta: heatInferno, Chance: rxInstant,
			Equation: "2H₂ + O₂ → 2H₂O (flame front)",
		},
		{
			A: world.ElMethane, B: world.MatFire,
			ProductA: world.ElCO2, ProductB: world.MatFire,
			MinTemp: world.TempNone, HeatDelta: heatFierce, Chance: rxInstant,
			Equation: "CH₄ + 2O₂ → CO₂ + 2H₂O (flame front)",
		},
		{
			// Nitrogen smothers fire — the useful counterpart to the magnesium
			// case, and what makes CO2's failure against magnesium land.
			A: world.ElNitrogen, B: world.MatFire,
			ProductA: world.ElNitrogen, ProductB: world.MatSmoke,
			MinTemp: world.TempNone, HeatDelta: coolEndoMild, Chance: rxFast,
			Equation: "N₂ displaces O₂ — combustion starved",
		},
		{
			A: world.ElCO2, B: world.MatFire,
			ProductA: world.ElCO2, ProductB: world.MatSmoke,
			MinTemp: world.TempNone, HeatDelta: coolEndoMild, Chance: rxFast,
			Equation: "CO₂ displaces O₂ — combustion starved",
		},
		{
			A: world.ElChlorine, B: world.MatPlant,
			ProductA: world.ElChlorine, ProductB: world.MatAsh,
			MinTemp: world.TempNone, HeatDelta: 0, Chance: rxFast,
			Equation: "Cl₂ bleaches and kills organic matter",
		},
		{
			A: world.ElAcid, B: world.MatRock,
			ProductA: world.ElAcid, ProductB: world.MatSand,
			MinTemp: world.TempNone, HeatDelta: heatMild, Chance: rxSlow,
			Equation: "CaCO₃ + 2HCl → CaCl₂ + H₂O + CO₂",
		},
	}
}
