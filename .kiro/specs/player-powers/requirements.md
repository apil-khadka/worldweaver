# Player Powers — Requirements

## REQ-POW-001: Four Powers
The game SHALL provide exactly four player powers: Rain, Heat, Wind, and Growth. Each power applies a distinct environmental effect to the world.

**Acceptance:** All four powers are selectable and produce visible, differentiated effects on world state.

## REQ-POW-002: Influence Economy
Each player SHALL have an influence budget (uint16, max 1000). Using a power costs influence per tick of activation. Budget depletes during use and regenerates when idle.

**Acceptance:** Continuous power use for >16 seconds drains budget to zero; power effect stops.

## REQ-POW-003: Server-Side Validation
All power activations SHALL be validated server-side. The server checks: valid power ID, sufficient influence budget, coordinates within world bounds, cooldown elapsed.

**Acceptance:** Client sending POWER_USE with budget=0 receives error; world state unchanged.

## REQ-POW-004: Radius and Intensity Limits
Each power SHALL have a defined radius (cells) and intensity. Rain: radius 5, adds moisture. Heat: radius 4, adds temperature. Wind: radius 6, applies horizontal force. Growth: radius 3, triggers plant growth in soil.

**Acceptance:** Rain at (100,100) increases moisture for cells within 5-cell radius; cells at distance 6+ unaffected.

## REQ-POW-005: Regeneration
Influence budget SHALL regenerate at 10 units per second (0.167 per tick at 60 TPS) when no power is active.

**Acceptance:** After full depletion, budget reaches 1000 in ~100 seconds of inactivity.

## REQ-POW-006: Simultaneous Players
Multiple players SHALL be able to use powers concurrently. Effects stack additively (two Rain powers on the same cell sum their moisture contributions).

**Acceptance:** Two players using Rain on overlapping areas produce double moisture increase vs single player.

## REQ-POW-007: Visible Effect
Power activation SHALL produce a visible change detectable within the same tick. Environmental fields modified by powers feed into material simulation on the next tick.

**Acceptance:** Rain power on a fire cell: fire extinguished within 2 ticks of continuous rain.

## References
- WorldWeaver_Full_Project_Documentation.md § 30 (Player Powers)
- WorldWeaver_Full_Project_Documentation.md § 31 (Influence Economy)
- WorldWeaver_Full_Project_Documentation.md § 32 (Power Effects)
