import { MoreHorizontal, Play } from "lucide-react";
import { Link } from "react-router-dom";
import { usePlayerStore } from "../../stores/playerStore";
import type { Track } from "../../types/audio";
import { formatTime } from "../../utils/time";
import { SignalArtwork } from "../ui/SignalArtwork";

export function TrackRow({ track, index, queue }: { track: Track; index: number; queue: Track[] }) {
  const currentId = usePlayerStore((state) => state.currentTrack?.id);
  const playTrack = usePlayerStore((state) => state.playTrack);
  return (
    <div className={`track-row ${currentId === track.id ? "is-playing" : ""}`}>
      <button className="track-row__index" disabled={!track.streamUrl} onClick={() => playTrack(track, queue)} aria-label={`Play ${track.title}`}><span>{index + 1}</span><Play size={15} /></button>
      <SignalArtwork label={track.title} compact />
      <div><Link to={`/track/${track.id}`}>{track.title}</Link><span>{track.creator?.displayName ?? "Public audio"}</span></div>
      <span className="track-row__category">{track.genres[0] ?? "—"}</span>
      <span className="track-row__date">{new Date(track.createdAt).toLocaleDateString()}</span>
      <span className="track-row__duration">{formatTime(track.durationSeconds)}</span>
      <button className="icon-button" disabled aria-label="More actions"><MoreHorizontal size={16} /></button>
    </div>
  );
}
