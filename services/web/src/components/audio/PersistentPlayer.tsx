import { Heart, ListMusic, Maximize2, Volume2, VolumeX } from "lucide-react";
import { useCallback, useEffect, useRef } from "react";
import { usePlayerStore } from "../../stores/playerStore";
import { formatTime } from "../../utils/time";
import { SignalArtwork } from "../ui/SignalArtwork";
import { LiveWaveform } from "./LiveWaveform";
import { MobilePlayerSheet } from "./MobilePlayerSheet";
import { PlaybackControls } from "./PlaybackControls";

export function PersistentPlayer() {
  const audioRef = useRef<HTMLAudioElement | null>(null);
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
  const setMediaElement = usePlayerStore((state) => state.setMediaElement);

  // <audio>는 트랙이 있을 때만 렌더되므로 마운트 이펙트로는 잡히지 않는다.
  // 콜백 ref라야 엘리먼트가 실제로 붙고 떨어지는 시점에 정확히 등록된다.
  const attachMedia = useCallback(
    (node: HTMLAudioElement | null) => {
      audioRef.current = node;
      setMediaElement(node);
    },
    [setMediaElement],
  );

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
  // 재생 중 queue의 다음 트랙으로 바뀌면 source effect가 audio.load()를 호출해
  // 엘리먼트가 일시정지된다. 트랙 식별자도 의존성에 넣어 새 source를 다시 재생한다.
  }, [isPlaying, setError, setPlaying, track?.id, track?.streamUrl]);

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
        <div><strong>Player ready</strong><span>Select a track to start listening.</span></div>
      </footer>
    );
  }

  return (
    <>
      <footer className="player" aria-label="Audio player">
        <audio
          ref={attachMedia}
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
          <span><strong>{track.title}</strong><small>{track.creator?.displayName ?? "Public audio"}</small></span>
        </button>
        <button className="icon-button player__like" disabled aria-label="Like track"><Heart size={17} /></button>
        <div className="player__center">
          <PlaybackControls disabled={!track.streamUrl} />
          <div className="player__timeline">
            <span>{formatTime(currentTime)}</span>
            {track.streamUrl && track.waveform ? <LiveWaveform waveform={track.waveform} trackId={track.id} variant="compact" interactive /> : <div className="player__empty-wave" aria-hidden="true" />}
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
      <MobilePlayerSheet />
    </>
  );
}
