# Player Powers — Design

## Action Queue

Player power activations arrive from the network layer as `PlayerAction` structs enqueued into the simulation's action queue:

```go
type PlayerAction struct {
    PlayerID  uint32
    Power     PowerID    // Rain=1, Heat=2, Wind=3, Growth=4
    X, Y      int16      // center of effect
    Tick      uint64     // client-reported tick (for lag detection)
}
```

The simulation drains the action queue at the start of each tick (before material processing).

## Validation in Network Goroutine

Pre-validation happens in the client read pump before enqueuing:
1. **Power ID valid:** Must be 1–4
2. **Coordinates in bounds:** 0 ≤ X < Width, 0 ≤ Y < Height
3. **Budget check:** Player's current influence ≥ cost of one tick of the power
4. **Rate limit:** Max one POWER_USE per power per tick per player

Invalid actions are rejected with an ERROR message and never reach the simulation queue.

## Influence Budget

```go
type PlayerState struct {
    ID        uint32
    Influence uint16   // current budget, max 1000
    Active    PowerID  // 0 = none
    CursorX   int16
    CursorY   int16
}
```

- **Cost per tick:** Rain=2, Heat=3, Wind=2, Growth=4
- **Regeneration:** +10 per second (~0.167/tick), only when Active==0
- Budget clamped to [0, 1000]

## Circular Influence Application

When a power is applied at (cx, cy), cells within the power's radius are affected:

```go
func applyPower(w *World, ps *PlayerState, action PlayerAction) {
    def := powerDefs[action.Power]
    for dy := -def.Radius; dy <= def.Radius; dy++ {
        for dx := -def.Radius; dx <= def.Radius; dx++ {
            if dx*dx + dy*dy > def.Radius*def.Radius { continue }
            idx := w.Index(int(action.X)+dx, int(action.Y)+dy)
            if idx == -1 { continue }
            applyEffect(w, idx, def)
        }
    }
    ps.Influence -= def.CostPerTick
}
```

## Power Definitions

| Power | Radius | CostPerTick | Effect |
|-------|--------|-------------|--------|
| Rain | 5 | 2 | moisture += 20, temperature -= 5 |
| Heat | 4 | 3 | temperature += 40 |
| Wind | 6 | 2 | applies horizontal displacement flag |
| Growth | 3 | 4 | if cell is Soil and moisture > 50, becomes Plant |

## Effect Application Order

Powers are applied BEFORE material simulation in each tick. This means:
1. Rain adds moisture → water simulation can use it to spread
2. Heat adds temperature → fire can ignite adjacent flammable cells
3. Wind sets displacement flags → falling simulation checks flag for horizontal movement
4. Growth converts cells → plant growth simulation extends from new cells

## Thread Safety

PlayerState is owned by the simulation goroutine. The network layer reads a snapshot copy for PLAYER_STATE broadcasts. Budget mutations happen only within the simulation tick.
