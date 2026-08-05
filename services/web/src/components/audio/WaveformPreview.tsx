import type { WaveformData } from "../../types/audio";

export function WaveformPreview({ waveform }: { waveform?: WaveformData }) {
  const bars = waveform ? samplePeaks(waveform.peaks, 96) : Array.from({ length: 12 }, (_, index) => 0.3 + (index % 3) * 0.22);
  return (
    <div className={`waveform-placeholder ${waveform ? "has-data" : ""}`} aria-label={waveform ? "Processed audio waveform" : "Waveform unavailable"}>
      {bars.map((peak, index) => <span key={index} style={{ height: `${Math.max(5, peak * 100)}%` }} />)}
    </div>
  );
}

function samplePeaks(peaks: [number, number][], maximumBars: number): number[] {
  if (peaks.length <= maximumBars) return peaks.map(amplitude);
  const bucketSize = peaks.length / maximumBars;
  return Array.from({ length: maximumBars }, (_, index) => {
    const start = Math.floor(index * bucketSize);
    const end = Math.max(start + 1, Math.floor((index + 1) * bucketSize));
    return Math.max(...peaks.slice(start, end).map(amplitude));
  });
}

function amplitude([low, high]: [number, number]): number {
  return Math.min(1, Math.max(Math.abs(low) / 128, Math.abs(high) / 127));
}
