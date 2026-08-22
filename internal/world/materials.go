package world

// Material IDs — stored as uint8 in the material array.
const (
	MatEmpty uint8 = iota // 0
	MatRock               // 1
	MatSoil               // 2
	MatSand               // 3
	MatWater              // 4
	MatPlant              // 5
	MatFire               // 6
	MatVapor              // 7
	MatSmoke              // 8
	MatLava               // 9
	MatIce                // 10
	MatAsh                // 11
	MatOil                // 12
	MatEmber              // 13
	MatHerbivore          // 14
	MatPredator           // 15
	MatCloud              // 16
)

// MaterialName returns a human-readable name for debug/serialization.
func MaterialName(m uint8) string {
	switch m {
	case MatEmpty:
		return "empty"
	case MatRock:
		return "rock"
	case MatSoil:
		return "soil"
	case MatSand:
		return "sand"
	case MatWater:
		return "water"
	case MatPlant:
		return "plant"
	case MatFire:
		return "fire"
	case MatVapor:
		return "vapor"
	case MatSmoke:
		return "smoke"
	case MatLava:
		return "lava"
	case MatIce:
		return "ice"
	case MatAsh:
		return "ash"
	case MatOil:
		return "oil"
	case MatEmber:
		return "ember"
	case MatHerbivore:
		return "herbivore"
	case MatPredator:
		return "predator"
	case MatCloud:
		return "cloud"
	default:
		return "unknown"
	}
}

// IsSolid returns true for materials that block downward movement.
func IsSolid(m uint8) bool {
	switch m {
	case MatRock, MatSoil, MatSand, MatPlant, MatIce, MatHerbivore, MatPredator:
		return true
	}
	return false
}

// IsFlammable returns true for materials that can catch fire.
func IsFlammable(m uint8) bool {
	return m == MatPlant || m == MatOil
}

// IsLiquid returns true for materials that flow laterally.
func IsLiquid(m uint8) bool {
	return m == MatWater || m == MatOil || m == MatLava
}

// IsGas returns true for materials that rise upward.
func IsGas(m uint8) bool {
	return m == MatVapor || m == MatSmoke || m == MatEmpty || m == MatCloud
}

// IsTransient returns true for materials with a finite lifetime.
func IsTransient(m uint8) bool {
	return m == MatFire || m == MatVapor || m == MatSmoke || m == MatEmber
}

// IsCreature returns true for living creature materials.
func IsCreature(m uint8) bool {
	return m == MatHerbivore || m == MatPredator
}

// Density returns a relative density value for liquid displacement.
// Higher values sink below lower values.
func Density(m uint8) uint8 {
	switch m {
	case MatEmpty:
		return 0
	case MatVapor, MatSmoke, MatCloud:
		return 5
	case MatOil:
		return 60
	case MatWater:
		return 80
	case MatIce:
		return 90
	case MatSand:
		return 120
	case MatSoil:
		return 150
	case MatRock, MatLava:
		return 200
	default:
		return 50
	}
}
