# Material System — Design

## Registry Pattern

A package-level `registry` map holds all material definitions indexed by uint8 ID. Registration happens in `init()` functions within each material definition file.

```go
// internal/material/registry.go
var registry [256]*Def  // indexed by material ID, nil = unregistered

func Register(def Def) {
    if registry[def.ID] != nil {
        panic(fmt.Sprintf("material: duplicate ID %d (%s vs %s)", def.ID, registry[def.ID].Name, def.Name))
    }
    registry[def.ID] = &def
}

func Get(id uint8) *Def { return registry[id] }
```

## Def Struct

```go
type Def struct {
    ID          uint8
    Name        string
    Density     uint8   // 0=gas, 1-127=liquid, 128-255=solid
    Flammable   bool
    Corrodible  bool
    SpreadRate  uint8   // cells per tick for liquids
    Lifetime    uint16  // 0 = infinite
    VisualClass string  // "powder", "liquid", "gas", "solid", "fire"
}
```

## Material Constants

```go
const (
    MatEmpty  uint8 = 0
    MatSand   uint8 = 1
    MatWater  uint8 = 2
    MatStone  uint8 = 3
    MatWood   uint8 = 4
    MatFire   uint8 = 5
    MatSmoke  uint8 = 6
    MatSoil   uint8 = 7
    MatPlant  uint8 = 8
    MatSteam  uint8 = 9
    MatEmber  uint8 = 10
)
```

## Registration via init()

Each material file (e.g., `sand.go`) calls `Register()` in its `init()`:

```go
// internal/material/sand.go
func init() {
    Register(Def{
        ID: MatSand, Name: "Sand", Density: 200,
        Flammable: false, VisualClass: "powder",
    })
}
```

## Query Helpers

Convenience functions avoid callers needing to nil-check:

```go
func IsSolid(id uint8) bool     { d := Get(id); return d != nil && d.Density >= 128 }
func IsLiquid(id uint8) bool    { d := Get(id); return d != nil && d.Density > 0 && d.Density < 128 }
func IsGas(id uint8) bool       { d := Get(id); return d != nil && d.Density == 0 }
func IsFlammable(id uint8) bool { d := Get(id); return d != nil && d.Flammable }
```

## Visual Class

The `VisualClass` field is transmitted to clients during initial material list sync. The client uses it to select appropriate rendering shaders (particle vs fluid vs static). The server never uses this field for simulation logic.
