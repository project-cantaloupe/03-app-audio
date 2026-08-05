import { Pause, Play, Repeat, Repeat1, Shuffle, SkipBack, SkipForward } from "lucide-react";
import { usePlayerStore } from "../../stores/playerStore";

export function PlaybackControls({ disabled = false, large = false }: { disabled?: boolean; large?: boolean }) {
  const isPlaying = usePlayerStore((state) => state.isPlaying);
  const setPlaying = usePlayerStore((state) => state.setPlaying);
  const repeatMode = usePlayerStore((state) => state.repeatMode);
  const cycleRepeat = usePlayerStore((state) => state.cycleRepeat);
  const shuffleEnabled = usePlayerStore((state) => state.shuffleEnabled);
  const toggleShuffle = usePlayerStore((state) => state.toggleShuffle);
  const next = usePlayerStore((state) => state.next);
  const previous = usePlayerStore((state) => state.previous);

  return (
    <div className={`playback-controls ${large ? "playback-controls--large" : ""}`}>
      <button className={`icon-button ${shuffleEnabled ? "is-active" : ""}`} onClick={toggleShuffle} disabled={disabled} aria-label="Toggle shuffle"><Shuffle size={16} /></button>
      <button className="icon-button" onClick={previous} disabled={disabled} aria-label="Previous track"><SkipBack size={18} /></button>
      <button className="play-button" onClick={() => setPlaying(!isPlaying)} disabled={disabled} aria-label={isPlaying ? "Pause" : "Play"}>
        {isPlaying ? <Pause size={large ? 26 : 20} fill="currentColor" /> : <Play size={large ? 26 : 20} fill="currentColor" />}
      </button>
      <button className="icon-button" onClick={next} disabled={disabled} aria-label="Next track"><SkipForward size={18} /></button>
      <button className={`icon-button ${repeatMode !== "off" ? "is-active" : ""}`} onClick={cycleRepeat} disabled={disabled} aria-label={`Repeat ${repeatMode}`}>
        {repeatMode === "one" ? <Repeat1 size={16} /> : <Repeat size={16} />}
      </button>
    </div>
  );
}
