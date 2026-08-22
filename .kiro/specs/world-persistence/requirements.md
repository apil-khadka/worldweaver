# World Persistence — Requirements

## REQ-PER-001: Versioned Binary Snapshot
The persistence layer SHALL save world state as a binary file with a versioned header. The format must be forward-compatible: older versions can be detected and migrated.

**Acceptance:** Save file starts with magic bytes "WWSV" followed by a uint16 version number.

## REQ-PER-002: Atomic Write
Saving world state SHALL be atomic — either the full file is written successfully or the previous save is preserved. A crash mid-write must not corrupt the save.

**Acceptance:** Kill -9 during save; previous save file intact and loadable on restart.

## REQ-PER-003: Restore on Startup
On startup, if a valid save file exists at the configured path, the engine SHALL load it and resume from the saved tick count.

**Acceptance:** Server saves at tick 1000; restart; server resumes with tick counter = 1000 and matching world state.

## REQ-PER-004: Corrupt File Handling
If the save file fails validation (wrong magic, truncated, checksum mismatch), the engine SHALL log a warning and start a fresh world instead of crashing.

**Acceptance:** Truncated save file at 50% size → server logs error, creates fresh world.

## REQ-PER-005: Periodic Save
The server SHALL automatically save world state at a configurable interval (default: every 300 seconds). Saves must not block the simulation for more than one tick.

**Acceptance:** Save occurs at 5-minute intervals; tick duration does not spike above 20ms during save.

## REQ-PER-006: Graceful Shutdown Save
On SIGINT/SIGTERM, the server SHALL save the current world state before exiting.

**Acceptance:** `kill <pid>` → save file updated with latest tick; subsequent restart loads it.

## References
- WorldWeaver_Full_Project_Documentation.md § 33 (Persistence Strategy)
- WorldWeaver_Full_Project_Documentation.md § 34 (Binary Format)
- WorldWeaver_Full_Project_Documentation.md § 35 (Crash Safety)
