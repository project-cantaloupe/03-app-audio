export type AudioStatus =
  | "UPLOAD_PENDING"
  | "UPLOADED"
  | "SCANNING"
  | "CLEAN"
  | "QUARANTINED"
  | "SCAN_FAILED"
  | "QUEUED"
  | "TRANSCODING"
  | "READY"
  | "TRANSCODE_FAILED"
  | "DELETED";

export type Visibility = "public" | "unlisted" | "private";

export type AudioRecord = {
  id: string;
  title: string;
  visibility: Visibility;
  status: AudioStatus;
  duration_ms?: number;
  created_at: string;
  updated_at: string;
};

export type PlaybackAccess = {
  audio_id: string;
  playback_url: string;
  waveform_url: string;
  expires_at: string;
};

export type WaveformData = {
  schema_version: 1;
  duration_ms: number;
  points_per_second: number;
  bits: 8;
  channels: 1;
  peaks: [number, number][];
};

export type CreatorSummary = {
  id: string;
  username: string;
  displayName: string;
  avatarUrl?: string;
};

export type Track = {
  id: string;
  title: string;
  description?: string;
  artworkUrl?: string;
  streamUrl?: string;
  waveformUrl?: string;
  waveform?: WaveformData;
  durationSeconds: number;
  creator?: CreatorSummary;
  genres: string[];
  moods: string[];
  visibility: Visibility;
  createdAt: string;
};

export type UploadSession = {
  audioId: string;
  uploadId: string;
  uploadUrl: string;
  uploadHeaders: Record<string, string>;
  expiresAt: string;
};
