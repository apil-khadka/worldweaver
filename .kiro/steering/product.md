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
