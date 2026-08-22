const diagram = `
  ┌─────────────────────────────────────────────────────────────┐
  │                     WorldWeaver Architecture                  │
  └─────────────────────────────────────────────────────────────┘

  ┌──────────────┐      WebSocket       ┌──────────────────────┐
  │              │◄────────────────────► │   Browser Client     │
  │   Go Server  │      (60 TPS)        │   ├─ WebGL2 Renderer │
  │              │                      │   ├─ Input Manager   │
  │  ┌────────┐  │                      │   └─ Audio Engine    │
  │  │ World  │  │                      └──────────────────────┘
  │  │ State  │  │
  │  │ (auth) │  │      WebSocket       ┌──────────────────────┐
  │  └────────┘  │◄────────────────────► │   Tauri Desktop     │
  │              │                      │   ├─ Native Window   │
  └──────────────┘                      │   └─ Platform APIs   │
                                        └──────────────────────┘
`;

const stack = [
  { label: "Go 1.22+", icon: "🔷" },
  { label: "WebSocket", icon: "🔌" },
  { label: "WebGL2", icon: "🎨" },
  { label: "TypeScript", icon: "📘" },
  { label: "Tauri v2", icon: "🖥️" },
  { label: "Vite", icon: "⚡" },
];

export default function Architecture() {
  return (
    <section className="architecture">
      <div className="architecture__inner">
        <h2 className="architecture__title">Architecture</h2>
        <div className="architecture__diagram">{diagram}</div>
        <div className="architecture__stack">
          {stack.map((s) => (
            <span key={s.label} className="architecture__badge">
              <span>{s.icon}</span>
              {s.label}
            </span>
          ))}
        </div>
        <div className="architecture__kiro">
          ✨ Built with Kiro
        </div>
      </div>
    </section>
  );
}
