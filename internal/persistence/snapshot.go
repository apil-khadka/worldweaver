// Package persistence handles saving and restoring world snapshots to disk.
//
// # Snapshot Format
//
// A snapshot is a binary file with the following layout:
//
//	[ Header ]
//	  magic:     4 bytes  "WWSN"
//	  version:   2 bytes  uint16, current = 1
//	  width:     4 bytes  int32
//	  height:    4 bytes  int32
//	  seed:      8 bytes  int64
//	  tick:      8 bytes  uint64
//	  reserved: 16 bytes
//
//	[ Material array ]   width*height bytes (uint8 each)
//	[ Temperature array] width*height*2 bytes (int16 LE each)
//	[ Moisture array ]   width*height bytes (uint8 each)
//	[ Lifetime array ]   width*height*2 bytes (uint16 LE each)
//
// This format is intentionally simple: no compression, no fragmentation.
// If the world is large (e.g., 2M cells) a snapshot is ~8 MB — trivially
// affordable on any server with a mounted volume.
package persistence

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/worldweaver/worldweaver/internal/world"
)

const (
	snapshotMagic   = "WWSN"
	snapshotVersion = uint16(1)
	snapshotFile    = "world.snapshot"
)

// Save writes the current world state to <dir>/world.snapshot.
// It writes to a temporary file then renames to avoid partial writes.
func Save(dir string, w *world.World) error {
	tmp := filepath.Join(dir, "world.snapshot.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("snapshot create: %w", err)
	}
	if err := writeSnapshot(f, w); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("snapshot write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("snapshot close: %w", err)
	}
	dest := filepath.Join(dir, snapshotFile)
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("snapshot rename: %w", err)
	}
	return nil
}

// Load restores world state from <dir>/world.snapshot.
// Returns an error if no snapshot exists or the file is incompatible.
func Load(dir string, w *world.World) error {
	path := filepath.Join(dir, snapshotFile)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("snapshot open: %w", err)
	}
	defer f.Close()
	return readSnapshot(f, w)
}

// SavePeriodic starts a background goroutine that saves a snapshot every
// interval.  The returned cancel function stops the goroutine.
func SavePeriodic(dir string, w *world.World, interval time.Duration) (cancel func()) {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if err := Save(dir, w); err != nil {
					// Log but do not crash — persistence errors are non-fatal.
					fmt.Fprintf(os.Stderr, "snapshot save error: %v\n", err)
				}
			}
		}
	}()
	return func() { close(stop) }
}

// ---- internal helpers ----

func writeSnapshot(w io.Writer, world *world.World) error {
	le := binary.LittleEndian

	// Header
	if _, err := io.WriteString(w, snapshotMagic); err != nil {
		return err
	}
	if err := binary.Write(w, le, snapshotVersion); err != nil {
		return err
	}
	if err := binary.Write(w, le, int32(world.Width)); err != nil {
		return err
	}
	if err := binary.Write(w, le, int32(world.Height)); err != nil {
		return err
	}
	if err := binary.Write(w, le, world.Seed); err != nil {
		return err
	}
	if err := binary.Write(w, le, world.Tick); err != nil {
		return err
	}
	// 16 reserved bytes
	if _, err := w.Write(make([]byte, 16)); err != nil {
		return err
	}

	// Arrays
	if _, err := w.Write(world.Material); err != nil {
		return err
	}
	if err := binary.Write(w, le, world.Temperature); err != nil {
		return err
	}
	if _, err := w.Write(world.Moisture); err != nil {
		return err
	}
	if err := binary.Write(w, le, world.Lifetime); err != nil {
		return err
	}
	return nil
}

func readSnapshot(r io.Reader, w *world.World) error {
	le := binary.LittleEndian

	// Header
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != snapshotMagic {
		return fmt.Errorf("invalid snapshot magic: %q", magic)
	}

	var version uint16
	if err := binary.Read(r, le, &version); err != nil {
		return err
	}
	if version != snapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d (want %d)", version, snapshotVersion)
	}

	var width, height int32
	if err := binary.Read(r, le, &width); err != nil {
		return err
	}
	if err := binary.Read(r, le, &height); err != nil {
		return err
	}
	if int(width) != w.Width || int(height) != w.Height {
		return fmt.Errorf("snapshot dimensions %dx%d don't match world %dx%d",
			width, height, w.Width, w.Height)
	}

	if err := binary.Read(r, le, &w.Seed); err != nil {
		return err
	}
	if err := binary.Read(r, le, &w.Tick); err != nil {
		return err
	}
	// skip 16 reserved bytes
	if _, err := io.ReadFull(r, make([]byte, 16)); err != nil {
		return err
	}

	// Arrays
	if _, err := io.ReadFull(r, w.Material); err != nil {
		return err
	}
	if err := binary.Read(r, le, &w.Temperature); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, w.Moisture); err != nil {
		return err
	}
	if err := binary.Read(r, le, &w.Lifetime); err != nil {
		return err
	}

	seedRestoredCreatures(w)
	return nil
}

// seedRestoredCreatures gives every restored creature a food reserve.
//
// Energy and thirst are not part of the snapshot format, so a world loaded from
// disk comes back with both zeroed. The simulation no longer tops up creatures
// found with no reserve — that behaviour made starving creatures immortal — so
// without this pass every creature in a restored world would die on its first
// update and the ecosystem would be wiped out by reloading.
func seedRestoredCreatures(w *world.World) {
	const restoredEnergy = 140
	for i, m := range w.Material {
		if world.IsCreature(m) {
			w.Energy[i] = restoredEnergy
			w.Thirst[i] = 0
		}
	}
}
