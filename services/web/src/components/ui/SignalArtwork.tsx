export function SignalArtwork({ label, compact = false }: { label: string; compact?: boolean }) {
  return (
    <div className={`signal-artwork ${compact ? "signal-artwork--compact" : ""}`} role="img" aria-label={`${label} artwork`}>
      <span />
      <span />
      <span />
      <span />
      <i />
    </div>
  );
}
