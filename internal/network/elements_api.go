// The element catalogue endpoint.
//
// The material list used to be duplicated five ways: the Go constants, the HTML
// palette buttons with inline hex colours, and a colour table in each of the three
// renderers. Keeping them in step was manual, which is why the placeable palette
// stalled at eleven entries while 23 materials existed and eight had no way to be
// placed at all.
//
// Serving the registry makes Go the single source of truth. The client builds its
// drawer, its colour texture and its element reference from this response, so adding
// an element server-side is enough to make it appear and be placeable.
//
// See ADR-011 and .kiro/specs/atomic-elements/requirements.md REQ-AE-004.
package network

import (
	"encoding/json"
	"net/http"

	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// elementDTO is one element as the client needs it.
//
// Deliberately not the internal struct: the wire shape carries display strings the
// simulation does not need (category and phase names, the formatted flavour line) and
// omits fields the client has no use for. Serialising the internal type directly
// would couple the client to field names that exist for simulation reasons.
type elementDTO struct {
	ID     uint8  `json:"id"`
	Name   string `json:"name"`
	Symbol string `json:"symbol,omitempty"`
	Atomic uint8  `json:"atomic,omitempty"`

	Category     string `json:"category"`
	CategoryID   uint8  `json:"categoryId"`
	Phase        string `json:"phase"`
	Placeable    bool   `json:"placeable"`

	Density      int16 `json:"density"`
	MeltingPoint int16 `json:"meltingPoint"`
	BoilingPoint int16 `json:"boilingPoint"`
	IgnitionTemp *int16 `json:"ignitionTemp,omitempty"`

	Flammability uint8 `json:"flammability"`
	Conductivity uint8 `json:"conductivity"`
	Reactivity   uint8 `json:"reactivity"`

	Colour  [4]uint8 `json:"colour"`
	Flavour string   `json:"flavour,omitempty"`

	// Reactions this element takes part in, so the drawer can tell a player what it
	// does instead of leaving them to find out by accident.
	Reactions []reactionDTO `json:"reactions,omitempty"`
}

// reactionDTO is one reaction as shown in the element reference.
type reactionDTO struct {
	With        string `json:"with"`
	Produces    string `json:"produces"`
	Equation    string `json:"equation"`
	NeedsHeat   bool   `json:"needsHeat"`
	MinTempC    *int16 `json:"minTempC,omitempty"`
	Catalyst    string `json:"catalyst,omitempty"`
	Exothermic  bool   `json:"exothermic"`
}

// categoryDTO groups elements for the drawer's sections.
type categoryDTO struct {
	ID       uint8        `json:"id"`
	Name     string       `json:"name"`
	Elements []elementDTO `json:"elements"`
}

// handleElements serves the element catalogue.
func handleElements(w http.ResponseWriter, r *http.Request) {
	elements := world.All()

	// Group in declaration order so the drawer's sections follow the curated order
	// of the registry rather than numeric ID.
	var categories []categoryDTO
	seen := map[world.Category]int{}

	for _, e := range elements {
		idx, ok := seen[e.Category]
		if !ok {
			categories = append(categories, categoryDTO{
				ID:   uint8(e.Category),
				Name: world.CategoryName(e.Category),
			})
			idx = len(categories) - 1
			seen[e.Category] = idx
		}
		categories[idx].Elements = append(categories[idx].Elements, toElementDTO(e))
	}

	w.Header().Set("Content-Type", "application/json")
	// The catalogue changes only when the binary does, so it is safe to cache for a
	// while. Without this every client re-fetched a payload with 44 elements and
	// their reactions on every page load.
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(map[string]any{
		"count":      len(elements),
		"categories": categories,
	})
}

// toElementDTO converts a registry element to its wire shape.
func toElementDTO(e *world.Element) elementDTO {
	dto := elementDTO{
		ID:           e.ID,
		Name:         e.Name,
		Symbol:       e.Symbol,
		Atomic:       e.Atomic,
		Category:     world.CategoryName(e.Category),
		CategoryID:   uint8(e.Category),
		Phase:        world.PhaseName(e.Phase),
		Density:      e.Density,
		MeltingPoint: e.MeltingPoint,
		BoilingPoint: e.BoilingPoint,
		Flammability: e.Flammability,
		Conductivity: e.Conductivity,
		Reactivity:   e.Reactivity,
		Colour:       e.Colour,
		Flavour:      e.Flavour,
		// Every registry element is placeable. The legacy materials are not all
		// placeable, which is why this is a field rather than an assumption.
		Placeable: true,
	}

	if e.IgnitionTemp != world.TempNone {
		t := e.IgnitionTemp
		dto.IgnitionTemp = &t
	}

	for _, r := range simulation.ReactionsFor(e.ID) {
		// Report the reaction from this element's point of view, so the drawer reads
		// "reacts with X" rather than making the player work out which side it is on.
		other, product := r.B, r.ProductB
		if r.B == e.ID {
			other, product = r.A, r.ProductA
		}

		rd := reactionDTO{
			With:       world.ElementName(other),
			Produces:   world.ElementName(product),
			Equation:   r.Equation,
			NeedsHeat:  r.MinTemp != world.TempNone,
			Exothermic: r.HeatDelta > 0,
		}
		if r.MinTemp != world.TempNone {
			// Internal temperatures are tenths of a degree; the client shows whole
			// degrees Celsius.
			c := r.MinTemp / 10
			rd.MinTempC = &c
		}
		if r.Catalyst != 0 {
			rd.Catalyst = world.ElementName(r.Catalyst)
		}
		dto.Reactions = append(dto.Reactions, rd)
	}

	return dto
}
