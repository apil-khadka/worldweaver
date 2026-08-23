// Element data — the periodic table subset the sandbox simulates.
//
// Physical values are real, sourced from the CRC Handbook of Chemistry and
// Physics and the NIST Chemistry WebBook. Densities are kg/m³ at room temperature
// and melting and boiling points are whole degrees Celsius.
//
// Using real numbers is not pedantry: it is cheaper than tuning. Tungsten survives
// a lava flow because its melting point is genuinely 3422 °C, gallium melts in a
// warm room because it genuinely melts at 30 °C, and bromine is the one liquid
// non-metal because it genuinely is. None of that needed a designer.
//
// IDs 23 onward are new; 0..22 are the legacy materials, which stay on their
// hardcoded predicates for now (see element.go for the fallback path). ID 255 is
// reserved as the NoTransition sentinel and must never be used.
package world

// New element IDs. Grouped by kind rather than by atomic number so related
// elements stay adjacent, which makes the table easier to read and edit.
const (
	// ── Gases ────────────────────────────────────────────────────────────────
	ElHydrogen uint8 = 23
	ElHelium   uint8 = 24
	ElNitrogen uint8 = 25
	ElOxygen   uint8 = 26
	ElChlorine uint8 = 27
	ElMethane  uint8 = 28
	ElSteam    uint8 = 29
	ElCO2      uint8 = 30
	ElAmmonia  uint8 = 31

	// ── Alkali and alkaline earth metals ─────────────────────────────────────
	ElSodium    uint8 = 32
	ElPotassium uint8 = 33
	ElMagnesium uint8 = 34
	ElCalcium   uint8 = 35

	// ── Transition and post-transition metals ────────────────────────────────
	ElAluminium uint8 = 36
	ElTitanium  uint8 = 37
	ElIron      uint8 = 38
	ElCopper    uint8 = 39
	ElZinc      uint8 = 40
	ElGallium   uint8 = 41
	ElSilver    uint8 = 42
	ElTin       uint8 = 43
	ElTungsten  uint8 = 44
	ElGold      uint8 = 45
	ElMercury   uint8 = 46
	ElLead      uint8 = 47
	ElUranium   uint8 = 48

	// ── Nonmetals and metalloids ─────────────────────────────────────────────
	ElCarbon     uint8 = 49
	ElSilicon    uint8 = 50
	ElPhosphorus uint8 = 51
	ElSulfur     uint8 = 52
	ElBromine    uint8 = 53

	// ── Compounds and mixtures ───────────────────────────────────────────────
	ElSalt      uint8 = 54
	ElRust      uint8 = 55
	ElMagnetite uint8 = 56
	ElAcid      uint8 = 57
	ElLye       uint8 = 58
	ElEthanol   uint8 = 59
	ElGunpowder uint8 = 60
	ElThermite  uint8 = 61
	ElGlass     uint8 = 62
	ElMoltenMet uint8 = 63 // generic molten metal, the melt product for metals
	ElQuicklime uint8 = 64
	ElMagOxide  uint8 = 65
	ElSoap      uint8 = 66
)

// Flammability, conductivity and reactivity are 0..255 scales. Named constants
// keep the table readable and make "why is this 180" answerable.
const (
	flamNone   uint8 = 0
	flamLow    uint8 = 40
	flamMedium uint8 = 110
	flamHigh   uint8 = 190
	flamExtreme uint8 = 250

	condLow    uint8 = 30
	condMedium uint8 = 90
	condHigh   uint8 = 170
	condMetal  uint8 = 220

	reactNone    uint8 = 0
	reactTrace   uint8 = 20
	reactLow     uint8 = 60
	reactMedium  uint8 = 120
	reactHigh    uint8 = 190
	reactViolent uint8 = 250
)

