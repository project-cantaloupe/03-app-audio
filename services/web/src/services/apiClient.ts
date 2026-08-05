import { getAuthHeaders } from "./authService";
import { ApiError, type ApiErrorPayload } from "../types/api";

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  Object.entries(getAuthHeaders()).forEach(([key, value]) => headers.set(key, String(value)));
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, { ...init, headers });
  } catch {
    throw new ApiError(0, "NETWORK_ERROR", "The audio service could not be reached.");
  }

  if (!response.ok) {
    let payload: ApiErrorPayload = {};
    try {
      payload = (await response.json()) as ApiErrorPayload;
    } catch {
      // 응답 본문이 JSON이 아니어도 HTTP 상태를 보존한다.
    }
    throw new ApiError(
      response.status,
      payload.error?.code ?? "REQUEST_FAILED",
      payload.error?.message ?? `Request failed with status ${response.status}.`,
    );
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
