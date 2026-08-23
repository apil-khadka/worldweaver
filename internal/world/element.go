// Element registry — material behaviour as data rather than control flow.
//
// # Why this exists
//
// Material behaviour used to be procedural: a dispatch switch routed each cell to
// a per-material handler, and every interaction lived as an if-chain inside one of
// those handlers. Classification was a second hardcoded layer of eight predicate
// functions plus Density(), each its own switch over the same IDs.
//
// That structure has three costs. Adding a material means editing every file that
// tests for a class it belongs to, with no compile-time signal that the set is
// complete — a material omitted from Density() silently sinks like every other
// omission. Interactions are not enumerable, so nothing can answer "what does
// water react with?" without reading every file that mentions water, and the
// client cannot show a player what an element does. And the palette ends up
// duplicated across the Go constants, the HTML buttons and each renderer's colour
// table, kept in sync by hand.
//
// An Element record replaces the predicates with fields. Phase selects the
// movement rule, Density orders sinking, and MeltsInto / BoilsInto / BurnsInto
// replace the per-material state-change code. Adding an element becomes a row.
//
// # Physical values are real
//
// Numbers come from the CRC Handbook of Chemistry and Physics and the NIST
// Chemistry WebBook, not from taste. Behaviour that falls out of correct values
// costs nothing to tune: tungsten melts at 3422 °C and therefore survives a lava
// flow, gallium melts at 30 °C and therefore melts in a warm room.
//
// See .kiro/specs/atomic-elements/design.md and ADR-011.
package world

import "fmt"

// Category groups elements for the client's element drawer and for the level
// gates that unlock them.
type Category uint8

const (
	CatTerrain Category = iota
	CatFluid
	CatAlkaliMetal
	CatAlkalineEarth
	CatTransitionMetal
	CatPostTransition
	CatMetalloid
	CatNonmetal
	CatHalogen
	CatNobleGas
	CatActinide
	CatCompound
	CatOrganic
	CatMixture
	CatLife
	CatExotic
)

// CategoryName returns the display label used by the client.
func CategoryName(c Category) string {
	switch c {
	case CatTerrain:
		return "Terrain"
	case CatFluid:
		return "Fluids"
	case CatAlkaliMetal:
		return "Alkali Metals"
	case CatAlkalineEarth:
		return "Alkaline Earth"
	case CatTransitionMetal:
		return "Transition Metals"
	case CatPostTransition:
		return "Post-Transition"
	case CatMetalloid:
		return "Metalloids"
	case CatNonmetal:
		return "Nonmetals"
	case CatHalogen:
		return "Halogens"
	case CatNobleGas:
		return "Noble Gases"
	case CatActinide:
		return "Actinides"
	case CatCompound:
		return "Compounds"
	case CatOrganic:
		return "Organics"
	case CatMixture:
		return "Mixtures"
	case CatLife:
		return "Life"
	case CatExotic:
		return "Exotic"
	default:
		return "Other"
	}
}

// Phase is the state of matter at the cell's current temperature, and selects
// which movement rule the simulation applies. It replaces the IsSolid / IsLiquid
// / IsGas predicate switches.
type Phase uint8

const (
	// PhaseRigid does not move: rock, iron, structural material.
	PhaseRigid Phase = iota
	// PhasePowder falls and piles at an angle of repose: sand, salt, ash.
	PhasePowder
	// PhaseLiquid falls and spreads laterally: water, oil, mercury.
	PhaseLiquid
	// PhaseGas rises and disperses: hydrogen, chlorine, steam.
	PhaseGas
	// PhaseEnergy is a transient effect rather than matter: fire, plasma.
	PhaseEnergy
	// PhaseLife is a creature, moved by its own behaviour rather than physics.
	PhaseLife
)

// PhaseName returns the display label for a phase.
func PhaseName(p Phase) string {
	switch p {
	case PhaseRigid:
		return "solid"
	case PhasePowder:
		return "powder"
	case PhaseLiquid:
		return "liquid"
	case PhaseGas:
		return "gas"
	case PhaseEnergy:
		return "energy"
	case PhaseLife:
		return "life"
	default:
		return "unknown"
	}
}

// NoTransition marks a MeltsInto / BoilsInto / BurnsInto field as unset.
//
// Zero would mean MatEmpty, which is a legitimate product — burning something to
// nothing is a real transition — so the sentinel has to be a value no element
// uses. 255 is the top of the uint8 range and is deliberately never registered.
const NoTransition uint8 = 255

// TempNone marks a temperature threshold as "no requirement".
const TempNone int16 = -32768

// Element describes one material completely.
//
// Temperatures are WHOLE degrees Celsius, not tenths. The simulation's older
// convention was tenths, which capped int16 at 3276 °C — below tungsten's boiling
// point of 5555 °C and too close to a thermite reaction's 2500 °C for comfort.
// Switching the unit buys the full -32768..32767 range and costs a decimal place
// nothing ever read. See design.md §2.1.
type Element struct {
	ID     uint8
	Name   string
	Symbol string // chemical symbol; empty for mixtures like thermite
	Atomic uint8  // atomic number; zero for compounds and mixtures

	Category Category
	Phase    Phase

	Density      int16 // kg/m³ — orders sinking and floating
	MeltingPoint int16 // °C
	BoilingPoint int16 // °C
	IgnitionTemp int16 // °C; TempNone if it does not ignite

	Flammability uint8 // 0..255
	Conductivity uint8 // 0..255, thermal
	Reactivity   uint8 // 0..255; zero skips the reaction pass entirely

	MeltsInto uint8 // element above MeltingPoint; NoTransition if none
	BoilsInto uint8 // element above BoilingPoint; NoTransition if none
	BurnsInto uint8 // element when ignited; NoTransition if none

	Colour  [4]uint8 // RGBA, served to the client
	Flavour string   // one line shown in the element drawer
}

