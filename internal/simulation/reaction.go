// Declarative reactions — pairwise chemistry as data.
//
// # Why a table
//
// Interactions used to be if-chains inside per-material handlers. Plasma's effect
// on five other materials was a switch in exotic.go, fire's spread was a different
// switch in fire.go, and lava quenching into rock was a third in lava.go. Nothing
// enumerated what reacted with what, so answering "what does water do?" meant
// reading every file that mentioned water, and the client could not show a player
// an element's behaviour because that knowledge existed only as control flow.
//
// A table makes the interaction set enumerable, which is what lets it be served to
// the client, validated for consistency, and tested row by row.
//
// # Performance
//
// A naive scan of 65 reactions against 4 neighbours is 260 comparisons per cell per
// tick, which at 2M cells is far outside the 16 ms budget in tech.md. Two things
// fix that:
//
//   - The table is indexed at startup into a flat [256][256] array of reaction
//     indices, so a lookup is one array read.
//   - Cells whose element has zero reactivity skip the neighbour walk entirely.
//     Rock, sand, soil and empty are the overwhelming majority of a real world, so
//     the pass costs nothing across most of the grid.
//
// See .kiro/specs/atomic-elements/design.md §3 and ADR-011.
package simulation

import (
	"fmt"
	"sync"

	"github.com/worldweaver/worldweaver/internal/world"
)

// Reaction describes one pairwise transformation.
//
// The pair is ORDER-INSENSITIVE: the index is built with both orderings, so
// sodium + water and water + sodium resolve to the same row and the table needs
// only one entry per pair.
type Reaction struct {
	A, B uint8 // reactants

	// ProductA is what A becomes and ProductB what B becomes. Either may be
	// MatEmpty — a reaction that consumes a reactant outright is legitimate.
	ProductA uint8
	ProductB uint8

	// MinTemp is the temperature in whole degrees Celsius at or above which the
	// reaction proceeds. world.TempNone means no requirement, which is how the
	// alkali metals react with water at room temperature.
	MinTemp int16

	// Catalyst must be present in the 3x3 neighbourhood for the reaction to
	// proceed. Zero means none required. This is how the Haber process is gated
	// behind iron without hardcoding it.
	Catalyst uint8

	// HeatDelta is the temperature change written into the surrounding cells:
	// positive for exothermic, negative for endothermic. This is what makes chains
	// work — an exothermic reaction can raise a neighbour past ITS threshold, so
	// thermite propagation is emergent rather than coded.
	//
	// It is NOT the molar enthalpy. 852 kJ/mol is not 852 degrees; the values here
	// are that enthalpy scaled into a per-cell temperature delta.
	HeatDelta int16

	// Chance is a 1-in-N roll per adjacency check. A reaction that fired on every
	// check would consume a reactant mass instantly at 60 ticks per second; the
	// roll spreads it over a visible duration. Element reactivity scales this.
	Chance uint16

	// Equation is the real chemical equation being modelled, kept as documentation
	// so the table can be checked against chemistry rather than taste. It is also
	// served to the client and shown in the element drawer.
	Equation string
}

// reactionTable is the authoritative list. See reactions_data.go.
var reactionTable []Reaction

// reactionIndex maps an ordered pair to a reactionTable position, or -1.
//
// 256*256 int16 is 128 KB, built once at startup. That is a deliberate trade: a
// flat array costs memory that a map would not, but a lookup is a single indexed
// read with no hashing on the hottest path in the simulation.
var reactionIndex [256][256]int16

// noReaction marks an unoccupied slot in the index.
const noReaction int16 = -1

