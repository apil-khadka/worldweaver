export default function Hero() {
  return (
    <section className="hero">
      <div className="hero__particles">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="hero__particle" />
        ))}
      </div>
      <div className="hero__content">
        <h1 className="hero__title">WorldWeaver</h1>
        <p className="hero__subtitle">One World. Many Forces.</p>
        <p className="hero__description">
          A multiplayer emergent world where 15 materials interact through scientifically-inspired
          physics. Shape the landscape with rain, heat, wind, growth, and life — watch ecosystems
          evolve in real-time alongside other players.
        </p>
        <a href="/play" className="hero__cta">
          Play Now
        </a>
      </div>
    </section>
  );
}
