import { useCallback, useEffect, useMemo, useRef } from "react";
import { usePlayerStore } from "../../stores/playerStore";
import type { WaveformData } from "../../types/audio";

type Variant = "detail" | "sheet" | "compact";

type LiveWaveformProps = {
  waveform: WaveformData;
  /** 이 파형이 속한 트랙. 지금 재생 중인 트랙일 때만 살아 움직인다. */
  trackId: string;
  variant?: Variant;
  interactive?: boolean;
};

const GEOMETRY: Record<Variant, { height: number; barWidth: number; barGap: number }> = {
  detail: { height: 130, barWidth: 3, barGap: 2 },
  sheet: { height: 92, barWidth: 3, barGap: 2 },
  compact: { height: 38, barWidth: 2, barGap: 1 },
};

/** 저장된 피크는 [low, high] 쌍이다. 두 값 중 큰 진폭을 0..1로 옮긴다. */
function amplitude([low, high]: [number, number]) {
  return Math.min(1, Math.max(Math.abs(low) / 128, Math.abs(high) / 127));
}

function downsample(peaks: [number, number][], count: number) {
  if (count <= 0) return new Float32Array(0);
  const bars = new Float32Array(count);
  const bucket = peaks.length / count;
  for (let i = 0; i < count; i += 1) {
    const start = Math.floor(i * bucket);
    const end = Math.max(start + 1, Math.floor((i + 1) * bucket));
    let peak = 0;
    for (let j = start; j < end && j < peaks.length; j += 1) peak = Math.max(peak, amplitude(peaks[j]));
    bars[i] = peak;
  }
  return bars;
}

function readColor(element: HTMLElement, name: string, fallback: string) {
  const value = getComputedStyle(element).getPropertyValue(name).trim();
  return value || fallback;
}

/**
 * 재생 중인 구간이 실제 진폭에 맞춰 솟아오르는 파형.
 *
 * 튀는 양은 전부 저장된 피크에서 나온다. 임의의 흔들림을 섞지 않기 때문에
 * 조용한 구간에서는 실제로 잠잠하다.
 */