// buildReactionIndex populates the lookup from the table.
//
// Both orderings are written so callers never have to normalise the pair.
func buildReactionIndex() error {
	for a := range reactionIndex {
		for b := range reactionIndex[a] {
			reactionIndex[a][b] = noReaction
		}
	}

	for i := range reactionTable {
		r := &reactionTable[i]

		if existing := reactionIndex[r.A][r.B]; existing != noReaction {
			return fmt.Errorf("reaction %d (%s) collides with reaction %d (%s): "+
				"the pair %d+%d is defined twice, so one silently wins",
				i, r.Equation, existing, reactionTable[existing].Equation, r.A, r.B)
		}

		reactionIndex[r.A][r.B] = int16(i)
		reactionIndex[r.B][r.A] = int16(i)
	}
	return nil
}

// BuildStatus guards the one-time lazy construction of reactionIndex. The index
// is built on first use rather than in a global init because the table rows live
// in reactions_data.go's init and Go does not guarantee module-init ordering for
// separate files. Leaving the build to a lazy gate makes the table correct in
// production even though nothing between startup and first use calls it — which
// was the bug behind every placed element reacting with empty space.
var (
	buildLock   sync.Mutex
	buildErr    error
	reactionBuilt bool
)

// ensureReactionIndex builds the lookup table once before any reaction is read.
func ensureReactionIndex() {
	if reactionBuilt {
		return
	}
	buildLock.Lock()
	if !reactionBuilt {
		buildErr = buildReactionIndex()
		reactionBuilt = true
	}
	buildLock.Unlock()
}

// LookupReactionDebug exposes the built index for the reaction pairs checked
// by the failing chemistry tests. Keeping it in-front behind a named test hook.
func LookupReactionDebug(a, b uint8) string {
	r := lookupReaction(a, b)
	if r == nil {
		return "(none)"
	}
	return r.Equation
}

// lookupReaction returns the reaction for a pair, or nil.
func lookupReaction(a, b uint8) *Reaction {
	ensureReactionIndex()
	i := reactionIndex[a][b]
	if i == noReaction {
		return nil
	}
	return &reactionTable[i]
}

// ValidateReactions checks the table against the element registry.
//
// This is what makes a 65-row table safe to hand-edit. A reaction naming an
// unregistered element would turn cells into something with no definition and
// therefore no behaviour, which is nearly impossible to diagnose from the
// symptom.
func ValidateReactions() error {
	known := func(id uint8) bool {
		if id == world.MatEmpty {
			return true
		}
		// Legacy materials are valid reactants even though they predate the
		// registry, so the check accepts anything at or below the last legacy ID.
		if id <= world.MatGrass {
			return true
		}
		_, ok := world.Lookup(id)
		return ok
	}

	for i := range reactionTable {
		r := &reactionTable[i]

		for label, id := range map[string]uint8{
			"reactant A": r.A, "reactant B": r.B,
			"product A": r.ProductA, "product B": r.ProductB,
		} {
			if !known(id) {
				return fmt.Errorf("reaction %q names %s = element %d, which is "+
					"not registered", r.Equation, label, id)
			}
		}

		if r.Catalyst != 0 && !known(r.Catalyst) {
			return fmt.Errorf("reaction %q needs catalyst element %d, which is "+
				"not registered", r.Equation, r.Catalyst)
		}

		if r.Chance == 0 {
			return fmt.Errorf("reaction %q has Chance 0, which would divide by "+
				"zero on the roll", r.Equation)
		}

		if r.Equation == "" {
			return fmt.Errorf("reaction %d+%d has no equation recorded, so it "+
				"cannot be checked against real chemistry", r.A, r.B)
		}
	}

	ensureReactionIndex()
	return buildErr
}

// ReactionsFor returns every reaction an element takes part in.
//
// This is what the client's element drawer shows, so a player can read what an
// element does instead of discovering it by accident.
func ReactionsFor(id uint8) []Reaction {
	var out []Reaction
	for i := range reactionTable {
		r := reactionTable[i]
		if r.A == id || r.B == id {
			out = append(out, r)
		}
	}
	return out
}

// AllReactions returns the whole table, for the HTTP endpoint and tests.
func AllReactions() []Reaction {
	out := make([]Reaction, len(reactionTable))
	copy(out, reactionTable)
	return out
}
