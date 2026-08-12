import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { Calendar, Clock3, Globe2, Play, ShieldCheck } from "lucide-react";
import { useParams } from "react-router-dom";
import { TrackActions } from "../components/tracks/TrackActions";
import { LiveWaveform } from "../components/audio/LiveWaveform";
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
  const hydrateTrack = usePlayerStore((state) => state.hydrateTrack);
  // 이 트랙이 재생 중일 때만 경과 시간을 보여 준다. 다른 트랙이 돌고 있으면 0:00이다.
  const playedSeconds = usePlayerStore((state) => (state.currentTrack?.id === trackId ? state.currentTime : 0));

  // 파형은 재생 버튼을 누른 뒤에 도착하는 경우가 많다. 그때 플레이어가 든
  // 스냅샷에도 채워 넣어야 하단 바 파형이 빈 줄로 남지 않는다.
  const waveformData = waveformQuery.data;
  useEffect(() => {
    if (waveformData) hydrateTrack(trackId, { waveform: waveformData });
  }, [hydrateTrack, trackId, waveformData]);

  if (query.isLoading) return <div className="track-detail"><Skeleton className="track-detail__skeleton" /><Skeleton className="track-detail__skeleton-copy" /></div>;
  if (query.isError || !query.data) return <EmptyState eyebrow="TRACK UNAVAILABLE" title="This track could not be loaded" description={query.error instanceof Error ? query.error.message : "Check the track link and your access."} action={<Button variant="secondary" onClick={() => void query.refetch()}>Try again</Button>} />;
  const record = query.data;
  const track = toTrack(record, playbackQuery.data, waveformQuery.data);
  return <div className="page-stack"><header className="track-detail"><SignalArtwork label={record.title} /><div className="track-detail__copy"><p className="eyebrow">TRACK · {record.status}</p><h1>{record.title}</h1><p>Public audio ready to play.</p><div className="track-meta"><span><Calendar />{new Date(record.created_at).toLocaleDateString()}</span><span><Clock3 />{record.duration_ms ? formatTime(record.duration_ms / 1000) : "Duration pending"}</span><span><Globe2 />{record.visibility}</span><span><ShieldCheck />{record.status === "READY" ? "Processed" : "Processing"}</span></div><div className="track-detail__actions"><Button onClick={() => playTrack(track)} disabled={!track.streamUrl}><Play size={17} fill="currentColor" /> {playbackQuery.isLoading ? "Preparing…" : "Play"}</Button><TrackActions /></div>{playbackQuery.isError ? <p className="inline-error">{playbackQuery.error instanceof Error ? playbackQuery.error.message : "Playback access could not be prepared."}</p> : null}</div></header><section className="waveform-panel">{waveformQuery.data ? <LiveWaveform waveform={waveformQuery.data} trackId={trackId} variant="detail" interactive /> : <WaveformPreview />}<div className="time-row"><span>{formatTime(playedSeconds)}</span><span>{record.duration_ms ? formatTime(record.duration_ms / 1000) : "—"}</span></div><p>{waveformQuery.data ? "Waveform generated from the processed audio artifact." : record.status === "READY" ? "Preparing signed playback and waveform access." : "Preparing the waveform and playback artifact."}</p></section><section className="track-description"><p className="eyebrow">ABOUT THIS TRACK</p><h2>Description and credits</h2><p>No description or credits have been published for this track.</p></section></div>;
}
