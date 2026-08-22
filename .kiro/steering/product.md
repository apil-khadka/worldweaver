# WorldWeaver — Product Steering

## Vision
Create a persistent shared world whose physics and environmental systems run entirely on a central Go server, allowing many users to influence one emergent simulation from any modern browser.

## Hackathon Context
- Competition: Ready, Spec, Ship Hackathon 2026 (Sponsor: Kiro)
- Deadline: August 23, 2026 23:59 UTC
- Judging: App Quality (40), Kiro Usage (20), Documentation (20), Innovation (15), Presentation (5)

## Product Thesis
> Can multiplayer become meaningful in a god-style sandbox when players control limited environmental forces whose consequences interact inside the same persistent emergent world?

## Core Experience
- Players manipulate environmental forces (Rain, Heat, Wind, Growth) — not individual cells
- One shared persistent world visible to all connected clients
- World Stability as collaborative objective
- Influence economy creates constraints and cooperation

## Non-Goals
- NPC civilizations, kingdoms, crafting, inventory, quests
- Account system, OAuth, payments
- Complex database backend, Kubernetes
- Dozens of disconnected materials without interactions

## Product Principles
1. The server owns the universe
2. Players manipulate forces, not cells
3. Emergence > feature count
4. Playable from a thin browser client
5. Performance claims must be measured
6. Multiplayer must change the experience
7. Kiro participates in the engineering workflow
8. A complete deep system beats an incomplete giant game

## Steering Rule
> Never turn WorldWeaver into a traditional content-heavy game. Prefer deeper simulation and networking quality over additional surface-level features.

## Expanded Vision (Phase 2)

### Ecosystem Simulation
Creatures emerge from the physics layer — herbivores graze plant material, predators hunt herbivores. Population dynamics create feedback loops: overgrazed biomes lose moisture retention, triggering desertification unless players intervene with Rain/Growth powers. The Life power lets players seed creatures directly.

### Weather & Erosion
Weather is emergent from the moisture/temperature simulation: clouds form when moisture exceeds thresholds at altitude, rain events redistribute water, lightning ignites dry material. Water flow over time erodes terrain, carving rivers and depositing sediment — the world visibly ages.

### 2.5D Rendering
The client renders depth layers to convey elevation, weather clouds above terrain, and underground water flow. This is purely a presentation concern — the server simulation remains 2D grid-based with height as a cell property.

### World Stability Scoring
A real-time "World Health" metric visible to all players: biodiversity index, temperature variance, moisture balance. This provides a collaborative objective without explicit quests — players naturally cooperate to stabilize ecosystems.

### Multi-World Architecture
The server hosts multiple independent worlds. Players choose a world on connect. Each world runs its own simulation loop and hub. This enables themed worlds (volcanic, arctic, lush) and load distribution across cores.
