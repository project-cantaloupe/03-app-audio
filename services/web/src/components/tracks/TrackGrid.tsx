import type { Track } from "../../types/audio";
import { EmptyState } from "../ui/EmptyState";
import { TrackCard } from "./TrackCard";

export function TrackGrid({ tracks }: { tracks: Track[] }) {
  if (!tracks.length) return <EmptyState title="No tracks here yet" description="New audio will appear here when the catalog API is connected." />;
  return <div className="track-grid">{tracks.map((track) => <TrackCard key={track.id} track={track} queue={tracks} />)}</div>;
}
