import SiteHeader from "./components/SiteHeader";
import Hero from "./components/Hero";
import LiveWorlds from "./components/LiveWorlds";
import HowItWorks from "./components/HowItWorks";
import Features from "./components/Features";
import Architecture from "./components/Architecture";
import CTA from "./components/CTA";
import Footer from "./components/Footer";

export default function App() {
  return (
    <div className="landing">
      <a className="skip-link" href="#main">
        Skip to main content
      </a>
      <SiteHeader />
      <main id="main" className="landing__main">
        <Hero />
        <LiveWorlds />
        <HowItWorks />
        <Features />
        <Architecture />
        <CTA />
      </main>
      <Footer />
    </div>
  );
}
