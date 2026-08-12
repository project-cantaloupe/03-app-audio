import { ChevronDown, Heart, ListPlus, ListMusic, Volume2 } from "lucide-react";
import { usePlayerStore } from "../../stores/playerStore";
import { formatTime } from "../../utils/time";
import { LiveWaveform } from "./LiveWaveform";
import { PlaybackControls } from "./PlaybackControls";
import { SignalArtwork } from "../ui/SignalArtwork";

export function MobilePlayerSheet() {
  const open = usePlayerStore((state) => state.mobileOpen);
  const setOpen = usePlayerStore((state) => state.setMobileOpen);
  const track = usePlayerStore((state) => state.currentTrack);
  const currentTime = usePlayerStore((state) => state.currentTime);
  const duration = usePlayerStore((state) => state.duration);

  if (!open || !track) return null;
  return (
    <div className="mobile-player-sheet" role="dialog" aria-modal="true" aria-label="Now playing">
      <button className="mobile-player-sheet__close" onClick={() => setOpen(false)} aria-label="Close player"><ChevronDown size={28} /></button>
      <SignalArtwork label={track.title} />
      <div className="mobile-player-sheet__title">
        <div><p className="eyebrow">NOW PLAYING</p><h2>{track.title}</h2><span>{track.creator?.displayName ?? "Public audio"}</span></div>
        <button className="icon-button" aria-label="Like track" disabled><Heart size={21} /></button>
      </div>
      {track.waveform ? <LiveWaveform waveform={track.waveform} trackId={track.id} variant="sheet" interactive /> : <div className="player__empty-wave" aria-hidden="true" />}
      <div className="time-row"><span>{formatTime(currentTime)}</span><span>{formatTime(duration)}</span></div>
      <PlaybackControls large disabled={!track.streamUrl} />
      <div className="mobile-player-sheet__actions">
        <button className="icon-button" aria-label="Add to playlist" disabled><ListPlus /></button>
        <button className="icon-button" aria-label="Open queue" disabled><ListMusic /></button>
        <button className="icon-button" aria-label="Choose output" disabled><Volume2 /></button>
      </div>
    </div>
  );
}
