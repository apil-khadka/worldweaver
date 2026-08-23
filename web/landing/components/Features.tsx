import Icon, { type IconName } from "./Icon";

interface Feature {
  icon: IconName;
  title: string;
  desc: string;
}

/**
 * Only features that exist in the shipped simulation. If something is not in
 * the server or the client today, it does not belong on this list.
 */
const features: Feature[] = [
  {
    icon: "server",
    title: "One authoritative world",
    desc: "The Go server advances the simulation at 60 ticks per second and owns every cell. Your client renders and requests; it never invents state. That is what makes two people editing the same ridge work at all.",
  },
  {
    icon: "layers",
    title: "Around twenty materials",
    desc: "Stone, water, sand, lava, plants and the rest interact through cellular rules rather than scripted outcomes. Most of what happens in a session is something nobody planned.",
  },
  {
    icon: "leaf",
    title: "A food chain to keep alive",
    desc: "Grass feeds grazers, grazers feed predators. Starve one link and the chain above it collapses — a shared consequence you have to manage together, not a score you farm alone.",
  },
  {
    icon: "wind",
    title: "Five elemental forces",
    desc: "Rain, Heat, Wind, Growth and Life. Each pushes the world in a different direction, and two players applying opposing forces to the same region is a legitimate way to play.",
  },
  {
    icon: "tools",
    title: "God-mode terraforming",
    desc: "Place, erase, raise and lower, with a material palette to pick from. Coarse enough to reshape a coastline in seconds, precise enough to patch someone else's mistake.",
  },
  {
    icon: "cursor",
    title: "You can see each other",
    desc: "Every connected player's cursor moves across your screen with their nickname attached. You watch intent happen live, which is what turns parallel building into actual collaboration.",
  },
  {
    icon: "target",
    title: "Rotating shared goals",
    desc: "Cooperative objectives cycle as you play, giving a group something to aim at without forcing anyone into a role. Progress is the world's, not any one player's.",
  },
  {
    icon: "display",
    title: "WebGL2 rendering",
    desc: "The client draws the world through WebGL2 in the browser, so joining is a link — no install, no launcher, no account gate between you and the terrain.",
  },
];

export default function Features() {
  return (
    <section className="features" id="what-you-get">
      <div className="container">
        <p className="section-eyebrow">What you get</p>
        <h2 className="section-title">Built so that other people matter</h2>
        <p className="section-lede">
          A sandbox is easy. A sandbox that stays coherent while eight people
          pull it in different directions is the hard part, and it is the part
          WorldWeaver is designed around.
        </p>

        <ul className="features__grid" role="list">
          {features.map((f) => (
            <li key={f.title} className="feature-card">
              <span className="feature-card__icon">
                <Icon name={f.icon} size={22} />
              </span>
              <h3 className="feature-card__title">{f.title}</h3>
              <p className="feature-card__desc">{f.desc}</p>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
