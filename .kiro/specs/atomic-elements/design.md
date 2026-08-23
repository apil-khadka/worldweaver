# Atomic Elements — Design

## 1. Why the current system cannot grow

Material behaviour is expressed procedurally, once per material, in whichever file
that material's simulation lives. The dispatch point is a single switch:

```go
// internal/simulation/cell.go
switch w.Material[i] {
case world.MatSand:  simulateSand(w, x, y)
case world.MatWater: simulateWater(w, x, y)
case world.MatLava:  simulateLava(w, x, y)
// ... one arm per material
}
```

Interactions are then buried inside those handlers. Plasma's effect on five other
materials is a switch inside `exotic.go`:

```go
switch w.GetMaterial(nx, ny) {
case world.MatRock, world.MatSoil, world.MatSand:
    w.SetMaterial(nx, ny, world.MatLava)
case world.MatWater, world.MatIce:
    w.SetMaterial(nx, ny, world.MatVapor)
case world.MatPlant, world.MatOil:
    w.SetMaterial(nx, ny, world.MatFire)
}
```

Three structural consequences follow.

**Interactions are not discoverable.** Nothing enumerates what reacts with what.
Answering "what does water do?" requires reading every file that mentions
`MatWater`. The client cannot show a player what an element does because that
knowledge exists only as control flow.

**Classification is a second source of truth.** `IsFlammable`, `IsLiquid`,
`IsSolid`, `IsGas`, `IsTransient`, `IsEmissive`, `IsHazard` and `Density` are eight
hardcoded switches over the same 23 IDs. A new material silently gets the default
from each one it is not added to — `Density()` returns 50, so every unregistered
material sinks and floats identically.

**The palette is duplicated three ways.** The Go constants, the HTML buttons with
inline hex colours, and `buildPaletteData()` in the WebGL renderer each carry their
own copy of the list, and a fourth and fifth copy exist in the WebGPU and
alternate WebGL2 renderers. They are kept in sync by hand.

### 1.1 What does not need to change

The investigation confirmed generous headroom in the data plane. `Material` is
`uint8`, so the ceiling is 256 IDs and only 23 are used. The WebGL palette texture
is already allocated 256×1 RGBA, so it needs no resize. The wire protocol sends
raw `uint8` cell bytes with no length assumption. **Adding 40 elements requires no
change to the texture, the protocol, or the snapshot layout.** The cost is entirely
in the hardcoded behaviour, which is what this design removes.

## 2. The element registry

An element is a record. Properties that the simulation currently infers from
switches become fields.

```go
type Element struct {
    ID       uint8
    Name     string  // "Magnesium"
    Symbol   string  // "Mg"  — empty for compounds like Thermite
    Atomic   uint8   // 12    — zero for compounds and mixtures
    Category Category
    Phase    Phase   // PhaseGas | PhaseLiquid | PhasePowder | PhaseRigid

    Density      int16 // kg/m³, drives sink/float ordering
    MeltingPoint int16 // °C
    BoilingPoint int16 // °C
    IgnitionTemp int16 // °C, 0 = does not ignite

    Flammability uint8 // 0..255
    Conductivity uint8 // 0..255 thermal
    Reactivity   uint8 // 0..255, scales reaction probability

    MeltsInto  uint8 // element ID above MeltingPoint
    BoilsInto  uint8 // element ID above BoilingPoint
    BurnsInto  uint8 // element ID when ignited

    Colour   [4]uint8 // RGBA, served to the client
    Flavour  string   // one line shown in the drawer
}
```

`Phase` replaces the `IsSolid`/`IsLiquid`/`IsGas` switches and selects the movement
rule. `Density` replaces the `Density()` switch. `MeltsInto`/`BoilsInto`/`BurnsInto`
replace the per-material state-change code in `ice.go`, `lava.go` and `oil.go`.

Physical values are real. They come from the CRC Handbook and the NIST Chemistry
WebBook, not from taste. Tungsten melts at 3,422 °C and therefore survives a lava
flow; gallium melts at 30 °C and therefore melts in a warm room. Behaviour that
falls out of correct numbers costs nothing to tune.

### 2.1 Temperature representation

`Temperature` is already `int16` in tenths of a degree, giving −3,270 °C to
+3,276 °C. That is **not enough** for this element set: tungsten boils at 5,555 °C
and thermite burns near 2,500 °C, which fits, but the plasma path already writes
1,200 and a thermite cascade can exceed the ceiling.

Two options were weighed. Widening to `int32` doubles the temperature array, adding
2 MB on a 2M-cell world. Switching the unit from tenths of a degree to whole
degrees keeps `int16` and buys a −32,768..32,767 °C range, losing a decimal place
that nothing reads.

**Decision: keep `int16`, change the unit to whole degrees.** The decimal was never
surfaced, and the memory budget in `tech.md` is tighter than the precision
requirement. This is a breaking change to the snapshot format and is handled in
§5.

## 3. The reaction table

A reaction is also a record. The pair is unordered — the table is consulted with
both operands — so `Na + H₂O` and `H₂O + Na` are one row.

```go
type Reaction struct {
    A, B     uint8 // reactants, order-insensitive
    ProductA uint8 // what A becomes
    ProductB uint8 // what B becomes

    MinTemp   int16 // °C threshold; -32768 = no requirement
    Catalyst  uint8 // element that must be adjacent; 0 = none
    HeatDelta int16 // °C released (positive) or absorbed (negative)
    Chance    uint16 // 1-in-N per adjacency check

    Equation string // "2Na + 2H₂O → 2NaOH + H₂"  — documentation
}
```

