import { Heart, ListPlus, MoreHorizontal, Play } from "lucide-react";
import { Link } from "react-router-dom";
import { usePlayerStore } from "../../stores/playerStore";
import type { Track } from "../../types/audio";
import { formatTime } from "../../utils/time";
import { SignalArtwork } from "../ui/SignalArtwork";

export function TrackCard({ track, queue = [track] }: { track: Track; queue?: Track[] }) {
  const playTrack = usePlayerStore((state) => state.playTrack);
  return (
    <article className="track-card">
      <div className="track-card__artwork">
        {track.artworkUrl ? <img src={track.artworkUrl} alt="" /> : <SignalArtwork label={track.title} />}
        <button className="track-card__play" onClick={() => playTrack(track, queue)} disabled={!track.streamUrl} aria-label={`Play ${track.title}`}><Play fill="currentColor" /></button>
      </div>
      <div className="track-card__copy">
        <Link to={`/track/${track.id}`}><strong>{track.title}</strong></Link>
        <span>{track.creator?.displayName ?? "Creator metadata unavailable"}</span>
        <small>{track.genres.join(" · ") || "Category unavailable"} · {formatTime(track.durationSeconds)}</small>
      </div>
      <div className="track-card__actions">
        <button className="icon-button" disabled aria-label="Like track"><Heart size={16} /></button>
        <button className="icon-button" disabled aria-label="Add to playlist"><ListPlus size={16} /></button>
        <button className="icon-button" disabled aria-label="More actions"><MoreHorizontal size={16} /></button>
      </div>
    </article>
  );
}
