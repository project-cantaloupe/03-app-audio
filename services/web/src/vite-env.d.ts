/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_PUBLIC_SITE_URL?: string;
  readonly VITE_AUTH_MODE?: "development" | "cognito";
  readonly VITE_DEV_SUBJECT?: string;
  readonly VITE_AUTH_REGION?: string;
  readonly VITE_AUTH_USER_POOL_ID?: string;
  readonly VITE_AUTH_CLIENT_ID?: string;
  readonly VITE_CDN_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
