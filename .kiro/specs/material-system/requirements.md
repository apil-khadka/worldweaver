# Material System — Requirements

## REQ-MAT-001: Unique Material IDs
Each material SHALL have a unique uint8 identifier assigned at compile time. ID 0 is reserved for Empty (air).

**Acceptance:** No two materials share the same ID; static analysis or init-time check confirms uniqueness.

## REQ-MAT-002: Registry Data Structure
All material definitions SHALL be stored in a single global registry accessible via `material.Get(id)` returning a `*Def` or nil.

**Acceptance:** `material.Get(MatSand)` returns a non-nil Def with correct properties.

## REQ-MAT-003: Boolean Property Helpers
The registry SHALL expose helper functions: `IsSolid(id)`, `IsLiquid(id)`, `IsGas(id)`, `IsFlammable(id)`, `Iite Corrodible(id)`.

**Acceptance:** `material.IsSolid(MatStone)` returns true; `material.IsLiquid(MatWater)` returns true.

## REQ-MAT-004: Centralised Definitions
All material properties (density, flammability, colour hint, spread rate) SHALL be defined in a single package (`internal/material`). No simulation code shall hard-code material properties.

**Acceptance:** `grep -r "density" internal/simulation/` returns zero hits.

## REQ-MAT-005: No Duplicate IDs at Startup
The registry SHALL panic during `init()` if a duplicate material ID is registered.

**Acceptance:** Test with intentional duplicate triggers panic with descriptive message.

## REQ-MAT-006: Extensibility
Adding a new material SHALL require only: (1) a new const ID, (2) a new `Register()` call with properties, (3) a simulation handler in the simulation package.

**Acceptance:** Adding "Mud" material requires changes to exactly 2 packages.

## REQ-MAT-007: Protocol Stability
Material IDs SHALL remain stable across server versions. Removed materials keep their ID reserved. A MATERIALS.md changelog tracks additions.

**Acceptance:** MATERIALS.md exists and lists all IDs with version introduced.

## REQ-MAT-008: Visual Class Hint
Each material definition SHALL include a `VisualClass` string (e.g., "powder", "liquid", "gas", "solid") used by the client renderer for shader selection.

**Acceptance:** All registered materials have a non-empty VisualClass field.

## References
- WorldWeaver_Full_Project_Documentation.md § 19 (Material Properties)
- WorldWeaver_Full_Project_Documentation.md § 20 (Material Registry)
