package simulation

import (
	"math"

	"github.com/worldweaver/worldweaver/internal/world"
)

// PowerType identifies a player power.
type PowerType uint8

const (
	PowerRain   PowerType = iota
	PowerHeat
	PowerWind
	PowerGrowth
)

// PlayerAction represents a validated player input to be applied to the world.
type PlayerAction struct {
	PlayerID  uint32
	Power     PowerType
	X, Y      int
	Radius    int
	Intensity float32
}

// applyAction applies the given player action to the world.
func applyAction(w *world.World, a PlayerAction) {
	r2 := a.Radius * a.Radius
	for dy := -a.Radius; dy <= a.Radius; dy++ {
		for dx := -a.Radius; dx <= a.Radius; dx++ {
			if dx*dx+dy*dy > r2 {
				continue
			}
			x, y := a.X+dx, a.Y+dy
			i := w.Index(x, y)
			if i < 0 {
				continue
			}
			switch a.Power {
			case PowerRain:
				applyRain(w, x, y, i, a.Intensity)
			case PowerHeat:
				applyHeat(w, x, y, i, a.Intensity, dx, dy)
			case PowerGrowth:
				applyGrowth(w, x, y, i, a.Intensity)
			case PowerWind:
				applyWind(w, x, y, i, a.Intensity)
			}
		}
	}
}

// applyRain converts empty cells near the radius boundary to water droplets.
func applyRain(w *world.World, x, y int, i int, intensity float32) {
	if w.Material[i] == world.MatEmpty {
		dist := math.Sqrt(float64(x*x + y*y)) // approximate
		_ = dist
		if w.RNG().Float32() < intensity*0.1 {
			w.SetMaterial(x, y, world.MatWater)
		}
	}
}

// applyHeat raises the temperature of all cells in the radius.
func applyHeat(w *world.World, x, y, i int, intensity float32, dx, dy int) {
	add := int16(intensity * 300)
	if w.Temperature[i]+add < 32767 {
		w.Temperature[i] += add
	} else {
		w.Temperature[i] = 32767
	}
}

// applyGrowth seeds new plant cells on moist soil.
func applyGrowth(w *world.World, x, y, i int, intensity float32) {
	if w.Material[i] == world.MatSoil && w.Moisture[i] >= 20 {
		if w.RNG().Float32() < intensity*0.05 {
			// Plant on the cell above if empty
			if w.GetMaterial(x, y-1) == world.MatEmpty {
				w.SetMaterial(x, y-1, world.MatPlant)
			}
		}
	}
}

// applyWind nudges sand and smoke horizontally.
func applyWind(w *world.World, x, y, i int, intensity float32) {
	mat := w.Material[i]
	if mat == world.MatSand || mat == world.MatSmoke {
		dx := 1
		if intensity < 0 {
			dx = -1
		}
		if w.GetMaterial(x+dx, y) == world.MatEmpty {
			w.Swap(x, y, x+dx, y)
			markMoved(w, x+dx, y)
		}
	}
}
