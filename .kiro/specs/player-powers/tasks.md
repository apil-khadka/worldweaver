# Player Powers — Tasks

## Phase 1 — Core Powers

- [x] Define PowerID constants (Rain=0, Heat=1, Wind=2, Growth=3, Life=4)
- [x] Create PlayerAction struct with PlayerID, Power, X, Y, Radius fields
- [x] Create PlayerState struct with ID, Influence, MaxInfluence, Level, Score
- [x] Implement power definitions table (radius, cost, effect per power)
- [x] Implement applyAction() with circular radius iteration
- [x] Implement Rain effect: spawn water in empty cells, increase moisture within radius
- [x] Implement Heat effect: increase temperature, ignite flammable materials
- [x] Implement Wind effect: apply horizontal displacement to materials
- [x] Implement Growth effect: convert Soil→Plant when moisture conditions met
- [x] Implement Life power: spawn herbivore/predator creatures when biome conditions met (level-gated to Lv.4)
- [x] Implement influence budget consumption per power activation
- [x] Implement influence regeneration when no power active (+10/sec, broadcast every 1s)
- [x] Clamp influence to [0, maxInfluence]
- [x] Add server-side validation in read pump (power ID, bounds, level requirement, budget)
- [x] Send ERROR message on invalid power request
- [x] Wire action queue drain into engine tick (before material sim)
- [x] Broadcast PLAYER_STATE (influence, level, score) to all clients
- [x] Add power cooldown via influence cost mechanism
- [x] Client-side prediction: local material cache mutation before server confirmation
- [x] Visual feedback: power radius indicator, application flash, glow effects
- [x] Audio feedback: procedural Web Audio sounds per power type
- [ ] Balance pass: tune radius/cost values based on playtest feedback

## Phase 2 — Expanded Powers & Progression

- [x] Implement 5-level player progression (XP from actions, unlocks at thresholds)
- [x] Level-gate Life power to Level 4
- [ ] Add power combos (Rain + Heat = steam/fog, Wind + Fire = firestorm spread)
- [ ] Implement power leveling (repeated use unlocks stronger variants)
- [ ] Add visual particle trails showing power influence area
