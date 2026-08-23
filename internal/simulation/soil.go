package simulation

import "github.com/worldweaver/worldweaver/internal/world"

const (
	// soilSproutInterval is how often damp soil is considered for sprouting.
	soilSproutInterval uint64 = 30

	// soilSproutChance is a 1-in-N roll per attempt. Deliberately slow: this is
	// the seed bank waking up, not grass spreading, which is handled in grass.go.
	soilSproutChance = 900

	// soilSproutMoisture is the moisture needed before anything will sprout.
	soilSproutMoisture uint8 = 60
)

// simulateSoil dries soil under heat and lets damp soil sprout fresh grass.
//
// Spontaneous sprouting exists to stop the food web reaching a dead end. Grass
// otherwise only spreads from existing grass, so once grazers ate the last blade
// the world could never recover: no grass meant no grazers, permanently. That is
// an absorbing state, and lattice predator-prey systems fall into it readily
// unless something reintroduces the bottom of the chain. Treating soil as holding
// a seed bank gives the ecosystem a floor to climb back from.
func simulateSoil(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Dry out under high temperature
	if w.Temperature[i] > 500 { // > 50.0 °C
		if w.Moisture[i] > 0 {
			w.Moisture[i]--
		}
		return
	}

	if w.Tick.Load()%soilSproutInterval != 0 {
		return
	}
	if w.Moisture[i] < soilSproutMoisture {
		return
	}

	above := w.Index(x, y-1)
	if above < 0 || w.Material[above] != world.MatEmpty {
		return
	}

	// Too cold or too hot for grass to establish.
	if t := w.Temperature[above]; t < grassTempMin || t > grassTempMax {
		return
	}

	if w.RNG().Intn(soilSproutChance) == 0 {
		w.SetMaterial(x, y-1, world.MatGrass)
	}
}