func init() {
	// ── Gases ────────────────────────────────────────────────────────────────

	register(Element{
		ID: ElHydrogen, Name: "Hydrogen", Symbol: "H", Atomic: 1,
		Category: CatNonmetal, Phase: PhaseGas,
		Density: 1, MeltingPoint: -259, BoilingPoint: -253, IgnitionTemp: 500,
		Flammability: flamExtreme, Conductivity: condMedium, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElSteam,
		Colour:  [4]uint8{214, 234, 248, 150},
		Flavour: "Lightest element. Detonates with oxygen.",
	})
	register(Element{
		ID: ElHelium, Name: "Helium", Symbol: "He", Atomic: 2,
		Category: CatNobleGas, Phase: PhaseGas,
		Density: 1, MeltingPoint: -272, BoilingPoint: -269, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactNone,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{247, 220, 111, 110},
		Flavour: "Utterly inert. Rises and escapes.",
	})
	register(Element{
		ID: ElNitrogen, Name: "Nitrogen", Symbol: "N", Atomic: 7,
		Category: CatNonmetal, Phase: PhaseGas,
		Density: 1, MeltingPoint: -210, BoilingPoint: -196, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactTrace,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{174, 214, 241, 120},
		Flavour: "Smothers fire. Makes ammonia under pressure.",
	})
	register(Element{
		ID: ElOxygen, Name: "Oxygen", Symbol: "O", Atomic: 8,
		Category: CatNonmetal, Phase: PhaseGas,
		Density: 1, MeltingPoint: -219, BoilingPoint: -183, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactViolent,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{163, 228, 245, 130},
		Flavour: "Feeds every fire. Rusts every metal.",
	})
	register(Element{
		ID: ElChlorine, Name: "Chlorine", Symbol: "Cl", Atomic: 17,
		Category: CatHalogen, Phase: PhaseGas,
		Density: 3, MeltingPoint: -101, BoilingPoint: -34, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactViolent,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{190, 228, 130, 170},
		Flavour: "Toxic green gas. Burns metals outright.",
	})
	register(Element{
		ID: ElMethane, Name: "Methane", Symbol: "CH4", Atomic: 0,
		Category: CatOrganic, Phase: PhaseGas,
		Density: 1, MeltingPoint: -182, BoilingPoint: -161, IgnitionTemp: 580,
		Flammability: flamExtreme, Conductivity: condLow, Reactivity: reactMedium,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElCO2,
		Colour:  [4]uint8{200, 220, 190, 120},
		Flavour: "Natural gas. Explosive in air.",
	})
	register(Element{
		ID: ElSteam, Name: "Steam", Symbol: "H2O", Atomic: 0,
		Category: CatCompound, Phase: PhaseGas,
		Density: 1, MeltingPoint: -32000, BoilingPoint: 32000, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{230, 240, 245, 130},
		Flavour: "Hot water vapour. Condenses on cold surfaces.",
	})
	register(Element{
		ID: ElCO2, Name: "Carbon Dioxide", Symbol: "CO2", Atomic: 0,
		Category: CatCompound, Phase: PhaseGas,
		Density: 2, MeltingPoint: -78, BoilingPoint: -57, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{200, 200, 205, 100},
		Flavour: "Smothers most fires — but feeds burning magnesium.",
	})
	register(Element{
		ID: ElAmmonia, Name: "Ammonia", Symbol: "NH3", Atomic: 0,
		Category: CatCompound, Phase: PhaseGas,
		Density: 1, MeltingPoint: -78, BoilingPoint: -33, IgnitionTemp: 651,
		Flammability: flamMedium, Conductivity: condLow, Reactivity: reactMedium,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElNitrogen,
		Colour:  [4]uint8{205, 235, 215, 130},
		Flavour: "Pungent. The Haber process product.",
	})

	// ── Alkali and alkaline earth metals ─────────────────────────────────────
	//
	// The alkali metals are the set piece of the whole registry: they react with
	// water violently and without needing any heat, which is the most famous
	// demonstration in chemistry and reads perfectly in a 2D grid.

	register(Element{
		ID: ElSodium, Name: "Sodium", Symbol: "Na", Atomic: 11,
		Category: CatAlkaliMetal, Phase: PhasePowder,
		Density: 970, MeltingPoint: 98, BoilingPoint: 883, IgnitionTemp: 115,
		Flammability: flamHigh, Conductivity: condMetal, Reactivity: reactViolent,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{220, 220, 210, 255},
		Flavour: "Explodes on contact with water. Do not get it wet.",
	})
	register(Element{
		ID: ElPotassium, Name: "Potassium", Symbol: "K", Atomic: 19,
		Category: CatAlkaliMetal, Phase: PhasePowder,
		Density: 862, MeltingPoint: 63, BoilingPoint: 759, IgnitionTemp: 65,
		Flammability: flamExtreme, Conductivity: condMetal, Reactivity: reactViolent,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{205, 200, 215, 255},
		Flavour: "More violent with water than sodium. Ignites in air.",
	})
	register(Element{
		ID: ElMagnesium, Name: "Magnesium", Symbol: "Mg", Atomic: 12,
		Category: CatAlkalineEarth, Phase: PhasePowder,
		Density: 1738, MeltingPoint: 650, BoilingPoint: 1091, IgnitionTemp: 473,
		Flammability: flamHigh, Conductivity: condMetal, Reactivity: reactHigh,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: ElMagOxide,
		Colour:  [4]uint8{235, 235, 228, 255},
		Flavour: "Burns blinding white — and keeps burning in CO2 or underwater.",
	})
	register(Element{
		ID: ElCalcium, Name: "Calcium", Symbol: "Ca", Atomic: 20,
		Category: CatAlkalineEarth, Phase: PhasePowder,
		Density: 1550, MeltingPoint: 842, BoilingPoint: 1484, IgnitionTemp: 842,
		Flammability: flamMedium, Conductivity: condMetal, Reactivity: reactHigh,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: ElQuicklime,
		Colour:  [4]uint8{228, 224, 208, 255},
		Flavour: "Fizzes in water. Burns to quicklime.",
	})

	// ── Transition and post-transition metals ────────────────────────────────

	register(Element{
		ID: ElAluminium, Name: "Aluminium", Symbol: "Al", Atomic: 13,
		Category: CatPostTransition, Phase: PhaseRigid,
		Density: 2700, MeltingPoint: 660, BoilingPoint: 2519, IgnitionTemp: 1500,
		Flammability: flamLow, Conductivity: condMetal, Reactivity: reactMedium,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: ElMagOxide,
		Colour:  [4]uint8{205, 208, 212, 255},
		Flavour: "Light and strong. The fuel half of thermite.",
	})
	register(Element{
		ID: ElTitanium, Name: "Titanium", Symbol: "Ti", Atomic: 22,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 4507, MeltingPoint: 1668, BoilingPoint: 3287, IgnitionTemp: 1200,
		Flammability: flamLow, Conductivity: condHigh, Reactivity: reactLow,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: ElMagOxide,
		Colour:  [4]uint8{170, 175, 180, 255},
		Flavour: "Survives what melts steel. Sparks brilliant white.",
	})
	register(Element{
		ID: ElIron, Name: "Iron", Symbol: "Fe", Atomic: 26,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 7874, MeltingPoint: 1538, BoilingPoint: 2862, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMetal, Reactivity: reactMedium,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{130, 128, 126, 255},
		Flavour: "Rusts in damp air. Melts in a thermite pour.",
	})
	register(Element{
		ID: ElCopper, Name: "Copper", Symbol: "Cu", Atomic: 29,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 8960, MeltingPoint: 1085, BoilingPoint: 2562, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMetal, Reactivity: reactLow,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{184, 115, 51, 255},
		Flavour: "Superb conductor. Immune to plain acid.",
	})
	register(Element{
		ID: ElZinc, Name: "Zinc", Symbol: "Zn", Atomic: 30,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 7134, MeltingPoint: 420, BoilingPoint: 907, IgnitionTemp: 460,
		Flammability: flamLow, Conductivity: condHigh, Reactivity: reactMedium,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: ElMagOxide,
		Colour:  [4]uint8{164, 172, 178, 255},
		Flavour: "Dissolves in acid, fizzing hydrogen.",
	})
	register(Element{
		ID: ElGallium, Name: "Gallium", Symbol: "Ga", Atomic: 31,
		Category: CatPostTransition, Phase: PhaseRigid,
		Density: 5910, MeltingPoint: 30, BoilingPoint: 2204, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condHigh, Reactivity: reactLow,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{190, 194, 200, 255},
		Flavour: "Melts in your hand. Crumbles aluminium on contact.",
	})
	register(Element{
		ID: ElSilver, Name: "Silver", Symbol: "Ag", Atomic: 47,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 10490, MeltingPoint: 962, BoilingPoint: 2162, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMetal, Reactivity: reactTrace,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{215, 218, 222, 255},
		Flavour: "The best conductor there is. Tarnishes in sulfur.",
	})
	register(Element{
		ID: ElTin, Name: "Tin", Symbol: "Sn", Atomic: 50,
		Category: CatPostTransition, Phase: PhaseRigid,
		Density: 7265, MeltingPoint: 232, BoilingPoint: 2602, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condHigh, Reactivity: reactLow,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{200, 200, 195, 255},
		Flavour: "Melts on a camp stove. Solder metal.",
	})
	register(Element{
		ID: ElTungsten, Name: "Tungsten", Symbol: "W", Atomic: 74,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 19250, MeltingPoint: 3422, BoilingPoint: 5555, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condHigh, Reactivity: reactTrace,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{110, 112, 115, 255},
		Flavour: "Highest melting point of any element. Lava does nothing to it.",
	})
	register(Element{
		ID: ElGold, Name: "Gold", Symbol: "Au", Atomic: 79,
		Category: CatTransitionMetal, Phase: PhaseRigid,
		Density: 19300, MeltingPoint: 1064, BoilingPoint: 2856, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMetal, Reactivity: reactNone,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{212, 175, 55, 255},
		Flavour: "Noble. Shrugs off acid unless chlorine helps.",
	})
	register(Element{
		ID: ElMercury, Name: "Mercury", Symbol: "Hg", Atomic: 80,
		Category: CatTransitionMetal, Phase: PhaseLiquid,
		Density: 13534, MeltingPoint: -39, BoilingPoint: 357, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condHigh, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{190, 190, 195, 255},
		Flavour: "Liquid metal. Dissolves gold, destroys aluminium.",
	})
	register(Element{
		ID: ElLead, Name: "Lead", Symbol: "Pb", Atomic: 82,
		Category: CatPostTransition, Phase: PhaseRigid,
		Density: 11340, MeltingPoint: 327, BoilingPoint: 1749, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactTrace,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{110, 110, 120, 255},
		Flavour: "Dense and soft. Blocks radiation.",
	})
	register(Element{
		ID: ElUranium, Name: "Uranium", Symbol: "U", Atomic: 92,
		Category: CatActinide, Phase: PhaseRigid,
		Density: 19050, MeltingPoint: 1132, BoilingPoint: 4131, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactMedium,
		MeltsInto: ElMoltenMet, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{120, 160, 110, 255},
		Flavour: "Radioactive and absurdly dense. Splits steam into hydrogen.",
	})

	// ── Nonmetals and metalloids ─────────────────────────────────────────────

	register(Element{
		ID: ElCarbon, Name: "Carbon", Symbol: "C", Atomic: 6,
		Category: CatNonmetal, Phase: PhasePowder,
		Density: 2260, MeltingPoint: 3550, BoilingPoint: 4827, IgnitionTemp: 700,
		Flammability: flamHigh, Conductivity: condMedium, Reactivity: reactMedium,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElCO2,
		Colour:  [4]uint8{40, 40, 44, 255},
		Flavour: "Burns to carbon dioxide. Smelts iron from its ore.",
	})
	register(Element{
		ID: ElSilicon, Name: "Silicon", Symbol: "Si", Atomic: 14,
		Category: CatMetalloid, Phase: PhasePowder,
		Density: 2330, MeltingPoint: 1414, BoilingPoint: 3265, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactLow,
		MeltsInto: ElGlass, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{100, 105, 115, 255},
		Flavour: "Semiconductor. The stuff of sand and glass.",
	})
	register(Element{
		ID: ElPhosphorus, Name: "Phosphorus", Symbol: "P", Atomic: 15,
		Category: CatNonmetal, Phase: PhasePowder,
		Density: 1820, MeltingPoint: 44, BoilingPoint: 280, IgnitionTemp: 34,
		Flammability: flamExtreme, Conductivity: condLow, Reactivity: reactViolent,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{240, 230, 190, 255},
		Flavour: "Bursts into flame in open air, unprompted.",
	})
	register(Element{
		ID: ElSulfur, Name: "Sulfur", Symbol: "S", Atomic: 16,
		Category: CatNonmetal, Phase: PhasePowder,
		Density: 2070, MeltingPoint: 115, BoilingPoint: 445, IgnitionTemp: 250,
		Flammability: flamHigh, Conductivity: condLow, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{230, 215, 80, 255},
		Flavour: "Burns with a blue flame and a choking smell.",
	})
	register(Element{
		ID: ElBromine, Name: "Bromine", Symbol: "Br", Atomic: 35,
		Category: CatHalogen, Phase: PhaseLiquid,
		Density: 3120, MeltingPoint: -7, BoilingPoint: 59, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactViolent,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{150, 50, 30, 230},
		Flavour: "The only non-metal that is liquid at room temperature.",
	})

	// ── Compounds and mixtures ───────────────────────────────────────────────

	register(Element{
		ID: ElSalt, Name: "Salt", Symbol: "NaCl", Atomic: 0,
		Category: CatCompound, Phase: PhasePowder,
		Density: 2165, MeltingPoint: 801, BoilingPoint: 1413, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{245, 245, 240, 255},
		Flavour: "Dissolves away in water. What sodium and chlorine settle into.",
	})
	register(Element{
		ID: ElRust, Name: "Rust", Symbol: "Fe2O3", Atomic: 0,
		Category: CatCompound, Phase: PhasePowder,
		Density: 5250, MeltingPoint: 1565, BoilingPoint: 3000, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{150, 75, 40, 255},
		Flavour: "Iron oxide. The oxidiser half of thermite.",
	})
	register(Element{
		ID: ElMagnetite, Name: "Magnetite", Symbol: "Fe3O4", Atomic: 0,
		Category: CatCompound, Phase: PhasePowder,
		Density: 5170, MeltingPoint: 1597, BoilingPoint: 3000, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactTrace,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{55, 55, 62, 255},
		Flavour: "Naturally magnetic iron ore.",
	})
	register(Element{
		ID: ElAcid, Name: "Acid", Symbol: "HCl", Atomic: 0,
		Category: CatCompound, Phase: PhaseLiquid,
		Density: 1180, MeltingPoint: -27, BoilingPoint: 48, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactViolent,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{200, 240, 120, 220},
		Flavour: "Eats most metals. Gold and copper resist it.",
	})
	register(Element{
		ID: ElLye, Name: "Lye", Symbol: "NaOH", Atomic: 0,
		Category: CatCompound, Phase: PhaseLiquid,
		Density: 1515, MeltingPoint: 318, BoilingPoint: 1388, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMedium, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{225, 235, 215, 220},
		Flavour: "Caustic base. Neutralises acid, turns oil into soap.",
	})
	register(Element{
		ID: ElEthanol, Name: "Ethanol", Symbol: "C2H5OH", Atomic: 0,
		Category: CatOrganic, Phase: PhaseLiquid,
		Density: 789, MeltingPoint: -114, BoilingPoint: 78, IgnitionTemp: 363,
		Flammability: flamExtreme, Conductivity: condLow, Reactivity: reactMedium,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElCO2,
		Colour:  [4]uint8{225, 225, 235, 200},
		Flavour: "Burns with an almost invisible flame.",
	})
	register(Element{
		ID: ElGunpowder, Name: "Gunpowder", Symbol: "", Atomic: 0,
		Category: CatMixture, Phase: PhasePowder,
		Density: 1700, MeltingPoint: 3000, BoilingPoint: 4000, IgnitionTemp: 300,
		Flammability: flamExtreme, Conductivity: condLow, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{60, 58, 55, 255},
		Flavour: "Saltpetre, charcoal and sulfur. Deflagrates, does not detonate.",
	})
	register(Element{
		ID: ElThermite, Name: "Thermite", Symbol: "", Atomic: 0,
		Category: CatMixture, Phase: PhasePowder,
		Density: 3900, MeltingPoint: 3000, BoilingPoint: 4000, IgnitionTemp: 1500,
		Flammability: flamHigh, Conductivity: condLow, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: ElMoltenMet,
		Colour:  [4]uint8{95, 70, 55, 255},
		Flavour: "Aluminium and rust. Burns at 2500 °C, straight through steel.",
	})
	register(Element{
		ID: ElGlass, Name: "Glass", Symbol: "SiO2", Atomic: 0,
		Category: CatCompound, Phase: PhaseRigid,
		Density: 2500, MeltingPoint: 1713, BoilingPoint: 2230, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactNone,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{200, 225, 230, 140},
		Flavour: "What sand becomes when it gets hot enough.",
	})
	register(Element{
		ID: ElMoltenMet, Name: "Molten Metal", Symbol: "", Atomic: 0,
		Category: CatFluid, Phase: PhaseLiquid,
		Density: 7000, MeltingPoint: -32000, BoilingPoint: 32000, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condMetal, Reactivity: reactMedium,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{255, 170, 60, 255},
		Flavour: "Metal hot enough to flow. Cools back into iron.",
	})
	register(Element{
		ID: ElQuicklime, Name: "Quicklime", Symbol: "CaO", Atomic: 0,
		Category: CatCompound, Phase: PhasePowder,
		Density: 3340, MeltingPoint: 2572, BoilingPoint: 2850, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactHigh,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{240, 238, 230, 255},
		Flavour: "Glows brilliantly when heated — the original limelight.",
	})
	register(Element{
		ID: ElMagOxide, Name: "Metal Oxide", Symbol: "MgO", Atomic: 0,
		Category: CatCompound, Phase: PhasePowder,
		Density: 3580, MeltingPoint: 2852, BoilingPoint: 3600, IgnitionTemp: TempNone,
		Flammability: flamNone, Conductivity: condLow, Reactivity: reactTrace,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: NoTransition,
		Colour:  [4]uint8{248, 248, 245, 255},
		Flavour: "The white ash left after a metal burns.",
	})
	register(Element{
		ID: ElSoap, Name: "Soap", Symbol: "", Atomic: 0,
		Category: CatOrganic, Phase: PhasePowder,
		Density: 1100, MeltingPoint: 300, BoilingPoint: 1000, IgnitionTemp: 500,
		Flammability: flamMedium, Conductivity: condLow, Reactivity: reactLow,
		MeltsInto: NoTransition, BoilsInto: NoTransition, BurnsInto: MatFire,
		Colour:  [4]uint8{238, 230, 215, 255},
		Flavour: "Lye plus oil. Saponification, the useful reaction.",
	})
}
