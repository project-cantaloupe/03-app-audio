import type { Track } from "../../types/audio";
import { TrackRow } from "../tracks/TrackRow";

export function PlaylistTrackList({ tracks }: { tracks: Track[] }) {
  return <div className="track-list">{tracks.map((track, index) => <TrackRow key={track.id} track={track} index={index} queue={tracks} />)}</div>;
}
