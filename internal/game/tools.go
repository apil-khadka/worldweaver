package game

// God-mode tools.
//
// Tools reuse the power message and its validation, rate limiting and influence
// accounting rather than introducing a parallel input channel. Everything still
// resolves to a PlayerAction applied inside the simulation tick, so the server
// remains the sole authority over world state.
const (
	// ToolForce applies an elemental power: rain, heat, wind, growth or life.
	// This is the default when a client sends no tool.
	ToolForce = "force"

	// ToolPlace paints a chosen material directly.
	ToolPlace = "place"

	// ToolErase clears cells back to empty.
	ToolErase = "erase"

	// ToolRaise adds terrain on top of existing ground.
	ToolRaise = "raise"

	// ToolLower removes the exposed surface of existing ground.
	ToolLower = "lower"
)

// ToolCostPerCell is the influence charged per cell covered by the brush.
//
// Direct manipulation is deliberately far more expensive per action than the
// elemental forces, so shaping the world at scale still means working with the
// simulation rather than painting over it.
var ToolCostPerCell = map[string]float32{
	ToolPlace: 0.0040,
	ToolErase: 0.0020,
	ToolRaise: 0.0060,
	ToolLower: 0.0060,
}

// MaxToolRadius caps the brush independently of the power radius, which scales
// with player level. Direct edits stay bounded regardless of progression.
const MaxToolRadius = 32

// HazardCostMultiplier is applied on top of the area cost when placing a
// destructive material.
const HazardCostMultiplier float32 = 4.0

// IsKnownTool reports whether the client supplied a tool the server implements.
func IsKnownTool(tool string) bool {
	switch tool {
	case ToolForce, ToolPlace, ToolErase, ToolRaise, ToolLower:
		return true
	default:
		return false
	}
}

// Material IDs that may be painted with ToolPlace.
//
// Creatures are excluded: they carry energy state in the temperature field and
// are spawned through the Life power, which initialises them correctly.
const (
	matEmpty     uint8 = 0
	matRock      uint8 = 1
	matSoil      uint8 = 2
	matSand      uint8 = 3
	matWater     uint8 = 4
	matPlant     uint8 = 5
	matFire      uint8 = 6
	matVapor     uint8 = 7
	matSmoke     uint8 = 8
	matLava      uint8 = 9
	matIce       uint8 = 10
	matAsh       uint8 = 11
	matOil       uint8 = 12
	matEmber     uint8 = 13
	matHerbivore uint8 = 14
	matPredator  uint8 = 15
	matCloud     uint8 = 16
	matVoid      uint8 = 17
	matRadiation uint8 = 18
	matPlasma    uint8 = 19
)

// PlaceableMaterials lists what ToolPlace accepts, in palette order.
var PlaceableMaterials = []uint8{
	matRock, matSoil, matSand, matWater, matPlant,
	matFire, matLava, matIce, matOil, matAsh,
	matVapor, matSmoke, matCloud,
	matPlasma, matRadiation, matVoid,
}

// HazardMaterials are placeable but destructive, so the client can present them
// separately and the server can charge more for them.
var HazardMaterials = []uint8{matPlasma, matRadiation, matVoid}

// IsHazardMaterial reports whether placing this material is destructive.
func IsHazardMaterial(m uint8) bool {
	for _, h := range HazardMaterials {
		if m == h {
			return true
		}
	}
	return false
}

// IsPlaceable reports whether a material may be painted directly.
func IsPlaceable(m uint8) bool {
	for _, allowed := range PlaceableMaterials {
		if m == allowed {
			return true
		}
	}
	return false
}
