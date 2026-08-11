import { apiRequest } from "./apiClient";
import type { AudioRecord, UploadSession } from "../types/audio";

type CreateUploadResponse = {
  audio_id: string;
  upload_id: string;
  upload_url: string;
  upload_headers: Record<string, string>;
  expires_at: string;
};

export type UploadProgress = {
  transferred: number;
  total: number;
};

export async function createUpload(file: File, title: string): Promise<UploadSession> {
  const checksum = await calculateSHA256(file);
  const contentType = resolveContentType(file);
  if (!contentType) throw new Error("This audio format is not accepted by the current upload API.");
  const response = await apiRequest<CreateUploadResponse>("/v1/audios/uploads", {
    method: "POST",
    body: JSON.stringify({
      title,
      content_type: contentType,
      content_length: file.size,
      checksum_sha256: checksum,
      visibility: "public",
    }),
  });

  return {
    audioId: response.audio_id,
    uploadId: response.upload_id,
    uploadUrl: response.upload_url,
    uploadHeaders: response.upload_headers,
    expiresAt: response.expires_at,
  };
}

export function resolveContentType(file: File): string | null {
  const accepted = new Set(["audio/mpeg", "audio/wav", "audio/x-wav", "audio/flac", "audio/aac", "audio/ogg"]);
  if (accepted.has(file.type)) return file.type;
  const extension = file.name.split(".").pop()?.toLowerCase();
  return ({ mp3: "audio/mpeg", wav: "audio/wav", flac: "audio/flac", aac: "audio/aac", ogg: "audio/ogg" } as Record<string, string>)[extension ?? ""] ?? null;
}

export function uploadFile(
  session: UploadSession,
  file: File,
  onProgress: (progress: UploadProgress) => void,
  signal: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    const abort = () => request.abort();
    signal.addEventListener("abort", abort, { once: true });

    request.open("PUT", session.uploadUrl);
    Object.entries(session.uploadHeaders).forEach(([key, value]) => request.setRequestHeader(key, value));
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) onProgress({ transferred: event.loaded, total: event.total });
    });
    request.addEventListener("load", () => {
      signal.removeEventListener("abort", abort);
      if (request.status >= 200 && request.status < 300) resolve();
      else reject(new Error(`Storage upload failed with status ${request.status}.`));
    });
    request.addEventListener("error", () => {
      signal.removeEventListener("abort", abort);
      reject(new Error("The storage upload was interrupted by a network error."));
    });
    request.addEventListener("abort", () => {
      signal.removeEventListener("abort", abort);
      reject(new DOMException("Upload cancelled", "AbortError"));
    });
    request.send(file);
  });
}

export function completeUpload(audioId: string) {
  return apiRequest<AudioRecord>(`/v1/audios/${encodeURIComponent(audioId)}/complete`, {
    method: "POST",
  });
}

async function calculateSHA256(file: File): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  const bytes = new Uint8Array(digest);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}
