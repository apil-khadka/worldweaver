package game

import "errors"

// PowerRequest is the input the server receives from a client.
// It is validated before being converted to a simulation.PlayerAction.
type PowerRequest struct {
	PlayerID uint32

	// Tool selects between applying an elemental force and editing the world
	// directly. An empty value is treated as ToolForce for older clients.
	Tool string

	// Power is only meaningful for ToolForce.
	Power uint8

	// Material is only meaningful for ToolPlace.
	Material uint8

	X, Y      int
	Radius    int
	Intensity float32
}

// Validate checks that the request is within allowed bounds.
// It also verifies and deducts the required influence from the player.
// Returns a non-nil error if the request should be rejected.
func (r *PowerRequest) Validate(p *Player, worldW, worldH int) error {
	if r.Tool == "" {
		r.Tool = ToolForce
	}
	if !IsKnownTool(r.Tool) {
		return errors.New("unknown tool")
	}

	// Coordinates must be inside world
	if r.X < 0 || r.X >= worldW || r.Y < 0 || r.Y >= worldH {
		return errors.New("coordinates out of bounds")
	}

	// Clamp intensity
	if r.Intensity < 0 {
		r.Intensity = 0
	}
	if r.Intensity > MaxIntensity {
		r.Intensity = MaxIntensity
	}

	if r.Tool == ToolForce {
		return r.validateForce(p)
	}
	return r.validateEdit(p)
}

// validateForce handles the elemental powers, whose reach and availability
// depend on player level.
func (r *PowerRequest) validateForce(p *Player) error {
	if _, ok := InfluenceCost[r.Power]; !ok {
		return errors.New("unknown power")
	}

	if !p.CanUsePower(r.Power) {
		return errors.New("power locked: level too low")
	}

	maxR := p.PowerRadius()
	if r.Radius < 1 {
		r.Radius = 1
	}
	if r.Radius > maxR {
		r.Radius = maxR
	}
	if r.Radius > MaxRadius {
		r.Radius = MaxRadius
	}

	if !p.ConsumeInfluence(InfluenceCost[r.Power]) {
		return errors.New("insufficient influence")
	}
	return nil
}

// validateEdit handles the direct world-editing tools, which are charged by the
// area they cover so a large brush drains influence quickly.
func (r *PowerRequest) validateEdit(p *Player) error {
	if r.Tool == ToolPlace && !IsPlaceable(r.Material) {
		return errors.New("material cannot be placed")
	}

	if r.Radius < 1 {
		r.Radius = 1
	}
	if r.Radius > MaxToolRadius {
		r.Radius = MaxToolRadius
	}

	cost := ToolCostPerCell[r.Tool] * float32(CellsInRadius(r.Radius))

	// Destructive materials cost more, so unleashing a void or a plasma burst is
	// a deliberate act rather than something to hold the mouse down on.
	if r.Tool == ToolPlace && IsHazardMaterial(r.Material) {
		cost *= HazardCostMultiplier
	}

	if !p.ConsumeInfluence(cost) {
		return errors.New("insufficient influence")
	}
	return nil
}

// CellsInRadius returns the number of cells inside a circular brush of the given
// radius. Used for area-scaled influence costs and for score accounting.
func CellsInRadius(radius int) int {
	if radius < 1 {
		return 1
	}
	r2 := radius * radius
	count := 0
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= r2 {
				count++
			}
		}
	}
	return count
}
