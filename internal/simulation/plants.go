package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Plant growth depends on BOTH moisture AND temperature:
//   - Needs minimum soil moisture to grow
//   - Needs temperature within a viable range (not too cold, not too hot)
//   - Dies from extreme heat (fire proximity, drought desiccation)
//   - Dies from prolonged drought (soil dries out completely)
//
// Temperature model (stored as tenths of °C):
//   - Optimal growth range: 10°C - 35°C (100 - 350 units)
//   - Heat death threshold: 60°C (600 units)
//   - Cold dormancy below 5°C (50 units) — no growth but survives
//
// Moisture model:
//   - Growth requires adjacent soil moisture >= 30
//   - Drought death: if ALL adjacent soil has moisture == 0 for extended period
//     (simulated probabilistically since we don't track per-plant timers)

const (
	plantMoistureMin   = 30  // minimum soil moisture for growth
	plantTempOptLow    = 100 // 10.0 °C — minimum for growth
	plantTempOptHigh   = 350 // 35.0 °C — maximum for optimal growth
	plantTempHeatDeath = 600 // 60.0 °C — plant dies instantly
	plantTempColdStop  = 50  // 5.0 °C — too cold to grow (but survives)
	plantGrowthChance  = 180 // 1-in-N chance per tick (tuned for nice spread)
	plantDroughtDeath  = 40  // 1-in-N chance to die per tick when bone-dry
)

func simulatePlant(w *world.World, x, y int) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	temp := w.Temperature[i]

	// Instant death from extreme heat (wildfire, lava proximity)
	if temp > plantTempHeatDeath {
		// Plant desiccates and dies — could ignite if hot enough
		if temp > 800 && w.RNG().Intn(3) == 0 {
			w.SetMaterial(x, y, world.MatFire)
			w.Lifetime[i] = 0 // fire will initialize
		} else {
			w.SetMaterial(x, y, world.MatEmpty)
		}
		return
	}

	// Drought death: check if any adjacent soil has moisture
	hasMoisture := false
	for _, d := range [][2]int{{0, 1}, {-1, 0}, {1, 0}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		// Sand counts as a root substrate alongside soil, otherwise desert
		// vegetation can never hold on even where the sand is damp.
		m := w.GetMaterial(nx, ny)
		if (m == world.MatSoil || m == world.MatSand) && w.GetMoisture(nx, ny) > 0 {
			hasMoisture = true
			break
		}
	}

	// Cells partway up a trunk or out in a canopy are not themselves next to
	// soil; they are fed through the stem below them. Treating them as
	// waterless killed anything taller than a single cell within seconds, so a
	// plant supported from below counts as sustained. If the base dies the
	// support disappears and the rest of the plant withers from the bottom up.
	if !hasMoisture && w.GetMaterial(x, y+1) == world.MatPlant {
		hasMoisture = true
	}

	if !hasMoisture {
		// Probabilistic drought death — over time dry plants wither
		if w.RNG().Intn(plantDroughtDeath) == 0 {
			w.SetMaterial(x, y, world.MatEmpty)
			return
		}
	}

	// Growth conditions: temperature must be in viable range AND moisture available
	if temp < plantTempColdStop || temp > plantTempOptHigh {
		return // dormant — too cold or too hot for growth
	}

	// Need adjacent soil with sufficient moisture
	bestMoisture := uint8(0)
	for _, d := range [][2]int{{0, 1}, {-1, 0}, {1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatSoil {
			m := w.GetMoisture(nx, ny)
			if m > bestMoisture {
				bestMoisture = m
			}
		}
	}

	if bestMoisture < plantMoistureMin {
		return // insufficient moisture for growth
	}

	// Growth probability scales slightly with moisture (healthier = faster)
	growChance := plantGrowthChance
	if bestMoisture > 100 {
		growChance = growChance * 2 / 3 // 33% faster when well-watered
	}

	if w.RNG().Intn(growChance) != 0 {
		return
	}

	// Attempt to spread to an adjacent empty cell that is on soil
	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}}
	// Shuffle direction preference using tick for variety
	start := int(w.Tick) % len(dirs)
	for offset := range len(dirs) {
		d := dirs[(start+offset)%len(dirs)]
		nx, ny := x+d[0], y+d[1]
		if w.GetMaterial(nx, ny) == world.MatEmpty {
			// Needs soil beneath to root into
			if w.GetMaterial(nx, ny+1) == world.MatSoil &&
				w.GetMoisture(nx, ny+1) >= plantMoistureMin {
				w.SetMaterial(nx, ny, world.MatPlant)
				return
			}
		}
	}
}
