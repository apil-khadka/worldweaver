# World Persistence — Design

## Binary Format Layout

```
Offset  Size       Field
0       4 bytes    Magic: "WWSV" (0x57 0x57 0x53 0x56)
4       2 bytes    Format version (uint16 LE, currently 1)
6       2 bytes    World width (uint16 LE)
8       2 bytes    World height (uint16 LE)
10      8 bytes    Seed (uint64 LE)
18      8 bytes    Tick counter (uint64 LE)
26      4 bytes    CRC32 of everything after this field
30      W*H        Material array (uint8[])
30+W*H  W*H*2     Temperature array (int16[] LE)
30+3WH  W*H       Moisture array (uint8[])
30+4WH  W*H*2     Lifetime array (uint16[] LE)
```

Total file size: `30 + W*H*6` bytes. For 1024×512: 30 + 3,145,728 = ~3 MB.

## Save Function

```go
func Save(w *World, path string) error {
    tmp := path + ".tmp"
    f, err := os.Create(tmp)
    if err != nil { return err }
    defer f.Close()

    buf := bufio.NewWriter(f)
    writeHeader(buf, w)
    writeMaterials(buf, w)
    writeTemperature(buf, w)
    writeMoisture(buf, w)
    writeLifetime(buf, w)

    if err := buf.Flush(); err != nil { return err }
    if err := f.Sync(); err != nil { return err }
    f.Close()

    return os.Rename(tmp, path)  // atomic on POSIX
}
```

## Atomic Rename Pattern

1. Write to `world.save.tmp`
2. `fsync` the file descriptor
3. `os.Rename("world.save.tmp", "world.save")` — atomic on POSIX filesystems

If the process crashes before rename, the old `.save` file is untouched. On restart, `.tmp` is ignored.

## Load Function

```go
func Load(path string) (*World, error) {
    data, err := os.ReadFile(path)
    if err != nil { return nil, err }

    if !bytes.Equal(data[:4], []byte("WWSV")) {
        return nil, ErrInvalidMagic
    }
    version := binary.LittleEndian.Uint16(data[4:6])
    if version > CurrentVersion {
        return nil, ErrUnsupportedVersion
    }

    // Validate CRC32
    storedCRC := binary.LittleEndian.Uint32(data[26:30])
    actualCRC := crc32.ChecksumIEEE(data[30:])
    if storedCRC != actualCRC {
        return nil, ErrChecksumMismatch
    }

    // Parse fields and construct world...
}
```

## SavePeriodic Goroutine

A dedicated goroutine runs a ticker at the configured save interval:

```go
func (e *Engine) SavePeriodic(ctx context.Context, interval time.Duration, path string) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            e.saveSnapshot(path)  // final save on shutdown
            return
        case <-ticker.C:
            e.saveSnapshot(path)
        }
    }
}
```

The `saveSnapshot` method copies the world arrays (between ticks, holding no lock beyond the swap) and writes to disk. The simulation is not blocked during the disk write — only the array copy happens in the tick gap.

## Shutdown Hook

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    // ...
    go engine.SavePeriodic(ctx, 5*time.Minute, savePath)
    <-ctx.Done()
    // SavePeriodic does final save when ctx cancelled
}
```
