import { QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { RouterProvider } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";
import { usePlayerStore } from "../stores/playerStore";
import { useUploadStore } from "../stores/uploadStore";
import { queryClient } from "./queryClient";
import { router } from "./router";

export function AppProviders() {
  const initialize = useAuthStore((state) => state.initialize);
  const subject = useAuthStore((state) => state.session?.subject ?? null);
  const previousSubject = useRef<string | null | undefined>(undefined);

  useEffect(() => { void initialize(); }, [initialize]);
  useEffect(() => {
    if (previousSubject.current !== undefined && previousSubject.current !== subject) {
      queryClient.clear();
      useUploadStore.getState().reset();
      usePlayerStore.getState().resetForIdentityChange();
    }
    previousSubject.current = subject;
  }, [subject]);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
