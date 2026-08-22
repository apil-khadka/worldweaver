package simulation

// TickOnce executes a single simulation tick synchronously.
// Used by benchmarks and deterministic tests that need step-by-step execution.
// Normal production operation uses the async fixed-timestep loop in Engine.Start().
func (e *Engine) TickOnce() {
	e.tick()
}
