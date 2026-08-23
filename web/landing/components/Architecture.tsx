const diagram = `   ┌──────────────────────────────┐
   │        Go world server       │
   │                              │
   │   authoritative world state  │
   │   simulation loop @ 60 TPS   │
   │   materials · food chain     │
   └───────────┬──────────────────┘
               │  WebSocket
       ┌───────┴────────┬────────────────┐
       │                │                │
┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐
│   Browser   │  │   Browser   │  │    Tauri    │
│   WebGL2    │  │   WebGL2    │  │   desktop   │
│  player one │  │ player two  │  │   shell     │
└─────────────┘  └─────────────┘  └─────────────┘`;

/** Short label for the diagram, read in place of the ASCII art. */
const diagramLabel =
  "Architecture diagram: a single Go world server holds the authoritative " +
  "state and runs the simulation loop, and every client connects to it over " +
  "WebSocket.";

const layers = [
  {
    name: "Go world server",
    detail:
      "Holds the only authoritative copy of the world, steps the simulation 60 times a second, and resolves every player's edits in one place.",
  },
  {
    name: "WebSocket transport",
    detail:
      "Carries player intent up to the server and world updates back down to each connected client.",
  },
  {
    name: "Browser client",
    detail:
      "Renders the world with WebGL2, draws other players' cursors, and sends tool and force requests. It keeps no authority of its own.",
  },
  {
    name: "Tauri desktop shell",
    detail:
      "Wraps the same client in a native window for players who would rather not live in a browser tab.",
  },
];

const stack = ["Go", "WebSocket", "WebGL2", "TypeScript", "React", "Vite", "Tauri"];

export default function Architecture() {
  return (
    <section className="architecture">
      <div className="container container--narrow">
        <p className="section-eyebrow">Under the hood</p>
        <h2 className="section-title">Thin clients, one source of truth</h2>

        <figure className="architecture__figure">
          {/* The ASCII art is a picture made of text: labelled as an image so
              assistive tech reads the label instead of the box-drawing
              characters. The list below is the real text alternative. */}
          <pre className="architecture__diagram" role="img" aria-label={diagramLabel}>
            {diagram}
          </pre>
          <figcaption className="architecture__caption">
            Every client talks to the same server process. There is no
            peer-to-peer path and no client-side simulation to disagree with.
          </figcaption>
        </figure>

        <dl className="architecture__layers">
          {layers.map((layer) => (
            <div key={layer.name} className="architecture__layer">
              <dt className="architecture__layer-name">{layer.name}</dt>
              <dd className="architecture__layer-detail">{layer.detail}</dd>
            </div>
          ))}
        </dl>

        <h3 className="architecture__stack-title">Stack</h3>
        <ul className="architecture__stack" role="list">
          {stack.map((item) => (
            <li key={item} className="architecture__badge">
              {item}
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