// registry is the ID-indexed element table, built once at init.
var registry [256]*Element

// registered preserves declaration order for All(), so the drawer lists elements
// in a curated order rather than by numeric ID.
var registered []*Element

// register adds an element to the registry. It panics on a duplicate ID because a
// duplicate is a programming error that must not reach a running world: the second
// registration would silently shadow the first and every cell of the shadowed
// element would change behaviour.
func register(e Element) {
	if registry[e.ID] != nil {
		panic(fmt.Sprintf("element ID %d registered twice: %q and %q",
			e.ID, registry[e.ID].Name, e.Name))
	}
	copied := e
	registry[e.ID] = &copied
	registered = append(registered, &copied)
}

// Lookup returns the element for an ID.
//
// The second return is false for IDs with no definition, which includes the
// legacy materials not yet migrated into the registry. Callers must treat that as
// "fall back to the hardcoded predicate", not as an error — the two systems
// coexist during the migration.
func Lookup(id uint8) (*Element, bool) {
	e := registry[id]
	return e, e != nil
}

// All returns every registered element in declaration order.
func All() []*Element {
	out := make([]*Element, len(registered))
	copy(out, registered)
	return out
}

// ByCategory groups the registry for the client's drawer.
func ByCategory() map[Category][]*Element {
	out := make(map[Category][]*Element)
	for _, e := range registered {
		out[e.Category] = append(out[e.Category], e)
	}
	return out
}

// Count reports how many elements are registered.
func Count() int { return len(registered) }

// isKnownMaterial reports whether an ID has behaviour defined anywhere.
//
// A registry element counts, and so does a legacy material: the 23 pre-registry
// IDs keep their hardcoded handlers and are perfectly valid as a transition target
// or a reaction product. Treating them as unknown would reject every transition
// into fire, smoke or water, which are the most common products there are.
func isKnownMaterial(id uint8) bool {
	if id <= MatGrass {
		return true
	}
	_, ok := Lookup(id)
	return ok
}

// Validate checks the registry for internal contradictions.
//
// This is what makes a large table safe to edit by hand. Every failure here is a
// mistake that would otherwise surface as inexplicable simulation behaviour: an
// element that boils below its melting point never becomes a liquid, and a
// transition naming an unregistered element turns cells into something with no
// definition and therefore no behaviour at all.
func Validate() error {
	for _, e := range registered {
		if e.Name == "" {
			return fmt.Errorf("element %d has no name", e.ID)
		}

		if e.ID == NoTransition {
			return fmt.Errorf("element %q uses ID %d, which is reserved as the "+
				"NoTransition sentinel", e.Name, NoTransition)
		}

		// A melting point above the boiling point describes a substance that is
		// solid, then gas, and never liquid. Every real case of this in a draft
		// table was a transposition typo.
		if e.MeltingPoint > e.BoilingPoint {
			return fmt.Errorf("element %q melts at %d °C but boils at %d °C, so "+
				"it would never be liquid", e.Name, e.MeltingPoint, e.BoilingPoint)
		}

		for label, target := range map[string]uint8{
			"MeltsInto": e.MeltsInto,
			"BoilsInto": e.BoilsInto,
			"BurnsInto": e.BurnsInto,
		} {
			if target == NoTransition {
				continue
			}
			if !isKnownMaterial(target) {
				return fmt.Errorf("element %q %s element %d, which is not registered",
					e.Name, label, target)
			}
		}

		if e.Flammability > 0 && e.IgnitionTemp == TempNone {
			return fmt.Errorf("element %q is flammable but has no ignition "+
				"temperature, so it can never actually catch", e.Name)
		}
	}
	return nil
}

// ── Registry-backed property accessors ───────────────────────────────────────
//
// Each falls back to the legacy hardcoded predicate when an ID has no element
// record, so the 23 pre-registry materials keep working unchanged.

// PhaseOf returns an element's phase, falling back to the legacy predicates.
func PhaseOf(m uint8) Phase {
	if e, ok := Lookup(m); ok {
		return e.Phase
	}
	switch {
	case IsCreature(m):
		return PhaseLife
	case IsGas(m):
		return PhaseGas
	case IsLiquid(m):
		return PhaseLiquid
	case IsSolid(m):
		return PhaseRigid
	default:
		return PhaseEnergy
	}
}

// DensityOf returns kg/m³ for registered elements.
//
// Legacy materials use the old 0..255 relative scale, which is not comparable, so
// they are mapped onto the real scale by multiplying by 12 — chosen so the legacy
// water value of 80 lands near water's real 1000 kg/m³ and the ordering between
// old and new materials stays correct.
func DensityOf(m uint8) int16 {
	if e, ok := Lookup(m); ok {
		return e.Density
	}
	return int16(Density(m)) * 12
}

// ReactivityOf reports how readily an element reacts. Zero means the reaction pass
// skips the cell entirely, which is what keeps rock and sand free.
func ReactivityOf(m uint8) uint8 {
	if e, ok := Lookup(m); ok {
		return e.Reactivity
	}
	return 0
}

// ElementName prefers the registry's name and falls back to the legacy switch.
func ElementName(m uint8) string {
	if e, ok := Lookup(m); ok {
		return e.Name
	}
	return MaterialName(m)
}
