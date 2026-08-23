package simulation

import "github.com/worldweaver/worldweaver/internal/world"

// Species traits.
//
// Behaviour is described by data rather than by one function per creature, so a
// new animal is a table entry instead of another near-duplicate simulate
// function. All three current species share the same update routine.
//
// Energy and thirst live in their own world fields. They previously shared the
// Temperature array, which meant the Heat power wrote straight into a creature's
// energy — heating an animal made it effectively immortal — and left no field
// free to give creatures a temperature response.

// Trait describes one species.
type Trait struct {
	Material uint8
	Name     string

	// Prey lists what this species eats, most preferred first. Grazers take grass
	// before woody growth; predators take whichever grazer is nearest.
	Prey []uint8

	// EatGain is energy recovered per meal.
	//
	// Transfer up the food web is deliberately lossy, following the ecological
	// ten percent rule: only a small share of the energy at one trophic level
	// reaches the next, which is why food chains are short and why each level
	// supports fewer individuals than the one below it. Here that shows up as a
	// predator needing several kills to cover what it spends between them.
	EatGain uint8

	// StartEnergy is the reserve a newborn or spawned creature begins with.
	StartEnergy uint8

	// ReproEnergy is the reserve required before reproducing, and the cost of
	// doing so. It must exceed StartEnergy or a population could breed for free.
	ReproEnergy uint8

	// ReproChance is a 1-in-N roll per eligible tick.
	ReproChance int

	// EnergyDecay is how much reserve is spent per creature update.
	EnergyDecay uint8

	// ThirstRate is how fast thirst accumulates per update.
	ThirstRate uint8

	// SenseRange is how far it looks for food or water, in cells.
	SenseRange int

	// MinTemp and MaxTemp bound the climate it can breed in, in tenths of a
	// degree. Outside this band it survives but cannot reproduce, so unsuitable
	// regions thin out rather than being hard-blocked.
	MinTemp, MaxTemp int16

	// Herds biases movement toward others of its own kind.
	Herds bool
}

// Traits is the species table, keyed by material ID.
var Traits = map[uint8]Trait{
	world.MatHerbivore: {
		Material: world.MatHerbivore, Name: "herbivore",
		// Grass first: it is the abundant, fast-regrowing food. Woody plants are a
		// fallback that yields the same energy but recovers far more slowly, so
		// relying on it depletes the landscape.
		Prey: []uint8{world.MatGrass, world.MatPlant}, EatGain: 45,
		// Breeding needs a nearly full reserve and costs most of it. Cheap
		// reproduction made the population overshoot its food supply and crash to
		// extinction — the divergent behaviour predator-prey models fall into
		// when nothing limits growth before the food runs out.
		StartEnergy: 90, ReproEnergy: 235, ReproChance: 9,
		EnergyDecay: 1, ThirstRate: 1, SenseRange: 5,
		MinTemp: -50, MaxTemp: 420,
	},
	world.MatSheep: {
		Material: world.MatSheep, Name: "sheep",
		Prey: []uint8{world.MatGrass}, EatGain: 45,
		// Hardier and calmer than a herbivore: a bigger reserve, a wider climate
		// band and it flocks, but it breeds least readily and only eats grass.
		StartEnergy: 110, ReproEnergy: 245, ReproChance: 14,
		EnergyDecay: 1, ThirstRate: 1, SenseRange: 4,
		MinTemp: -120, MaxTemp: 380,
		Herds: true,
	},
	world.MatPredator: {
		Material: world.MatPredator, Name: "predator",
		Prey: []uint8{world.MatHerbivore, world.MatSheep}, EatGain: 90,
		// Burns energy twice as fast as its prey and needs a larger reserve to
		// breed, so a given amount of grazing supports far fewer predators than
		// grazers. That ratio is the ten percent rule expressed in cells.
		StartEnergy: 130, ReproEnergy: 248, ReproChance: 10,
		EnergyDecay: 2, ThirstRate: 1, SenseRange: 8,
		MinTemp: -40, MaxTemp: 450,
	},
}

