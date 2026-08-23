import Icon, { type IconName } from "./Icon";

/** Decorative drifting motes. Count only affects visual density. */
const PARTICLE_COUNT = 8;

const stats: { icon: IconName; value: string; label: string }[] = [
  { icon: "bolt", value: "60", label: "server ticks per second" },
  { icon: "layers", value: "~20", label: "material types" },
  { icon: "wind", value: "5", label: "elemental forces" },
  { icon: "users", value: "8", label: "people per world" },
];

export default function Hero() {
  return (
    <section className="hero">
      {/* Purely decorative: hidden from assistive tech, disabled under
          prefers-reduced-motion. */}
      <div className="hero__particles" aria-hidden="true">
        {Array.from({ length: PARTICLE_COUNT }).map((_, i) => (
          <div key={i} className="hero__particle" />
        ))}
      </div>

      <div className="hero__content">
        <p className="hero__eyebrow">One world. Many forces.</p>

        <h1 className="hero__title">
          Shape a world
          <span className="hero__title-accent">nobody owns alone</span>
        </h1>

        <p className="hero__description">
          WorldWeaver drops up to eight people into the same living terrain at
          the same moment. Raise a ridge while someone else floods the valley
          behind it. Seed grass and wait for the grazers to find it. There is no
          private copy to retreat to — one simulation, running on one server,
          and whatever the group leaves behind is the world.
        </p>

        <div className="hero__actions">
          <a className="button button--primary button--lg" href="/play">
            Enter a world
            <Icon name="arrow" size={18} />
          </a>
          <a className="button button--ghost button--lg" href="#live-worlds">
            See who is playing
          </a>
        </div>

        <dl className="hero__stats">
          {stats.map((stat) => (
            <div key={stat.label} className="hero__stat">
              <dt className="hero__stat-label">
                <Icon name={stat.icon} size={16} />
                {stat.label}
              </dt>
              <dd className="hero__stat-value">{stat.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
