import { create } from "zustand";
import {
  beginSignIn,
  getAuthMode,
  getSession,
  signOut,
  subscribeAuth,
  type AuthMode,
  type AuthSession,
} from "../services/authService";

type AuthState = {
  mode: AuthMode;
  session: AuthSession | null;
  loading: boolean;
  error: string | null;
  initialize: () => Promise<void>;
  refresh: () => Promise<void>;
  signIn: (returnTo?: string) => Promise<void>;
  signOut: () => Promise<void>;
};

let unsubscribe: (() => void) | null = null;

export const useAuthStore = create<AuthState>((set) => ({
  mode: getAuthMode(),
  session: null,
  loading: true,
  error: null,
  initialize: async () => {
    if (!unsubscribe) unsubscribe = subscribeAuth((session) => set({ session, loading: false, error: null }));
    try {
      set({ loading: true, error: null });
      set({ session: await getSession(), loading: false });
    } catch (error) {
      set({ session: null, loading: false, error: errorMessage(error) });
    }
  },
  refresh: async () => {
    try {
      set({ session: await getSession(), loading: false, error: null });
    } catch (error) {
      set({ session: null, loading: false, error: errorMessage(error) });
    }
  },
  signIn: async (returnTo) => {
    try {
      set({ loading: true, error: null });
      await beginSignIn(returnTo);
    } catch (error) {
      set({ loading: false, error: errorMessage(error) });
    }
  },
  signOut: async () => {
    try {
      set({ loading: true, error: null });
      await signOut();
    } catch (error) {
      set({ loading: false, error: errorMessage(error) });
    }
  },
}));

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Authentication could not be completed.";
}
