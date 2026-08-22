const features = [
  {
    icon: "🌊",
    title: "Emergent Physics",
    desc: "15 materials with scientifically-inspired interactions — water flows, lava solidifies, gases diffuse, and plants grow through cellular automata rules.",
  },
  {
    icon: "🧬",
    title: "Living Ecosystem",
    desc: "Predator-prey dynamics powered by Lotka-Volterra cellular automata. Populations rise and fall based on resource availability and environmental pressure.",
  },
  {
    icon: "🌍",
    title: "Multiplayer",
    desc: "Real-time shared world with cursor presence. Every player's actions affect the same simulation — collaborate or compete to shape the world.",
  },
  {
    icon: "🎮",
    title: "5 Forces",
    desc: "Rain, Heat, Wind, Growth, Life — each force has unique effects on the world's materials. Combine them for emergent chain reactions.",
  },
  {
    icon: "🏔️",
    title: "2.5D Isometric",
    desc: "WebGL2-powered rendering with depth and lighting. Watch your world come alive with dynamic shadows and smooth particle effects.",
  },
  {
    icon: "📊",
    title: "Real-time Metrics",
    desc: "TPS, latency, stability — everything measured. See exactly how the simulation performs with live telemetry overlays.",
  },
];

export default function Features() {
  return (
    <section className="features">
      <h2 className="features__title">A World That Lives</h2>
      <div className="features__grid">
        {features.map((f) => (
          <div key={f.title} className="feature-card">
            <div className="feature-card__icon">{f.icon}</div>
            <h3 className="feature-card__title">{f.title}</h3>
            <p className="feature-card__desc">{f.desc}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
