/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_PUBLIC_SITE_URL?: string;
  readonly VITE_AUTH_MODE?: "development" | "oidc";
  readonly VITE_DEV_SUBJECT?: string;
  readonly VITE_OIDC_ISSUER_URL?: string;
  readonly VITE_OIDC_CLIENT_ID?: string;
  readonly VITE_OIDC_REDIRECT_URI?: string;
  readonly VITE_OIDC_POST_LOGOUT_REDIRECT_URI?: string;
  readonly VITE_OIDC_SCOPE?: string;
  readonly VITE_CDN_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

type CantaloupeRuntimeConfig = {
  readonly authMode?: "development" | "oidc";
  readonly oidcIssuerUrl?: string;
  readonly oidcClientId?: string;
  readonly oidcRedirectUri?: string;
  readonly oidcPostLogoutRedirectUri?: string;
  readonly oidcScope?: string;
};

interface Window {
  readonly __CNTLP_RUNTIME_CONFIG__?: CantaloupeRuntimeConfig;
}
