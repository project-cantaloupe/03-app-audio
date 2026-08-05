import { useQuery } from "@tanstack/react-query";
import { Calendar, Clock3, Lock, Play, ShieldCheck } from "lucide-react";
import { useParams } from "react-router-dom";
import { TrackActions } from "../components/tracks/TrackActions";
import { WaveformPreview } from "../components/audio/WaveformPreview";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { SignalArtwork } from "../components/ui/SignalArtwork";
import { Skeleton } from "../components/ui/Skeleton";
import { getAudio, getPlayback, getWaveform, trackKeys, toTrack } from "../services/trackService";
import { usePlayerStore } from "../stores/playerStore";
import { formatTime } from "../utils/time";

export function TrackPage() {
  const { trackId = "" } = useParams();
  const query = useQuery({ queryKey: trackKeys.detail(trackId), queryFn: () => getAudio(trackId), enabled: Boolean(trackId) });
  const playbackQuery = useQuery({ queryKey: trackKeys.playback(trackId), queryFn: () => getPlayback(trackId), enabled: query.data?.status === "READY" });
  const waveformQuery = useQuery({
    queryKey: trackKeys.waveform(trackId, playbackQuery.data?.expires_at ?? ""),
    queryFn: () => getWaveform(playbackQuery.data!.waveform_url),
    enabled: Boolean(playbackQuery.data?.waveform_url),
  });
  const playTrack = usePlayerStore((state) => state.playTrack);
  if (query.isLoading) return <div className="track-detail"><Skeleton className="track-detail__skeleton" /><Skeleton className="track-detail__skeleton-copy" /></div>;
  if (query.isError || !query.data) return <EmptyState eyebrow="TRACK UNAVAILABLE" title="This track could not be loaded" description={query.error instanceof Error ? query.error.message : "Check the track link and your access."} action={<Button variant="secondary" onClick={() => void query.refetch()}>Try again</Button>} />;
  const record = query.data;
  const track = toTrack(record, playbackQuery.data, waveformQuery.data);
  return <div className="page-stack"><header className="track-detail"><SignalArtwork label={record.title} /><div className="track-detail__copy"><p className="eyebrow">TRACK · {record.status}</p><h1>{record.title}</h1><p>Creator and descriptive metadata will appear after the metadata API is connected.</p><div className="track-meta"><span><Calendar />{new Date(record.created_at).toLocaleDateString()}</span><span><Clock3 />{record.duration_ms ? formatTime(record.duration_ms / 1000) : "Duration pending"}</span><span><Lock />{record.visibility}</span><span><ShieldCheck />{record.status === "READY" ? "Processed" : "Processing"}</span></div><div className="track-detail__actions"><Button onClick={() => playTrack(track)} disabled={!track.streamUrl}><Play size={17} fill="currentColor" /> {playbackQuery.isLoading ? "Preparing…" : "Play"}</Button><TrackActions /></div>{playbackQuery.isError ? <p className="inline-error">{playbackQuery.error instanceof Error ? playbackQuery.error.message : "Playback access could not be prepared."}</p> : null}</div></header><section className="waveform-panel"><WaveformPreview waveform={waveformQuery.data} /><div className="time-row"><span>0:00</span><span>{record.duration_ms ? formatTime(record.duration_ms / 1000) : "—"}</span></div><p>{waveformQuery.data ? "Waveform generated from the processed audio artifact." : record.status === "READY" ? "Preparing signed playback and waveform access." : "Preparing the waveform and playback artifact."}</p></section><section className="track-description"><p className="eyebrow">ABOUT THIS TRACK</p><h2>Description and credits</h2><p>No description has been published for this track. The page intentionally does not generate creator, genre, play-count, or engagement data.</p></section></div>;
}
