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
	PowerLife // 4 — spawns herbivores
)

// Tool names, mirroring the values in internal/game. An empty Tool is treated as
// ToolForce so the elemental powers remain the default behaviour.
const (
	ToolForce = "force"
	ToolPlace = "place"
	ToolErase = "erase"
	ToolRaise = "raise"
	ToolLower = "lower"
)

// PlayerAction represents a validated player input to be applied to the world.
type PlayerAction struct {
	PlayerID uint32

	// Tool selects between an elemental force and a direct world edit.
	Tool string

	// Power is used when Tool is ToolForce.
	Power PowerType

	// Material is used when Tool is ToolPlace.
	Material uint8

	X, Y      int
	Radius    int
	Intensity float32
}

// applyAction applies the given player action to the world.
// It also marks affected chunks as changed to prevent them from sleeping.
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
			// Mark the chunk as changed so it stays awake.
			w.MarkDirty(x, y)

			switch a.Tool {
			case ToolPlace:
				applyPlace(w, x, y, i, a.Material)
			case ToolErase:
				applyErase(w, x, y, i)
			case ToolRaise:
				applyRaise(w, x, y, i)
			case ToolLower:
				applyLower(w, x, y, i)
			default:
				applyForce(w, x, y, i, dx, dy, a)
			}
		}
	}
}

// applyForce dispatches the elemental powers.
func applyForce(w *world.World, x, y, i, dx, dy int, a PlayerAction) {
	switch a.Power {
	case PowerRain:
		applyRain(w, x, y, i, a.Intensity)
	case PowerHeat:
		applyHeat(w, x, y, i, a.Intensity, dx, dy)
	case PowerGrowth:
		applyGrowth(w, x, y, i, a.Intensity)
	case PowerWind:
		applyWind(w, x, y, i, a.Intensity)
	case PowerLife:
		applyLife(w, x, y, i, a.Intensity)
	}
}

// applyPlace paints a material directly, initialising any state the material
// needs so it behaves correctly on the next tick.
func applyPlace(w *world.World, x, y, i int, mat uint8) {
	w.SetMaterial(x, y, mat)

	switch mat {
	case world.MatFire, world.MatEmber:
		// Transient materials burn for a bounded time; without a lifetime they
		// would never expire.
		w.Lifetime[i] = uint16(90 + w.RNG().Intn(60))
		w.Temperature[i] = 700
	case world.MatVapor, world.MatSmoke:
		w.Lifetime[i] = uint16(80 + w.RNG().Intn(80))
	case world.MatLava:
		w.Temperature[i] = 1200
	case world.MatIce:
		w.Temperature[i] = -100
	case world.MatVoid:
		// Lifetime bounds how much a void can consume before it collapses.
		w.Lifetime[i] = voidInitialLifetime
	case world.MatRadiation:
		w.Lifetime[i] = radiationInitialLifetime
	case world.MatPlasma:
		w.Lifetime[i] = plasmaInitialLifetime
		w.Temperature[i] = plasmaHeat
	case world.MatCarrion:
		w.Lifetime[i] = carrionInitialLifetime
	case world.MatSoil, world.MatSand:
		// Give fresh ground enough moisture to support growth, otherwise
		// anything planted on it withers immediately.
		if w.Moisture[i] < 90 {
			w.Moisture[i] = 90
		}
	}

	// Registry elements the player paints are loose material: they fall, flow
	// and sink under gravity until they come to rest. Elements born of a
	// reaction or drawn from the generated world are not flagged, so they stay
	// settled — which lets a chemistry scene hold its geometry.
	if _, ok := world.Lookup(mat); ok {
		w.Flags[i] |= world.FlagMobile
	}
}

// applyErase clears a cell and the transient state attached to it.
func applyErase(w *world.World, x, y, i int) {
	w.SetMaterial(x, y, world.MatEmpty)
	w.Lifetime[i] = 0
}

