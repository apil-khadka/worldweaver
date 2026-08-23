# World Persistence — Tasks

## Phase 1 — Binary Snapshot

- [x] Define binary format constants (magic "WWSN", version 1, header: width/height/seed/tick + 16 reserved bytes)
- [x] Implement writeSnapshot() — writes header + Material + Temperature + Moisture + Lifetime arrays
- [x] Implement Save() — writes to temp file, validates, atomic rename via os.Rename
- [x] Implement Load() — reads file, validates magic/version, checks dimension match
- [x] Handle corrupt/truncated file gracefully — returns error, server logs and generates fresh world
- [x] Integrate load-on-startup in main(): attempt Load, fallback to Generate()
- [x] Implement SavePeriodic() goroutine with configurable interval (default 5 min)
- [x] Wire signal.NotifyContext for SIGINT/SIGTERM in main()
- [x] Final save on shutdown before exit
- [ ] Add CRC32 checksum validation (currently no integrity check beyond format validation)
- [ ] Add version migration path (v1 → v2 if format changes)
- [ ] Compress payload with LZ4 for large worlds (benchmark first)
- [ ] Add save-file integrity CLI tool (`worldweaver verify <path>`)
