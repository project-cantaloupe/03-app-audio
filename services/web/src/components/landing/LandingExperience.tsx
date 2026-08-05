import { ArrowRight, RadioTower, Upload } from "lucide-react";
import { Link } from "react-router-dom";
import { useReducedMotion } from "../../hooks/useReducedMotion";
import { CinematicVideoBackground } from "./CinematicVideoBackground";

export function LandingExperience() {
  const reducedMotion = useReducedMotion();

  return (
    <div className="landing">
      <CinematicVideoBackground reducedMotion={reducedMotion} />
      <nav className="landing-nav" aria-label="Landing navigation">
        <Link to="/" className="brand animate-blur-fade-up" style={{ animationDelay: "0ms" }}><RadioTower size={23} /><span>Cantaloupe</span></Link>
        <div>
          <Link className="animate-blur-fade-up" style={{ animationDelay: "100ms" }} to="/discover">Discover</Link>
          <Link className="animate-blur-fade-up" style={{ animationDelay: "150ms" }} to="/upload">Upload</Link>
          <Link className="landing-nav__enter liquid-glass animate-blur-fade-up" style={{ animationDelay: "250ms" }} to="/discover">Enter app <ArrowRight size={15} /></Link>
        </div>
      </nav>

      <main className="landing-content">
        <section className="landing-section landing-section--hero">
          <div className="landing-copy">
            <p className="eyebrow animate-blur-fade-up" style={{ animationDelay: "300ms" }}>SHARE SOUND. FIND YOUR SIGNAL.</p>
            <h1 className="animate-blur-fade-up" style={{ animationDelay: "400ms" }}>A place for sounds<br />that deserve to travel.</h1>
            <p className="landing-lead animate-blur-fade-up" style={{ animationDelay: "500ms" }}>Upload your work, reach new listeners, and discover audio beyond the algorithm.</p>
            <div className="landing-actions">
              <Link className="button button--primary button--lg animate-blur-fade-up" style={{ animationDelay: "600ms" }} to="/discover">Start listening <ArrowRight size={17} /></Link>
              <Link className="button button--secondary button--lg liquid-glass animate-blur-fade-up" style={{ animationDelay: "700ms" }} to="/upload"><Upload size={17} /> Upload your first track</Link>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