`HeatDelta` is what makes chains work: an exothermic reaction writes heat into the
cells it touches, and a neighbouring pair whose `MinTemp` is now satisfied reacts on
a later tick. Thermite propagation is emergent rather than coded.

`Chance` keeps reactions legible. A reaction that fires on every adjacency check
consumes a reactant mass instantly at 60 ticks per second; a 1-in-N roll spreads it
over a visible duration. Reactivity from the element record scales this, so caesium
does not react at sodium's pace.

### 3.1 Lookup cost

A naive scan of 65 reactions against 4 neighbours per cell is 260 comparisons per
cell per tick, which at 2M cells is far outside the 16 ms budget in `tech.md`.

The table is therefore indexed at startup into a flat `[256][256]` lookup of
reaction indices — 64 KB of `int16`, built once. A reaction check becomes one array
read. Cells whose element has `Reactivity == 0` skip the neighbour walk entirely,
which excludes rock, sand and soil — the overwhelming majority of a typical world.

### 3.2 Reaction set

65 reactions are drawn from real chemistry, selected for being visually legible in
a grid. The famous ones anchor the set:

| Reaction | Equation | Threshold | ΔH |
|---|---|---|---|
| Alkali metal in water | `2Na + 2H₂O → 2NaOH + H₂` | none | +184 kJ/mol |
| Potassium in water | `2K + 2H₂O → 2KOH + H₂` | none | +196 kJ/mol |
| Oxyhydrogen | `2H₂ + O₂ → 2H₂O` | ignition | +572 kJ/mol |
| Thermite | `2Al + Fe₂O₃ → 2Fe + Al₂O₃` | 1,500 °C | +852 kJ/mol |
| Magnesium in CO₂ | `2Mg + CO₂ → 2MgO + C` | 473 °C | +810 kJ/mol |
| Neutralisation | `HCl + NaOH → NaCl + H₂O` | none | +57 kJ/mol |
| Acid on metal | `Zn + 2HCl → ZnCl₂ + H₂` | none | +153 kJ/mol |
| Rusting | `4Fe + 3O₂ → 2Fe₂O₃` | none, slow | +826 kJ/mol |
| Sand to glass | `SiO₂(s) → SiO₂(l)` | 1,713 °C | endothermic |
| Haber process | `N₂ + 3H₂ → 2NH₃` | 450 °C + Fe | +92 kJ/mol |

Magnesium burning in carbon dioxide is the clearest demonstration that the table is
real: it is the counter-intuitive case that a hand-written rule set would never
include, and it means a CO₂ fire blanket makes a magnesium fire worse.

`HeatDelta` is derived from molar enthalpy scaled into a per-cell temperature
delta, not used directly — 852 kJ/mol is not 852 °C. The scale factor is a single
tuning constant.

## 4. Progression

Score is currently `CellsAffected*1 + CreaturesSpawned*5 + WaterCreated*2 +
StabilityContribution*10`, accumulated per cell. Radius 8 covers 197 cells, so one
Rain application scores 591 and one second of holding the button scores 4,728
against a max-level threshold of 10,000.

Three independent defects compound:

1. **Reward scales with area.** O(r²) in the brush radius, so a bigger brush is
   strictly better and the radius cap is the real progression gate.
2. **Thresholds grow ~5× per level over five levels.** Against per-cell scoring
   this is indistinguishable from flat.
3. **Influence never binds.** Rain costs 0.033 influence against 30/second of
   regeneration, so the resource that was meant to pace play is always full.

The replacement awards per *action* with a sub-linear area term, so a wide brush
is a convenience rather than a multiplier:

```
actionScore = base(power) + sqrt(cellsAffected) * areaBonus
```

Thresholds become exponential, `50 * 2.2^(level-2)`, extended from 5 levels to 25
so there is a long tail to climb. Anti-farm rules attach to the scoreboard, which
currently has none:

- **Repetition damping** — an application overlapping the previous one by more
  than 60% scores 10%.
- **Rate damping** — the first 30 actions per minute score fully, the next 30 at
  half, the remainder at a tenth.
- **Movement bonus** — applying further than one radius from the last application
  scores 1.5×, rewarding working across the world rather than into one hole.

Levels 1–5 keep their existing unlocks; 6–25 gate element categories, so the
element set doubles as the progression content.

## 5. Snapshot compatibility

Changing the temperature unit invalidates existing snapshots: a saved 200 means
20.0 °C under the old unit and 200 °C under the new one, and nothing in the file
distinguishes them.

The snapshot header carries a version. Version 1 files are read with a conversion
pass dividing temperature by ten, and written back as version 2. A version 1 file
is therefore migrated on first load rather than rejected, and the shipped
`world.snapshot` remains loadable.

The existing snapshot format also omits `Energy` and `Thirst`, so creature state is
lost across a restart and reconstructed by a seeding pass. That is an existing
defect and is fixed in the same version bump, since the format is changing anyway.

## 6. Sequencing

The registry has to land before reactions, because reactions reference element IDs
and the consistency validation is what makes a 65-row table safe to edit. The
temperature unit change has to land before thermite, because thermite overflows the
old range. The drawer has to land after the HTTP endpoint, because it is built from
the served definitions.

Progression and the extinction floor are independent of all of it and can land in
parallel — the floor is already implemented.

```
Phase 1  registry + validation + HTTP endpoint
Phase 2  temperature unit + snapshot v2 migration
Phase 3  reaction table + indexed lookup + heat cascade
Phase 4  element drawer built from the endpoint
Phase 5  progression curve + anti-farm
```
