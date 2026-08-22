export default function Footer() {
  return (
    <footer className="footer">
      <nav className="footer__links">
        <a
          href="https://github.com/pablodz/worldweaver"
          className="footer__link"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub
        </a>
        <a href="/play" className="footer__link">
          Play
        </a>
        <a
          href="https://kiro.dev/hackathon"
          className="footer__link"
          target="_blank"
          rel="noopener noreferrer"
        >
          Hackathon
        </a>
      </nav>
      <p className="footer__copyright">
        © 2026 WorldWeaver. All rights reserved.
      </p>
      <p className="footer__hackathon">
        Built for Ready, Spec, Ship Hackathon 2026
      </p>
    </footer>
  );
}
