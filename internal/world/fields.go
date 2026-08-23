package world

// Cell flags — packed bit flags stored in the Flags array.
const (
	FlagMoved   uint8 = 1 << 0 // set when a cell has already been moved this tick
	FlagWet     uint8 = 1 << 1 // soil has been recently wetted
	FlagHot     uint8 = 1 << 2 // cell is above fire-spread temperature
)

// GetMaterial returns the material at (x, y). Returns MatEmpty for OOB.
func (w *World) GetMaterial(x, y int) uint8 {
	i := w.Index(x, y)
	if i < 0 {
		return MatEmpty
	}
	return w.Material[i]
}

// SetMaterial sets the material at (x, y) and marks the chunk dirty.
func (w *World) SetMaterial(x, y int, m uint8) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	w.Material[i] = m
	w.MarkDirty(x, y)
}

// Swap exchanges the material (and optional field data) between two cells.
func (w *World) Swap(x1, y1, x2, y2 int) {
	i := w.Index(x1, y1)
	j := w.Index(x2, y2)
	if i < 0 || j < 0 {
		return
	}
	w.Material[i], w.Material[j] = w.Material[j], w.Material[i]
	w.Moisture[i], w.Moisture[j] = w.Moisture[j], w.Moisture[i]
	w.Temperature[i], w.Temperature[j] = w.Temperature[j], w.Temperature[i]
	w.Lifetime[i], w.Lifetime[j] = w.Lifetime[j], w.Lifetime[i]
	// Creature state must travel with the creature, or moving would reset a
	// creature's reserves and it would never starve or grow thirsty.
	w.Energy[i], w.Energy[j] = w.Energy[j], w.Energy[i]
	w.Thirst[i], w.Thirst[j] = w.Thirst[j], w.Thirst[i]
	w.MarkDirty(x1, y1)
	w.MarkDirty(x2, y2)
}

// GetTemperature returns the temperature at (x, y) in tenths of a degree.
func (w *World) GetTemperature(x, y int) int16 {
	i := w.Index(x, y)
	if i < 0 {
		return 0
	}
	return w.Temperature[i]
}

// SetTemperature sets the temperature at (x, y).
func (w *World) SetTemperature(x, y int, t int16) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	w.Temperature[i] = t
}

// GetMoisture returns the moisture at (x, y).
func (w *World) GetMoisture(x, y int) uint8 {
	i := w.Index(x, y)
	if i < 0 {
		return 0
	}
	return w.Moisture[i]
}

// SetMoisture sets the moisture at (x, y).
func (w *World) SetMoisture(x, y int, v uint8) {
	i := w.Index(x, y)
	if i < 0 {
		return
	}
	w.Moisture[i] = v
}

// ClearMoveFlags resets FlagMoved on every cell. Called at the start of each tick.
func (w *World) ClearMoveFlags() {
	for i := range w.Flags {
		w.Flags[i] &^= FlagMoved
	}
}
