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
	default:
		return "unknown"
	}
}

// IsSolid returns true for materials that block downward movement.
func IsSolid(m uint8) bool {
	switch m {
	case MatRock, MatSoil, MatSand, MatPlant:
		return true
	}
	return false
}

// IsFlammable returns true for materials that can catch fire.
func IsFlammable(m uint8) bool {
	return m == MatPlant
}

// IsTransient returns true for materials with a finite lifetime.
func IsTransient(m uint8) bool {
	return m == MatFire || m == MatVapor || m == MatSmoke
}
