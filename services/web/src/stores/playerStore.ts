import { create } from "zustand";
import type { Track } from "../types/audio";

export type RepeatMode = "off" | "one" | "all";

type PlayerState = {
  currentTrack: Track | null;
  queue: Track[];
  currentIndex: number;
  isPlaying: boolean;
  isBuffering: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isMuted: boolean;
  repeatMode: RepeatMode;
  shuffleEnabled: boolean;
  error: string | null;
  mobileOpen: boolean;
  playTrack: (track: Track, queue?: Track[]) => void;
  setPlaying: (playing: boolean) => void;
  setBuffering: (buffering: boolean) => void;
  setTiming: (currentTime: number, duration: number) => void;
  setVolume: (volume: number) => void;
  toggleMuted: () => void;
  cycleRepeat: () => void;
  toggleShuffle: () => void;
  setError: (error: string | null) => void;
  setMobileOpen: (open: boolean) => void;
  next: () => void;
  previous: () => void;
};

export const usePlayerStore = create<PlayerState>((set, get) => ({
  currentTrack: null,
  queue: [],
  currentIndex: -1,
  isPlaying: false,
  isBuffering: false,
  currentTime: 0,
  duration: 0,
  volume: 0.8,
  isMuted: false,
  repeatMode: "off",
  shuffleEnabled: false,
  error: null,
  mobileOpen: false,
  playTrack: (track, queue = [track]) => {
    const index = Math.max(0, queue.findIndex((item) => item.id === track.id));
    set({ currentTrack: track, queue, currentIndex: index, isPlaying: true, error: null });
  },
  setPlaying: (isPlaying) => set({ isPlaying }),
  setBuffering: (isBuffering) => set({ isBuffering }),
  setTiming: (currentTime, duration) => set({ currentTime, duration }),
  setVolume: (volume) => set({ volume: Math.min(1, Math.max(0, volume)), isMuted: false }),
  toggleMuted: () => set((state) => ({ isMuted: !state.isMuted })),
  cycleRepeat: () =>
    set((state) => ({ repeatMode: state.repeatMode === "off" ? "all" : state.repeatMode === "all" ? "one" : "off" })),
  toggleShuffle: () => set((state) => ({ shuffleEnabled: !state.shuffleEnabled })),
  setError: (error) => set({ error }),
  setMobileOpen: (mobileOpen) => set({ mobileOpen }),
  next: () => {
    const { queue, currentIndex, repeatMode } = get();
    if (!queue.length) return;
    const nextIndex = currentIndex + 1 < queue.length ? currentIndex + 1 : repeatMode === "all" ? 0 : currentIndex;
    const track = queue[nextIndex];
    if (track) set({ currentIndex: nextIndex, currentTrack: track, currentTime: 0, isPlaying: true, error: null });
  },
  previous: () => {
    const { queue, currentIndex, currentTime } = get();
    if (currentTime > 4) {
      set({ currentTime: 0 });
      return;
    }
    const previousIndex = Math.max(0, currentIndex - 1);
    const track = queue[previousIndex];
    if (track) set({ currentIndex: previousIndex, currentTrack: track, currentTime: 0, isPlaying: true, error: null });
  },
}));
