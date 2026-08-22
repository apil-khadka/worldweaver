# WorldWeaver Design System

## Philosophy

The simulation is the visual hero. The UI supports it, never overpowers it.

WorldWeaver should feel like an instrument for observing and influencing a living world — not a generic SaaS dashboard.

## Design Tokens

All visual primitives are CSS custom properties in `web/src/design/tokens/tokens.css`.

### Colours (Semantic)
| Token | Use |
|-------|-----|
| `--ww-bg` | Main background |
| `--ww-surface` | Panel/card backgrounds |
| `--ww-text` | Primary text |
| `--ww-text-secondary` | Secondary labels |
| `--ww-text-muted` | Disabled/tertiary text |
| `--ww-accent` | Interactive elements |
| `--ww-success` | Good state (high stability) |
| `--ww-warning` | Caution state |
| `--ww-danger` | Bad state (low stability) |

### Power Colours
| Token | Power |
|-------|-------|
| `--ww-power-rain` | Rain (#63b3ed) |
| `--ww-power-heat` | Heat (#f6ad55) |
| `--ww-power-wind` | Wind (#a0aec0) |
| `--ww-power-growth` | Growth (#68d391) |

### Spacing Scale (4px base)
`--ww-space-1` (4px) through `--ww-space-12` (48px)

### Typography
- Sans: Inter / Segoe UI / system-ui
- Mono: JetBrains Mono / Fira Code (metrics display)
- Sizes: xs(11) sm(12) md(13) base(14) lg(16)

### Radius
sm(4) md(6) lg(10) — consistent across all components

### Motion
- fast: 80ms (hover states)
- normal: 160ms (transitions)
- Respects `prefers-reduced-motion`

## Visual Hierarchy

1. **World canvas** (largest, always visible)
2. **Power controls** (immediately accessible)
3. **World health/stability** (status awareness)
4. **Connection state** (critical system info)
5. **Diagnostics** (toggleable, not always visible)

## Component Conventions

- All components use `ww-` CSS class prefix
- BEM-like naming: `.ww-btn--primary`, `.ww-panel__header`
- Material colours live in `render/` — NOT in the design system
- Components consume tokens; they never define raw colour values

## Responsive Strategy

| Breakpoint | Layout |
|------------|--------|
| < 640px | Mobile: full-width canvas, bottom power bar |
| 640–1024px | Tablet: side panel option |
| > 1024px | Desktop: top power bar, footer metrics |

## Accessibility

- Semantic HTML (`<button>`, `<input type="range">`)
- Visible focus states on all interactive elements
- Colour never the only signal (combined with icon/text)
- Touch targets ≥ 44px on mobile
