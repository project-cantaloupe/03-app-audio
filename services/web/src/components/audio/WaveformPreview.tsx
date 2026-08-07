const PLACEHOLDER_BARS = Array.from({ length: 12 }, (_, index) => 0.3 + (index % 3) * 0.22);

/**
 * 파형 아티팩트가 아직 없을 때 자리를 지키는 플레이스홀더.
 * 실제 피크가 도착하면 LiveWaveform이 대신 그린다.
 */
export function WaveformPreview() {
  return (
    <div className="waveform-placeholder" aria-label="Waveform unavailable">
      {PLACEHOLDER_BARS.map((peak, index) => <span key={index} style={{ height: `${peak * 100}%` }} />)}
    </div>
  );
}
