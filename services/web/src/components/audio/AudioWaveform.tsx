import { useEffect, useRef } from "react";
import WaveSurfer from "wavesurfer.js";
import type { WaveformData } from "../../types/audio";

type AudioWaveformProps = {
  media: HTMLAudioElement | null;
  height?: number;
  compact?: boolean;
  waveform: WaveformData;
};

export function AudioWaveform({ media, height = 38, compact = false, waveform }: AudioWaveformProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current || !media || !media.src) return;
    const wavesurfer = WaveSurfer.create({
      container: containerRef.current,
      media,
      peaks: [waveform.peaks.flatMap(([low, high]) => [low / 128, high / 127])],
      duration: waveform.duration_ms / 1000,
      height,
      barWidth: compact ? 1 : 2,
      barGap: compact ? 1 : 2,
      barRadius: 2,
      waveColor: "#62666d",
      progressColor: "#f7f8fa",
      cursorColor: "#f7f8fa",
      cursorWidth: compact ? 0 : 1,
      normalize: true,
      interact: !compact,
    });
    return () => wavesurfer.destroy();
  }, [compact, height, media, media?.src, waveform]);

  return <div ref={containerRef} className="audio-waveform" aria-hidden="true" />;
}
