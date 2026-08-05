import { Link } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";

export function DiscoverPage() {
  return (
    <div className="page-stack">
      <header className="page-hero page-hero--discover">
        <div><p className="eyebrow">DISCOVER INDEPENDENT AUDIO</p><h1>Find a signal<br />worth following.</h1><p>The catalog will combine music, podcasts, mixes, ambient work, and field recordings without flattening them into one feed.</p></div>
        <div className="discovery-orbit" aria-hidden="true"><span /><span /><span /></div>
      </header>
      <section className="section-block"><div className="section-heading"><div><p className="eyebrow">CATALOG</p><h2>Recently published</h2></div></div><EmptyState title="The catalog is quiet" description="Published tracks will appear when the collection API is available. You can upload and process the first track now." action={<Link className="button button--primary button--md" to="/upload">Upload a track</Link>} /></section>
      <section className="discovery-pillars" aria-label="Future discovery dimensions"><article><span>01</span><h3>By signal</h3><p>Mood, texture, and format filters built on real metadata.</p></article><article><span>02</span><h3>By creator</h3><p>Follow independent creators and hear what they publish next.</p></article><article><span>03</span><h3>By collection</h3><p>Editorial playlists without fabricated popularity rankings.</p></article></section>
    </div>
  );
}
