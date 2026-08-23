package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Grass is the base of the food web.
//
// It is deliberately separate from MatPlant. Woody growth forms trees and
// regrows over minutes, which is far too slow to feed grazers: a flock strips it
// and the population collapses with no way back. Grass regrows in seconds, so
// grazing pressure and regrowth can reach a balance.
//
// The regrowth rate is the local carrying capacity of the food web. Lattice
// Lotka-Volterra models are only stable when the prey's food is finite and
// replenishing; without that the system either diverges or collapses to an
// absorbing state.

const (
	// grassSpreadInterval is how often grass attempts to spread, in ticks.
	// Roughly 6 Hz at 60 TPS — fast enough to recover from grazing.
	grassSpreadInterval uint64 = 10

	// grassSpreadChance is a 1-in-N roll per attempt.
	grassSpreadChance = 5

	// grassMoistureMin is the soil moisture grass needs to take root.
	grassMoistureMin uint8 = 25

	// Grass survives a wider temperature band than woody plants but still burns
	// off in extreme heat. Values are tenths of a degree.
	grassTempMin int16 = -300
	grassTempMax int16 = 520

	// grassDrySpreadPenalty makes grass spread more reluctantly on poor ground.
	grassDrySpreadPenalty = 4
)

// simulateGrass grows grass across damp ground and burns it off where it is too
// hot or too dry.
func simulateGrass(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Grass needs ground beneath it. Without support it withers, which stops it
	// hanging in the air after the soil below is dug out.
	below := w.GetMaterial(x, y+1)
	if below != world.MatSoil && below != world.MatSand && below != world.MatGrass {
		w.SetMaterial(x, y, world.MatEmpty)
		return
	}

	temp := w.Temperature[i]
	if temp > grassTempMax {
		// Scorched: leaves ash rather than vanishing, so the nutrients stay put.
		w.SetMaterial(x, y, world.MatAsh)
		return
	}
	if temp < grassTempMin {
		w.SetMaterial(x, y, world.MatEmpty)
		return
	}

	if w.Tick.Load()%grassSpreadInterval != 0 {
		return
	}

	rootMoisture := w.GetMoisture(x, y+1)
	if rootMoisture < grassMoistureMin {
		// Dry ground: grass dies back slowly rather than persisting on bare sand.
		if w.RNG().Intn(120) == 0 {
			w.SetMaterial(x, y, world.MatEmpty)
		}
		return
	}

	chance := grassSpreadChance
	if rootMoisture < 90 {
		chance *= grassDrySpreadPenalty
	}
	if w.RNG().Intn(chance) != 0 {
		return
	}

	// Spread sideways onto adjacent bare ground.
	dirs := [2]int{-1, 1}
	first := int(w.Tick.Load()) % 2
	for k := 0; k < 2; k++ {
		dx := dirs[(first+k)%2]
		nx := x + dx
		ni := w.Index(nx, y)
		if ni < 0 || w.Material[ni] != world.MatEmpty {
			continue
		}
		ground := w.GetMaterial(nx, y+1)
		if ground != world.MatSoil && ground != world.MatSand {
			continue
		}
		if w.GetMoisture(nx, y+1) < grassMoistureMin {
			continue
		}
		w.SetMaterial(nx, y, world.MatGrass)
		return
	}
}
