// Package materials owns the material registry — a data-driven catalogue of
// every material that can exist in the world.
//
// # Why a registry?
//
// The previous architecture used switch statements scattered across simulation
// files.  As the material count grows (rock, soil, sand, water, plant, fire,
// vapor, smoke, lava, ice, oil, ash…) that approach becomes brittle.
//
// A registry provides:
//   - one authoritative source of material properties;
//   - easy iteration over all materials in tests and benchmarks;
//   - clean extension: adding a material is adding a registry entry;
//   - protocol stability: material IDs are fixed here.
//
// Behaviour that is too complex for a struct field is still implemented by
// dedicated system files (fire.go, liquids.go, etc.) which query the registry.
package materials

// ID is the canonical uint8 identifier stored in world.Material[].
// Values match exactly what the protocol transmits to clients.
// Never reorder or reassign existing values — doing so is a breaking change
// that requires incrementing ProtocolVersion.
type ID = uint8

const (
	Empty ID = 0
	Rock  ID = 1
	Soil  ID = 2
	Sand  ID = 3
	Water ID = 4
	Plant ID = 5
	Fire  ID = 6
	Vapor ID = 7
	Smoke ID = 8
	Lava  ID = 9
	Ice   ID = 10
	Ash   ID = 11
	Oil   ID = 12
	Ember ID = 13
	Cloud ID = 16
)

// Def describes the physical and simulation properties of a material.
type Def struct {
	ID          ID
	Name        string
	Density     uint8  // relative density; higher sinks below lower
	Movable     bool   // can this material be displaced?
	Flammable   bool   // can fire spread to this cell?
	Liquid      bool   // flows laterally
	Gas         bool   // rises upward
	Transient   bool   // has a finite lifetime (fire, vapor, smoke)
	Conductive  bool   // conducts heat to neighbours
	Permeable   bool   // water can raise moisture through this material
	VisualClass string // hint to renderer for procedural texture selection
}

// registry is the global material table, indexed by material ID.
var registry [256]Def

// R exposes read-only access to the registry.
var R = &registryReader{}

type registryReader struct{}

// Get returns the Def for the given material ID.
// Returns the Empty def for unregistered IDs.
func (*registryReader) Get(id ID) Def { return registry[id] }

// IsSolid returns true for materials that block downward movement.
func (*registryReader) IsSolid(id ID) bool {
	return !registry[id].Movable && !registry[id].Liquid && !registry[id].Gas
}

// IsFlammable returns true for materials fire can spread to.
func (*registryReader) IsFlammable(id ID) bool { return registry[id].Flammable }

// IsTransient returns true for materials with a finite lifetime.
func (*registryReader) IsTransient(id ID) bool { return registry[id].Transient }

// Name returns the material's display name.
func (*registryReader) Name(id ID) string { return registry[id].Name }

func init() {
	defs := []Def{
		{ID: Empty, Name: "empty",  Density: 0,   Movable: true,  Liquid: false, Gas: true,  VisualClass: "empty"},
		{ID: Rock,  Name: "rock",   Density: 200, Movable: false, Flammable: false, VisualClass: "rock"},
		{ID: Soil,  Name: "soil",   Density: 150, Movable: false, Permeable: true, Conductive: true, VisualClass: "soil"},
		{ID: Sand,  Name: "sand",   Density: 120, Movable: true,  VisualClass: "sand"},
		{ID: Water, Name: "water",  Density: 80,  Movable: true,  Liquid: true,  Conductive: true, VisualClass: "water"},
		{ID: Plant, Name: "plant",  Density: 40,  Movable: false, Flammable: true, VisualClass: "plant"},
		{ID: Fire,  Name: "fire",   Density: 0,   Movable: false, Transient: true, Conductive: true, VisualClass: "fire"},
		{ID: Vapor, Name: "vapor",  Density: 0,   Movable: true,  Gas: true,  Transient: true, VisualClass: "vapor"},
		{ID: Smoke, Name: "smoke",  Density: 0,   Movable: true,  Gas: true,  Transient: true, VisualClass: "smoke"},
		{ID: Lava,  Name: "lava",   Density: 200, Movable: true,  Liquid: true, Flammable: false, Conductive: true, VisualClass: "lava"},
		{ID: Ice,   Name: "ice",    Density: 90,  Movable: false, VisualClass: "ice"},
		{ID: Ash,   Name: "ash",    Density: 10,  Movable: true,  VisualClass: "ash"},
		{ID: Oil,   Name: "oil",    Density: 60,  Movable: true,  Liquid: true, Flammable: true, VisualClass: "oil"},
		{ID: Ember, Name: "ember",  Density: 5,   Movable: true,  Transient: true, Flammable: false, VisualClass: "ember"},
		{ID: Cloud, Name: "cloud",  Density: 0,   Movable: true,  Gas: true, VisualClass: "cloud"},
	}
	for _, d := range defs {
		registry[d.ID] = d
	}
}