// creatureUpdateInterval is how often creature logic runs. Creatures act on a
// slower schedule than materials; energy figures are tuned against this rate.
const creatureUpdateInterval uint64 = 6

const (
	// thirstDeath is the point at which a creature dies of dehydration.
	thirstDeath uint8 = 255

	// thirstSeek is when it starts prioritising water over food.
	thirstSeek uint8 = 150

	// drinkMoisture is the soil moisture that counts as drinkable.
	drinkMoisture uint8 = 60
)

// simulateCreature advances one creature of any species.
func simulateCreature(w *world.World, x, y int, t Trait) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}

	// Gravity first: a creature with nothing beneath it is falling, not acting.
	if below := w.Index(x, y+1); below >= 0 && w.Material[below] == world.MatEmpty {
		w.Swap(x, y, x, y+1)
		markMoved(w, x, y+1)
		return
	}

	// Creatures update on a slower schedule than materials.
	if w.Tick%creatureUpdateInterval != 0 {
		return
	}

	// Energy is never topped up here. An earlier version refilled any creature
	// found with zero energy, intending to catch ones placed without a reserve;
	// because a creature drinking from damp ground keeps thirst at zero, that
	// condition matched starving creatures too and made them immortal. Every path
	// that creates a creature — generation, the Life power, reproduction, snapshot
	// loading — now seeds the reserve itself.

	// ── Hazards ─────────────────────────────────────────────────────────────
	// Standing next to something lethal kills regardless of reserves.
	for _, d := range cardinalDirs {
		if world.IsHazard(w.GetMaterial(x+d[0], y+d[1])) {
			die(w, x, y, i)
			return
		}
	}

	// ── Upkeep ──────────────────────────────────────────────────────────────
	if w.Energy[i] <= t.EnergyDecay {
		die(w, x, y, i)
		return
	}
	w.Energy[i] -= t.EnergyDecay

	if w.Thirst[i] >= thirstDeath-t.ThirstRate {
		die(w, x, y, i)
		return
	}
	w.Thirst[i] += t.ThirstRate

	// Keep the chunk awake so this creature keeps being simulated. Without it a
	// still creature's chunk sleeps and it neither starves nor acts again.
	w.MarkDirty(x, y)

	// ── Drink ───────────────────────────────────────────────────────────────
	// Drinking is incidental and does not consume the update. Taking the turn
	// meant a creature standing beside water drank on every single update — since
	// thirst rises each time — and so never ate, then starved next to a full
	// water supply.
	drink(w, x, y, i)

	// ── Eat ─────────────────────────────────────────────────────────────────
	if len(t.Prey) > 0 && ate(w, x, y, i, t) {
		return
	}

	// ── Reproduce ───────────────────────────────────────────────────────────
	if w.Energy[i] >= t.ReproEnergy && inBreedingClimate(w, i, t) {
		if w.RNG().Intn(t.ReproChance) == 0 && reproduce(w, x, y, t) {
			w.Energy[i] -= t.StartEnergy
			return
		}
	}

	// ── Move ────────────────────────────────────────────────────────────────
	move(w, x, y, i, t)
}

// die converts a creature to carrion so its biomass returns to the soil.
func die(w *world.World, x, y, i int) {
	w.Energy[i] = 0
	w.Thirst[i] = 0
	w.SetMaterial(x, y, world.MatCarrion)
	w.Lifetime[i] = carrionInitialLifetime
}

// drink quenches thirst from adjacent water or damp ground.
func drink(w *world.World, x, y, i int) {
	if w.Thirst[i] == 0 {
		return
	}
	for _, d := range eightDirs {
		nx, ny := x+d[0], y+d[1]
		ni := w.Index(nx, ny)
		if ni < 0 {
			continue
		}
		if w.Material[ni] == world.MatWater || w.Moisture[ni] >= drinkMoisture {
			w.Thirst[i] = 0
			return
		}
	}
}

