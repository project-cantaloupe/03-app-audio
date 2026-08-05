import { Heart, ListMusic, Maximize2, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { usePlayerStore } from "../../stores/playerStore";
import { formatTime } from "../../utils/time";
import { SignalArtwork } from "../ui/SignalArtwork";
import { AudioWaveform } from "./AudioWaveform";
import { MobilePlayerSheet } from "./MobilePlayerSheet";
import { PlaybackControls } from "./PlaybackControls";

export function PersistentPlayer() {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [media, setMedia] = useState<HTMLAudioElement | null>(null);
  const track = usePlayerStore((state) => state.currentTrack);
  const isPlaying = usePlayerStore((state) => state.isPlaying);
  const setPlaying = usePlayerStore((state) => state.setPlaying);
  const setBuffering = usePlayerStore((state) => state.setBuffering);
  const setTiming = usePlayerStore((state) => state.setTiming);
  const setError = usePlayerStore((state) => state.setError);
  const error = usePlayerStore((state) => state.error);
  const currentTime = usePlayerStore((state) => state.currentTime);
  const duration = usePlayerStore((state) => state.duration);
  const volume = usePlayerStore((state) => state.volume);
  const setVolume = usePlayerStore((state) => state.setVolume);
  const isMuted = usePlayerStore((state) => state.isMuted);
  const toggleMuted = usePlayerStore((state) => state.toggleMuted);
  const setMobileOpen = usePlayerStore((state) => state.setMobileOpen);
  const next = usePlayerStore((state) => state.next);

  useEffect(() => setMedia(audioRef.current), []);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    if (track?.streamUrl) {
      audio.src = track.streamUrl;
      audio.load();
    } else {
      audio.removeAttribute("src");
      audio.load();
    }
  }, [track?.id, track?.streamUrl]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !audio.src) return;
    if (isPlaying) {
      void audio.play().catch(() => {
        setPlaying(false);
        setError("Playback was blocked or the stream URL is unavailable.");
      });
    } else {
      audio.pause();
    }
  }, [isPlaying, setError, setPlaying]);

  useEffect(() => {
    if (audioRef.current) {
      audioRef.current.volume = volume;
      audioRef.current.muted = isMuted;
    }
  }, [isMuted, volume]);

  if (!track) {
    return (
      <footer className="player player--idle" aria-label="Audio player">
        <div className="player__idle-signal" aria-hidden="true" />
        <div><strong>Player ready</strong><span>Select an available track when the playback API is connected.</span></div>
      </footer>
    );
  }

  return (
    <>
      <footer className="player" aria-label="Audio player">
        <audio
          ref={audioRef}
          preload="metadata"
          onTimeUpdate={(event) => setTiming(event.currentTarget.currentTime, event.currentTarget.duration || 0)}
          onLoadedMetadata={(event) => setTiming(0, event.currentTarget.duration || 0)}
          onWaiting={() => setBuffering(true)}
          onPlaying={() => setBuffering(false)}
          onEnded={next}
          onError={() => setError("Track unavailable. Refresh the playback URL and try again.")}
        />
        <button className="player__track" onClick={() => setMobileOpen(true)} aria-label={`Open player for ${track.title}`}>
          <SignalArtwork label={track.title} compact />
          <span><strong>{track.title}</strong><small>{track.creator?.displayName ?? "Creator metadata unavailable"}</small></span>
        </button>
        <button className="icon-button player__like" disabled aria-label="Like track"><Heart size={17} /></button>
        <div className="player__center">
          <PlaybackControls disabled={!track.streamUrl} />
          <div className="player__timeline">
            <span>{formatTime(currentTime)}</span>
            {media && track.streamUrl && track.waveform ? <AudioWaveform media={media} waveform={track.waveform} compact /> : <div className="player__empty-wave" aria-hidden="true" />}
            <span>{formatTime(duration)}</span>
          </div>
        </div>
        <div className="player__tools">
          <button className="icon-button" disabled aria-label="Open queue"><ListMusic size={18} /></button>
          <button className="icon-button" onClick={toggleMuted} aria-label={isMuted ? "Unmute" : "Mute"}>{isMuted ? <VolumeX size={18} /> : <Volume2 size={18} />}</button>
          <input aria-label="Volume" type="range" min="0" max="1" step="0.01" value={volume} onChange={(event) => setVolume(Number(event.target.value))} />
          <button className="icon-button" onClick={() => setMobileOpen(true)} aria-label="Open full player"><Maximize2 size={17} /></button>
        </div>
        {error ? <p className="player__error" role="alert">{error}</p> : null}
      </footer>
      <MobilePlayerSheet mediaRef={audioRef} />
    </>
  );
}
