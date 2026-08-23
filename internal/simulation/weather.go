package simulation

import (
	"math"

	"github.com/worldweaver/worldweaver/internal/world"
)

// Weather cycle constants — tunable parameters for the emergent weather system.
const (
	// Evaporation: water cells above this temperature (in the Temperature array)
	// have a chance to become vapor each tick.
	weatherEvapTempThreshold int16   = 150
	weatherEvapChance        float64 = 0.02 // probability per tick

	// Cloud formation: vapor cells in the top N% of the world accumulate into clouds.
	cloudZoneFraction float64 = 0.1 // top 10% of world height
	cloudMinAdjacent  int     = 3   // adjacent vapor cells needed to form cloud

	// Rain: probability range for clouds releasing water.
	rainBaseChance float64 = 0.01
	rainMaxChance  float64 = 0.05

	// Wind: clouds drift horizontally based on a global oscillating wind direction.
	windOscillationPeriod float64 = 600.0 // ticks for one full sin cycle
)

// GlobalWindDirection returns the current horizontal wind direction for the given tick.
// Returns -1, 0, or 1 based on sin(tick / windOscillationPeriod).
func GlobalWindDirection(tick uint64) int {
	v := math.Sin(float64(tick) / windOscillationPeriod)
	if v > 0.3 {
		return 1
	}
	if v < -0.3 {
		return -1
	}
	return 0
}

// simulateWeatherEvaporation checks if a water cell should evaporate based on temperature.
// Called from the environment pass for water cells with high temperature.
func simulateWeatherEvaporation(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	if w.Material[i] != world.MatWater {
		return
	}
	if w.Temperature[i] > weatherEvapTempThreshold {
		if w.RNG().Float64() < weatherEvapChance {
			w.SetMaterial(x, y, world.MatVapor)
			w.Lifetime[i] = uint16(80 + w.RNG().Intn(80))
		}
	}
}

// simulateCloudFormation checks if vapor cells in the cloud zone should merge into clouds.
// Vapor at the top of the world with enough adjacent vapor cells becomes a cloud.
func simulateCloudFormation(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	if w.Material[i] != world.MatVapor {
		return
	}

	// Only form clouds in the top zone
	cloudZoneHeight := int(float64(w.Height) * cloudZoneFraction)
	if y >= cloudZoneHeight {
		return
	}

	// Count adjacent vapor/cloud cells
	adjacent := 0
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		nx, ny := x+d[0], y+d[1]
		mat := w.GetMaterial(nx, ny)
		if mat == world.MatVapor || mat == world.MatCloud {
			adjacent++
		}
	}

	if adjacent >= cloudMinAdjacent {
		w.SetMaterial(x, y, world.MatCloud)
		w.Lifetime[i] = 0 // clouds have no decay timer
	}
}

// simulateCloud handles cloud behavior: wind drift and rain release.
func simulateCloud(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Wind drift: move cloud horizontally based on global wind direction
	windDir := GlobalWindDirection(w.Tick)
	if windDir != 0 {
		nx := x + windDir
		if w.GetMaterial(nx, y) == world.MatEmpty {
			w.Swap(x, y, nx, y)
			markMoved(w, nx, y)
			// Update coordinates after drift
			x = nx
			i = w.Index(x, y)
			if i < 0 {
				return
			}
		}
	}

	// Rain: release water below the cloud
	adjacentClouds := countAdjacentClouds(w, x, y)
	rainChance := rainBaseChance + (rainMaxChance-rainBaseChance)*float64(adjacentClouds)/8.0
	if rainChance > rainMaxChance {
		rainChance = rainMaxChance
	}

	if w.RNG().Float64() < rainChance {
		// Precipitation must conserve mass: the cloud becomes the raindrop rather
		// than emitting one while persisting. Leaving the cloud in place turned
		// every cloud into an endless water source, and a world left running
		// flooded solid — 96% of cells occupied, with the surface never settling.
		below := y + 1
		if below < w.Height && w.GetMaterial(x, below) == world.MatEmpty {
			w.SetMaterial(x, below, world.MatWater)
			w.SetMaterial(x, y, world.MatEmpty)
		}
	}
}

// countAdjacentClouds returns how many of the 8 neighbors are cloud cells.
func countAdjacentClouds(w *world.World, x, y int) int {
	count := 0
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		if w.GetMaterial(x+d[0], y+d[1]) == world.MatCloud {
			count++
		}
	}
	return count
}

// updateWeatherCycle runs the weather-specific environment pass on active chunks.
// Called from the main environment update after temperature processing.
func updateWeatherCycle(w *world.World) {
	chunkSize := w.ChunkSize

	for cy := range w.ChunkH {
		for cx := range w.ChunkW {
			idx := cy*w.ChunkW + cx
			if w.Chunks[idx].Sleeping {
				continue
			}

			startX := cx * chunkSize
			startY := cy * chunkSize
			endX := startX + chunkSize
			endY := startY + chunkSize
			if endX > w.Width {
				endX = w.Width
			}
			if endY > w.Height {
				endY = w.Height
			}

			for y := startY; y < endY; y++ {
				for x := startX; x < endX; x++ {
					i := y*w.Width + x
					mat := w.Material[i]

					switch mat {
					case world.MatWater:
						simulateWeatherEvaporation(w, x, y)
					case world.MatVapor:
						simulateCloudFormation(w, x, y)
					}
				}
			}
		}
	}
}
