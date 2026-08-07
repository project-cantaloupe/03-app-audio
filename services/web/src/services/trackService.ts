import { apiRequest } from "./apiClient";
import type { AudioPage, AudioRecord, PlaybackAccess, Track, WaveformData } from "../types/audio";

export const trackKeys = {
  detail: (audioId: string) => ["audio", audioId] as const,
  playback: (audioId: string) => ["audio", audioId, "playback"] as const,
  waveform: (audioId: string, expiresAt: string) => ["audio", audioId, "waveform", expiresAt] as const,
  list: () => ["audios"] as const,
  publicList: () => ["audios", "public"] as const,
};

export function getAudio(audioId: string) {
  return apiRequest<AudioRecord>(`/v1/audios/${encodeURIComponent(audioId)}`);
}

// 본인이 올린 트랙만 최신순으로 돌려준다. 공개 피드가 아니다.
// 처리 중인 트랙도 함께 오므로 목록에서 진행 상태를 볼 수 있다.
export function listAudios(params: { limit?: number; cursor?: string; scope?: "owner" | "public" } = {}) {
  const query = new URLSearchParams();
  if (params.limit) query.set("limit", String(params.limit));
  if (params.cursor) query.set("cursor", params.cursor);
  if (params.scope === "public") query.set("scope", "public");
  const suffix = query.toString();
  return apiRequest<AudioPage>(`/v1/audios${suffix ? `?${suffix}` : ""}`);
}

// 공개 여부만 바꾼다. 공개해도 재생 URL은 여전히 서명이 필요하다.
export function setVisibility(audioId: string, visibility: "public" | "private") {
  return apiRequest<AudioRecord>(`/v1/audios/${encodeURIComponent(audioId)}`, {
    method: "PATCH",
    body: JSON.stringify({ visibility }),
  });
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
