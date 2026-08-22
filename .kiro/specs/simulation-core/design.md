# Simulation Core — Design

## Architecture

The simulation engine runs in a dedicated goroutine with a `time.Ticker` at 60 TPS. It:
1. Drains the player-action queue (lock-free swap)
2. Clears per-cell FlagMoved bits
3. Applies queued player actions
4. Iterates cells bottom-to-top, alternating horizontal direction
5. Dispatches per-material behaviour via `simulateCell()`
6. Increments world tick counter
7. Records tick duration in metrics

## Simulation Ordering

Using ordered in-place simulation (not double-buffering) to minimise memory:
- Bottom-to-top ensures falling materials don't get processed twice
- FlagMoved prevents a moved cell from being re-processed at its destination
- Horizontal alternation (even tick → left-to-right, odd → right-to-left) eliminates directional bias

## Material Dispatch

`cell.go` contains a switch on material ID that calls the appropriate simulation function. Each material file (sand.go, water.go, fire.go, etc.) implements its own behaviour.

## Multi-Rate Extension Point

The current implementation runs all systems at 60 Hz. The design allows wrapping system calls in rate dividers:
```go
if w.Tick % 2 == 0 { simulateFire(...) }    // 30 Hz
if w.Tick % 12 == 0 { simulatePlants(...) }  // 5 Hz
```

## Active Chunk Optimisation

Currently all chunks are processed. Future: skip chunks where `Active == false`. A chunk is activated by player interaction, moving material, or neighbour activity.

## Thread Safety

The simulation goroutine is the ONLY writer to world state. Network reads for broadcast happen between ticks or on snapshot copies. Player actions are enqueued via mutex-protected slice swap.
