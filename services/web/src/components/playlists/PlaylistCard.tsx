import { Link } from "react-router-dom";

export function PlaylistCard({ id, title }: { id: string; title: string }) {
  return <article className="playlist-card"><div className="playlist-card__art" aria-hidden="true" /><Link to={`/playlist/${id}`}>{title}</Link></article>;
}
