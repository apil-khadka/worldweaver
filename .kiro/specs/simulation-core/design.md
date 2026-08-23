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

## Multi-Rate Scheduling (Implemented)

The engine runs different subsystems at their natural frequencies via tick modulo:
```go
Materials:   60 Hz (every tick)
Fire:        30 Hz (tick % 2 == 0)
Plants:       5 Hz (tick % 12 == 0)
Creatures:   10 Hz (tick % 6 == 0)
Weather:      2 Hz (tick % 30 == 0)
```

## Active Chunk Optimisation (Implemented)

Chunks have a `Sleeping` flag. The tick loop skips sleeping chunks entirely. A chunk is woken by:
- Player power application (EnqueueAction wakes target chunk + radius neighbors)
- Material movement crossing chunk boundary (ChangedThisTick → WakeNeighbors)
- Idle tick counter reaching sleep threshold (UpdateSleepStates)

This is the core performance optimization — an idle world uses near-zero CPU.

## Thread Safety

The simulation goroutine is the ONLY writer to world state. Network reads for broadcast happen between ticks or on snapshot copies. Player actions are enqueued via mutex-protected slice swap.
