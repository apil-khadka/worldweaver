package game

import "github.com/worldweaver/worldweaver/internal/world"

// WorldStability is a normalized composite metric (0.0–1.0) that represents
// the health of the shared ecosystem.
//
// This is a game mechanic, NOT a scientific ecosystem model.
// The formula is designed to produce interesting emergent feedback loops
// for players, not to simulate real ecology accurately.
//
// Formula:
//
//	stability = 0.25*temperatureScore + 0.25*moistureScore
//	          + 0.25*vegetationScore  + 0.25*fireScore
type WorldStability struct {
	Overall     float32
	Temperature float32
	Moisture    float32
	Vegetation  float32
	Fire        float32
}

// Compute derives world stability from the current world state.
//
// It performs a full scan of all cells, so it should be called at a low
// frequency (e.g., every 30 ticks) rather than every simulation tick.
func Compute(w *world.World) WorldStability {
	var totalCells int
	var fireCells, plantCells int
	var totalTemp, totalMoisture int64

	for i, mat := range w.Material {
		totalCells++
		switch mat {
		case world.MatFire:
			fireCells++
		case world.MatPlant:
			plantCells++
		}
		totalTemp += int64(w.Temperature[i])
		totalMoisture += int64(w.Moisture[i])
	}

	if totalCells == 0 {
		return WorldStability{}
	}

	// Temperature score: ideal avg ≈ 200 (20.0 °C stored as fixed-point tenths).
	// Score degrades as average drifts above the ideal.
	avgTemp := float32(totalTemp) / float32(totalCells)
	tempScore := 1.0 - clampF((avgTemp-200)/500, 0, 1)

	// Moisture score: ideal average moisture byte ≈ 80/255.
	avgMoisture := float32(totalMoisture) / float32(totalCells)
	moistureScore := 1.0 - clampF((80-avgMoisture)/80, 0, 1)

	// Vegetation score: more plants → better, up to a cap.
	vegetationFraction := float32(plantCells) / float32(totalCells)
	vegetationScore := clampF(vegetationFraction*50, 0, 1)

	// Fire score: any active fire reduces the ecosystem score.
	fireFraction := float32(fireCells) / float32(totalCells)
	fireScore := 1.0 - clampF(fireFraction*1000, 0, 1)

	overall := 0.25*tempScore + 0.25*moistureScore + 0.25*vegetationScore + 0.25*fireScore

	return WorldStability{
		Overall:     overall,
		Temperature: tempScore,
		Moisture:    moistureScore,
		Vegetation:  vegetationScore,
		Fire:        fireScore,
	}
}

func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
