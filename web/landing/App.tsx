import Hero from "./components/Hero";
import Features from "./components/Features";
import Architecture from "./components/Architecture";
import CTA from "./components/CTA";
import Footer from "./components/Footer";

export default function App() {
  return (
    <div className="landing">
      <Hero />
      <Features />
      <Architecture />
      <CTA />
      <Footer />
    </div>
  );
}
