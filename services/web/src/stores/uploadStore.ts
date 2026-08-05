import { create } from "zustand";
import type { AudioRecord, UploadSession } from "../types/audio";

export type UploadStage = "idle" | "preparing" | "uploading" | "verifying" | "processing" | "ready" | "failed";

type UploadState = {
  stage: UploadStage;
  session: UploadSession | null;
  record: AudioRecord | null;
  transferred: number;
  total: number;
  error: string | null;
  setStage: (stage: UploadStage) => void;
  setSession: (session: UploadSession | null) => void;
  setRecord: (record: AudioRecord | null) => void;
  setProgress: (transferred: number, total: number) => void;
  setError: (error: string | null) => void;
  reset: () => void;
};

export const useUploadStore = create<UploadState>((set) => ({
  stage: "idle",
  session: null,
  record: null,
  transferred: 0,
  total: 0,
  error: null,
  setStage: (stage) => set({ stage }),
  setSession: (session) => set({ session }),
  setRecord: (record) => set({ record }),
  setProgress: (transferred, total) => set({ transferred, total }),
  setError: (error) => set({ error, stage: error ? "failed" : "idle" }),
  reset: () => set({ stage: "idle", session: null, record: null, transferred: 0, total: 0, error: null }),
}));