export function LiveWaveform({ waveform, trackId, variant = "detail", interactive = false }: LiveWaveformProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const barsRef = useRef<Float32Array>(new Float32Array(0));
  const energyRef = useRef(0);
  const colorsRef = useRef({ played: "#55d6be", pending: "#667085" });

  const geometry = GEOMETRY[variant];
  const trackDuration = Math.max(0.001, waveform.duration_ms / 1000);

  const isCurrent = usePlayerStore((state) => state.currentTrack?.id === trackId);
  const isPlaying = usePlayerStore((state) => state.isPlaying);
  const storeTime = usePlayerStore((state) => state.currentTime);

  const peaks = waveform.peaks;
  const peakCount = peaks.length;

  const readProgress = useCallback(() => {
    const state = usePlayerStore.getState();
    if (state.currentTrack?.id !== trackId) return 0;
    const time = state.mediaElement?.currentTime ?? state.currentTime;
    return Math.min(1, Math.max(0, time / trackDuration));
  }, [trackDuration, trackId]);

  const amplitudeAt = useMemo(() => {
    return (progress: number) => {
      if (!peakCount) return 0;
      const position = Math.min(peakCount - 1, Math.max(0, progress * (peakCount - 1)));
      const index = Math.floor(position);
      const next = Math.min(peakCount - 1, index + 1);
      const blend = position - index;
      return amplitude(peaks[index]) * (1 - blend) + amplitude(peaks[next]) * blend;
    };
  }, [peakCount, peaks]);

  const draw = useCallback(
    (progress: number, energy: number) => {
      const canvas = canvasRef.current;
      const context = canvas?.getContext("2d");
      if (!canvas || !context) return;

      const ratio = canvas.width / Math.max(1, canvas.clientWidth);
      const width = canvas.clientWidth;
      const height = geometry.height;
      const bars = barsRef.current;
      const total = bars.length;
      if (!total) return;

      const { played, pending } = colorsRef.current;

      context.setTransform(ratio, 0, 0, ratio, 0, 0);
      context.clearRect(0, 0, width, height);

      const step = geometry.barWidth + geometry.barGap;
      const head = progress * total;
      // 재생 지점 주변만 부풀린다. 폭은 막대 수에 비례시켜 어떤 길이에서도 비슷하게 보인다.
      const spread = Math.max(3, total * 0.03);

      for (let i = 0; i < total; i += 1) {
        const distance = Math.abs(i - head);
        const lift = energy * Math.exp(-(distance * distance) / (2 * spread * spread));
        const value = Math.min(1, bars[i] * (1 + lift * 0.95));
        const barHeight = Math.max(2, value * height * 0.94);
        const x = i * step;
        const y = (height - barHeight) / 2;

        context.fillStyle = i <= head ? played : pending;
        context.globalAlpha = i <= head ? 0.55 + 0.45 * value : 0.4;
        if (context.roundRect) {
          context.beginPath();
          context.roundRect(x, y, geometry.barWidth, barHeight, geometry.barWidth / 2);
          context.fill();
        } else {
          context.fillRect(x, y, geometry.barWidth, barHeight);
        }
      }

      context.globalAlpha = 1;
      if (progress > 0 && progress < 1) {
        // 재생 헤드의 번짐도 실제 진폭을 따라간다.
        const x = Math.min(width - 1, head * step);
        context.shadowColor = played;
        context.shadowBlur = 6 + energy * 16;
        context.fillStyle = played;
        context.fillRect(x, 0, 1.5, height);
        context.shadowBlur = 0;
      }
    },
    [geometry.barGap, geometry.barWidth, geometry.height],
  );

  // 캔버스 크기와 막대 샘플링은 폭이 바뀔 때만 다시 잡는다.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const resize = () => {
      const width = canvas.clientWidth;
      if (!width) return;
      const ratio = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.round(width * ratio);
      canvas.height = Math.round(geometry.height * ratio);
      colorsRef.current = {
        played: readColor(canvas, "--signal", "#55d6be"),
        pending: readColor(canvas, "--muted", "#667085"),
      };
      const count = Math.max(1, Math.floor(width / (geometry.barWidth + geometry.barGap)));
      barsRef.current = downsample(peaks, count);
      draw(readProgress(), 0);
    };

    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    return () => observer.disconnect();
  }, [draw, geometry.barGap, geometry.barWidth, geometry.height, peaks, readProgress]);

  // 재생 중에는 엘리먼트를 직접 읽어 프레임마다 그린다.
  useEffect(() => {
    if (!isCurrent || !isPlaying) {
      energyRef.current = 0;
      return;
    }

    let frame = 0;
    const render = () => {
      frame = requestAnimationFrame(render);
      const progress = readProgress();
      const target = amplitudeAt(progress);
      // 프레임마다 목표치로 당겨 급격한 값 변화를 다듬는다.
      energyRef.current += (target - energyRef.current) * 0.35;
      energyRef.current = target > energyRef.current ? target : energyRef.current;
      draw(progress, energyRef.current);
    };
    frame = requestAnimationFrame(render);
    return () => cancelAnimationFrame(frame);
  }, [amplitudeAt, draw, isCurrent, isPlaying, readProgress]);

  // 일시정지·정지 상태는 store의 낮은 빈도 갱신만 따라간다. 이 effect를
  // animation loop와 분리해야 timeupdate가 60fps loop를 재시작하지 않는다.
  useEffect(() => {
    if (isCurrent && isPlaying) return;
    energyRef.current = 0;
    draw(isCurrent ? Math.min(1, storeTime / trackDuration) : 0, 0);
  }, [draw, isCurrent, isPlaying, storeTime, trackDuration]);

  const seekTo = useCallback(
    (seconds: number) => {
      const state = usePlayerStore.getState();
      const media = state.mediaElement;
      if (!media || state.currentTrack?.id !== trackId) return;
      const nextTime = Math.min(trackDuration, Math.max(0, seconds));
      media.currentTime = nextTime;
      state.setTiming(nextTime, media.duration || trackDuration);
    },
    [trackDuration, trackId],
  );

  const seek = (event: React.MouseEvent<HTMLCanvasElement>) => {
    if (!interactive || !isCurrent) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
    seekTo(ratio * trackDuration);
  };

  const seekWithKeyboard = (event: React.KeyboardEvent<HTMLCanvasElement>) => {
    if (!interactive || !isCurrent) return;
    const currentTime = usePlayerStore.getState().mediaElement?.currentTime ?? storeTime;
    let nextTime: number | undefined;
    if (event.key === "ArrowLeft") nextTime = currentTime - 5;
    if (event.key === "ArrowRight") nextTime = currentTime + 5;
    if (event.key === "Home") nextTime = 0;
    if (event.key === "End") nextTime = trackDuration;
    if (nextTime === undefined) return;
    event.preventDefault();
    seekTo(nextTime);
  };

  const canSeek = interactive && isCurrent;
  const accessibleTime = Math.round(Math.min(trackDuration, Math.max(0, storeTime)));
  const accessibleDuration = Math.max(1, Math.round(trackDuration));

  return (
    <canvas
      ref={canvasRef}
      className={`live-waveform live-waveform--${variant}${canSeek ? " is-interactive" : ""}`}
      style={{ height: geometry.height }}
      onClick={seek}
      onKeyDown={seekWithKeyboard}
      role={canSeek ? "slider" : "img"}
      tabIndex={canSeek ? 0 : undefined}
      aria-label={canSeek ? "Audio playback position" : "Audio waveform"}
      aria-valuemin={canSeek ? 0 : undefined}
      aria-valuemax={canSeek ? accessibleDuration : undefined}
      aria-valuenow={canSeek ? accessibleTime : undefined}
      aria-valuetext={canSeek ? `${accessibleTime} seconds of ${accessibleDuration}` : undefined}
    />
  );
}
