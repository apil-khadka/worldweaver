# World Persistence — Tasks

- [x] Define binary format constants (magic bytes, version, header size)
- [x] Implement writeHeader() — magic, version, dimensions, seed, tick, CRC placeholder
- [x] Implement Save() — writes all arrays in order (material, temp, moisture, lifetime)
- [x] Compute and write CRC32 over payload after arrays are written
- [x] Implement atomic write via tmp file + os.Rename
- [x] Implement Load() — reads file, validates magic and version
- [x] Validate CRC32 on load; return ErrChecksumMismatch on failure
- [x] Reconstruct World from loaded arrays (dimensions, seed, tick, all fields)
- [x] Handle corrupt/truncated file gracefully — log error, return nil world
- [x] Integrate load-on-startup in main(): attempt Load, fallback to NewWorld
- [x] Implement SavePeriodic goroutine with configurable interval
- [x] Wire signal.NotifyContext for SIGINT/SIGTERM
- [x] Final save on context cancellation before exit
- [ ] Add version migration path (v1 → v2 if format changes)
- [ ] Compress payload with LZ4 for large worlds (benchmark first)
- [ ] Add save-file integrity CLI tool (`worldweaver verify <path>`)
