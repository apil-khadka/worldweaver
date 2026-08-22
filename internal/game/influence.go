package game

// InfluenceCost defines the influence drained per second for each power.
// Costs are expressed per-tick (at 60 TPS) in the simulation engine.
//
// These values are intentionally tunable — they are game-balance decisions,
// not simulation invariants. The goal is to create interesting trade-offs:
// cheap powers encourage casual use; expensive powers require commitment.
var InfluenceCost = map[uint8]float32{
	PowerRain:   2.0 / 60, // 2 influence/sec → 0.033/tick
	PowerHeat:   3.0 / 60, // 3 influence/sec → 0.050/tick
	PowerWind:   1.0 / 60, // 1 influence/sec → 0.017/tick
	PowerGrowth: 4.0 / 60, // 4 influence/sec → 0.067/tick
}

// Defined power IDs must match simulation.PowerType.
const (
	PowerRain   uint8 = 0
	PowerHeat   uint8 = 1
	PowerWind   uint8 = 2
	PowerGrowth uint8 = 3
)

// PowerName returns a human-readable power name for the given ID.
func PowerName(p uint8) string {
	switch p {
	case PowerRain:
		return "rain"
	case PowerHeat:
		return "heat"
	case PowerWind:
		return "wind"
	case PowerGrowth:
		return "growth"
	default:
		return "unknown"
	}
}

// MaxRadius is the server-enforced maximum influence radius for any power.
// Clients that send a larger value are clamped to this.
const MaxRadius = 64

// MaxIntensity is the server-enforced maximum intensity value.
const MaxIntensity float32 = 1.0
