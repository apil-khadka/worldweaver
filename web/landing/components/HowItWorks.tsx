import Icon, { type IconName } from "./Icon";

const steps: { icon: IconName; title: string; body: string }[] = [
  {
    icon: "cursor",
    title: "You ask",
    body: "Pick a tool and a material, then click. Your browser does not change the world — it sends the request over a WebSocket and keeps drawing what it last knew.",
  },
  {
    icon: "server",
    title: "The server decides",
    body: "A Go process holds the only real copy of the world and advances it sixty times a second. It applies your edit, runs the material rules around it, and moves the food chain forward.",
  },
  {
    icon: "globe",
    title: "Everyone sees the same thing",
    body: "The result is broadcast to every player in that world. Because the server is the single source of truth, nobody drifts into their own version of events and nobody can rewrite the terrain locally.",
  },
];

export default function HowItWorks() {
  return (
    <section className="how" id="how-it-works">
      <div className="container container--narrow">
        <p className="section-eyebrow">How it works</p>
        <h2 className="section-title">
          One simulation, many hands on it
        </h2>
        <p className="section-lede">
          Shared worlds fall apart when every player runs their own physics. So
          WorldWeaver does not: the simulation is <em>server-authoritative</em>,
          which is a plain idea with a heavy name.
        </p>

        <ol className="how__steps">
          {steps.map((step, i) => (
            <li key={step.title} className="how-step">
              <span className="how-step__number" aria-hidden="true">
                {i + 1}
              </span>
              <h3 className="how-step__title">
                <Icon name={step.icon} size={18} />
                {step.title}
              </h3>
              <p className="how-step__body">{step.body}</p>
            </li>
          ))}
        </ol>

        <p className="how__note">
          The trade-off is honest: your edits take one network round trip to
          become real. In exchange, the world you are looking at is the world
          everyone else is looking at.
        </p>
      </div>
    </section>
  );
}