// ate consumes an adjacent prey cell, transferring its biomass to the eater.
//
// The prey cell is left empty rather than becoming carrion: that biomass has
// moved into the predator, so turning it into carrion as well would duplicate it.
func ate(w *world.World, x, y, i int, t Trait) bool {
	// Prey is ordered by preference, so a grazer clears grass before starting on
	// woody growth.
	for _, prey := range t.Prey {
		for _, d := range eightDirs {
			nx, ny := x+d[0], y+d[1]
			ni := w.Index(nx, ny)
			if ni < 0 || w.Material[ni] != prey {
				continue
			}
			w.SetMaterial(nx, ny, world.MatEmpty)
			w.Energy[ni] = 0
			w.Thirst[ni] = 0

			gain := int(w.Energy[i]) + int(t.EatGain)
			if gain > 255 {
				gain = 255
			}
			w.Energy[i] = uint8(gain)
			return true
		}
	}
	return false
}

// eatsPrey reports whether a material is on this species' menu.
func eatsPrey(t Trait, m uint8) bool {
	for _, p := range t.Prey {
		if m == p {
			return true
		}
	}
	return false
}

// inBreedingClimate reports whether the local temperature allows reproduction.
func inBreedingClimate(w *world.World, i int, t Trait) bool {
	temp := w.Temperature[i]
	return temp >= t.MinTemp && temp <= t.MaxTemp
}

// reproduce places a newborn in an adjacent empty cell.
func reproduce(w *world.World, x, y int, t Trait) bool {
	start := w.RNG().Intn(len(eightDirs))
	for k := 0; k < len(eightDirs); k++ {
		d := eightDirs[(start+k)%len(eightDirs)]
		nx, ny := x+d[0], y+d[1]
		ni := w.Index(nx, ny)
		if ni < 0 || w.Material[ni] != world.MatEmpty {
			continue
		}
		w.SetMaterial(nx, ny, t.Material)
		w.Energy[ni] = t.StartEnergy
		w.Thirst[ni] = 0
		markMoved(w, nx, ny)
		return true
	}
	return false
}

// move steps the creature one cell, biased by what it currently needs.
//
// Thirst overrides hunger, because dehydration kills sooner than starvation once
// thirst is high. Gradient following is enough for both: moisture is already
// elevated around water, so no pathfinding is required.
func move(w *world.World, x, y, i int, t Trait) {
	var tx, ty int
	var found bool

	switch {
	case w.Thirst[i] >= thirstSeek:
		tx, ty, found = seek(w, x, y, t.SenseRange, func(ni int) bool {
			return w.Material[ni] == world.MatWater || w.Moisture[ni] >= drinkMoisture
		})
	case len(t.Prey) > 0:
		tx, ty, found = seek(w, x, y, t.SenseRange, func(ni int) bool {
			return eatsPrey(t, w.Material[ni])
		})
	}

	// Herd animals fall back to following their own kind when nothing is needed.
	if !found && t.Herds {
		tx, ty, found = seek(w, x, y, t.SenseRange, func(ni int) bool {
			return w.Material[ni] == t.Material
		})
	}

	dir := cardinalDirs[w.RNG().Intn(len(cardinalDirs))]
	if found {
		// Step along the dominant axis toward the target.
		dx, dy := sign(tx-x), sign(ty-y)
		if abs(tx-x) >= abs(ty-y) && dx != 0 {
			dir = [2]int{dx, 0}
		} else if dy != 0 {
			dir = [2]int{0, dy}
		}
	}

	nx, ny := x+dir[0], y+dir[1]
	ni := w.Index(nx, ny)
	if ni < 0 || w.Material[ni] != world.MatEmpty {
		return
	}
	w.Swap(x, y, nx, ny)
	markMoved(w, nx, ny)
}

// seek scans a square neighbourhood for the nearest cell satisfying match.
func seek(w *world.World, x, y, radius int, match func(i int) bool) (int, int, bool) {
	best := radius*radius*4 + 1
	bx, by := 0, 0
	found := false

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			ni := w.Index(nx, ny)
			if ni < 0 || !match(ni) {
				continue
			}
			if d := dx*dx + dy*dy; d < best {
				best, bx, by, found = d, nx, ny, true
			}
		}
	}
	return bx, by, found
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