// applyRaise builds ground upward: an empty cell resting directly on solid
// terrain becomes soil. Sweeping the brush over a hillside thickens it.
func applyRaise(w *world.World, x, y, i int) {
	if w.Material[i] != world.MatEmpty {
		return
	}
	if !isSolidTerrain(w.GetMaterial(x, y+1)) {
		return
	}
	w.SetMaterial(x, y, world.MatSoil)
	if w.Moisture[i] < 90 {
		w.Moisture[i] = 90
	}
}

// applyLower carves ground away: an exposed surface cell is removed. Repeated
// passes dig downward one layer at a time.
func applyLower(w *world.World, x, y, i int) {
	if !isSolidTerrain(w.Material[i]) {
		return
	}
	if w.GetMaterial(x, y-1) != world.MatEmpty {
		return // not an exposed surface, so nothing to strip yet
	}
	w.SetMaterial(x, y, world.MatEmpty)
}

// isSolidTerrain reports whether a material forms diggable ground.
func isSolidTerrain(m uint8) bool {
	switch m {
	case world.MatRock, world.MatSoil, world.MatSand, world.MatIce, world.MatAsh:
		return true
	default:
		return false
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

// applyGrowth seeds vegetation on moist ground.
//
// Grass comes first and most readily: it is the renewable base of the food web,
// so encouraging it is how a player feeds grazers. Woody growth appears less
// often and on wetter ground, and grass already present may be promoted into it.
func applyGrowth(w *world.World, x, y, i int, intensity float32) {
	mat := w.Material[i]

	// Promote established grass into woody growth where the ground is rich.
	if mat == world.MatGrass && w.GetMoisture(x, y+1) >= 120 {
		if w.RNG().Float32() < intensity*0.02 {
			w.SetMaterial(x, y, world.MatPlant)
		}
		return
	}

	if mat != world.MatSoil && mat != world.MatSand {
		return
	}
	if w.Moisture[i] < 20 {
		return
	}

	above := w.Index(x, y-1)
	if above < 0 || w.Material[above] != world.MatEmpty {
		return
	}

	if w.RNG().Float32() < intensity*0.12 {
		w.SetMaterial(x, y-1, world.MatGrass)
		// Damp the ground a little so the new growth can hold on.
		if w.Moisture[i] < 90 {
			w.Moisture[i] = 90
		}
	}
}

// applyWind nudges sand, smoke, cloud, and lighter materials horizontally.
func applyWind(w *world.World, x, y, i int, intensity float32) {
	mat := w.Material[i]
	if mat == world.MatSand || mat == world.MatSmoke || mat == world.MatVapor || mat == world.MatEmber || mat == world.MatCloud {
		dx := 1
		if intensity < 0 {
			dx = -1
		}
		// Cloud cells get pushed 2 cells (power=2) for stronger directional control
		if mat == world.MatCloud {
			nx := x + dx
			if w.GetMaterial(nx, y) == world.MatEmpty {
				w.Swap(x, y, nx, y)
				markMoved(w, nx, y)
				// Second push
				nx2 := nx + dx
				if w.GetMaterial(nx2, y) == world.MatEmpty {
					w.Swap(nx, y, nx2, y)
					markMoved(w, nx2, y)
				}
			}
		} else {
			if w.GetMaterial(x+dx, y) == world.MatEmpty {
				w.Swap(x, y, x+dx, y)
				markMoved(w, x+dx, y)
			}
		}
	}
}

// applyLife spawns grazing creatures on empty cells.
//
// Which grazer appears depends on the ground: sheep favour grassy soil, plain
// herbivores turn up anywhere. Newborns are given their species' starting
// reserve, otherwise they would starve on their first update.
func applyLife(w *world.World, x, y, i int, intensity float32) {
	if w.Material[i] != world.MatEmpty {
		return
	}
	if w.RNG().Float32() >= intensity*0.08 {
		return
	}

	species := world.MatHerbivore
	if w.GetMaterial(x, y+1) == world.MatSoil && w.RNG().Intn(2) == 0 {
		species = world.MatSheep
	}

	w.SetMaterial(x, y, species)
	if t, ok := Traits[species]; ok {
		w.Energy[i] = t.StartEnergy
		w.Thirst[i] = 0
	}
}
