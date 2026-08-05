import { apiRequest } from "./apiClient";
import type { AudioRecord, PlaybackAccess, Track, WaveformData } from "../types/audio";

export const trackKeys = {
  detail: (audioId: string) => ["audio", audioId] as const,
  playback: (audioId: string) => ["audio", audioId, "playback"] as const,
  waveform: (audioId: string, expiresAt: string) => ["audio", audioId, "waveform", expiresAt] as const,
};

export function getAudio(audioId: string) {
  return apiRequest<AudioRecord>(`/v1/audios/${encodeURIComponent(audioId)}`);
}

export function getPlayback(audioId: string) {
  return apiRequest<PlaybackAccess>(`/v1/audios/${encodeURIComponent(audioId)}/playback`);
}

export async function getWaveform(url: string): Promise<WaveformData> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Waveform request failed with status ${response.status}.`);
  const waveform = (await response.json()) as WaveformData;
  if (waveform.schema_version !== 1 || waveform.channels !== 1 || !Array.isArray(waveform.peaks)) {
    throw new Error("The waveform artifact has an unsupported format.");
  }
  return waveform;
}

export function toTrack(record: AudioRecord, playback?: PlaybackAccess, waveform?: WaveformData): Track {
  return {
    id: record.id,
    title: record.title,
    durationSeconds: Math.max(0, Math.floor((record.duration_ms ?? 0) / 1000)),
    genres: [],
    moods: [],
    visibility: record.visibility,
    createdAt: record.created_at,
    streamUrl: playback?.playback_url,
    waveformUrl: playback?.waveform_url,
    waveform,
  };
}

// 목록·검색 API가 추가되면 이 모듈 안에서만 응답을 Track으로 변환한다.
