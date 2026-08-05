import { create } from "zustand";
import { getSession, type AuthSession } from "../services/authService";

type AuthState = {
  session: AuthSession | null;
  refresh: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  session: getSession(),
  refresh: () => set({ session: getSession() }),
}));
