package game

import "errors"

// PowerRequest is the input the server receives from a client.
// It is validated before being converted to a simulation.PlayerAction.
type PowerRequest struct {
	PlayerID  uint32
	Power     uint8
	X, Y      int
	Radius    int
	Intensity float32
}

// Validate checks that the request is within allowed bounds.
// It also verifies and deducts the required influence from the player.
// Returns a non-nil error if the request should be rejected.
func (r *PowerRequest) Validate(p *Player, worldW, worldH int) error {
	// Power must be known
	if _, ok := InfluenceCost[r.Power]; !ok {
		return errors.New("unknown power")
	}

	// Coordinates must be inside world
	if r.X < 0 || r.X >= worldW || r.Y < 0 || r.Y >= worldH {
		return errors.New("coordinates out of bounds")
	}

	// Clamp radius
	if r.Radius < 1 {
		r.Radius = 1
	}
	if r.Radius > MaxRadius {
		r.Radius = MaxRadius
	}

	// Clamp intensity
	if r.Intensity < 0 {
		r.Intensity = 0
	}
	if r.Intensity > MaxIntensity {
		r.Intensity = MaxIntensity
	}

	// Consume influence
	cost := InfluenceCost[r.Power]
	if !p.ConsumeInfluence(cost) {
		return errors.New("insufficient influence")
	}

	return nil
}
