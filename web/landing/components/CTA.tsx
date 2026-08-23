import Icon from "./Icon";

export default function CTA() {
  return (
    <section className="cta">
      <div className="container container--narrow cta__inner">
        <h2 className="cta__title">Bring someone with you</h2>
        <p className="cta__desc">
          WorldWeaver is more interesting with a second pair of hands in it. Pick
          a world, send the link to whoever you want in there, and start
          arguing about where the river should go.
        </p>
        <div className="cta__actions">
          <a className="button button--primary button--lg" href="/play">
            Enter a world
            <Icon name="arrow" size={18} />
          </a>
          <a className="button button--ghost button--lg" href="#live-worlds">
            Browse live worlds
          </a>
        </div>
        <p className="cta__note">No install and no account. It is a link.</p>
      </div>
    </section>
  );
}
