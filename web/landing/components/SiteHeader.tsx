const navLinks = [
  { href: "#live-worlds", label: "Live worlds" },
  { href: "#how-it-works", label: "How it works" },
  { href: "#what-you-get", label: "What you get" },
];

export default function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header__inner">
        <a className="site-header__brand" href="#main">
          <span className="site-header__mark" aria-hidden="true" />
          WorldWeaver
        </a>

        <nav className="site-header__nav" aria-label="Sections">
          {navLinks.map((link) => (
            <a key={link.href} className="site-header__link" href={link.href}>
              {link.label}
            </a>
          ))}
        </nav>

        <a className="button button--primary button--sm" href="/play">
          Enter a world
        </a>
      </div>
    </header>
  );
}
