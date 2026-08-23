export default function Footer() {
  return (
    <footer className="footer">
      <div className="container footer__inner">
        <div className="footer__brand">
          <p className="footer__name">WorldWeaver</p>
          <p className="footer__tagline">One world. Many forces.</p>
        </div>

        <nav className="footer__nav" aria-label="Footer">
          <a className="footer__link" href="/play">
            Play
          </a>
          <a className="footer__link" href="#live-worlds">
            Live worlds
          </a>
          <a className="footer__link" href="#how-it-works">
            How it works
          </a>
          <a
            className="footer__link"
            href="https://github.com/pablodz/worldweaver"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
          <a
            className="footer__link"
            href="https://kiro.dev/hackathon"
            target="_blank"
            rel="noopener noreferrer"
          >
            Hackathon
          </a>
        </nav>
      </div>

      <div className="container footer__legal">
        <p>© 2026 WorldWeaver</p>
        <p>Built with Kiro for the Ready, Spec, Ship Hackathon 2026</p>
      </div>
    </footer>
  );
}
