import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "VITE_");
  const publicSiteUrl = (env.VITE_PUBLIC_SITE_URL ?? "").replace(/\/$/, "");
  return {
    plugins: [
      react(),
      {
        name: "cantaloupe-public-site-url",
        transformIndexHtml: (html: string) => html.replaceAll("__PUBLIC_SITE_URL__", publicSiteUrl),
      },
    ],
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/v1": "http://localhost:8080",
        "/healthz": "http://localhost:8080",
        "/readyz": "http://localhost:8080",
      },
    },
    preview: {
      host: "0.0.0.0",
      port: 4173,
    },
    build: {
      chunkSizeWarningLimit: 700,
    },
  };
});
