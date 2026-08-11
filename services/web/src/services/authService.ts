import { UserManager, WebStorageStateStore, type User } from "oidc-client-ts";

export type AuthMode = "development" | "oidc";

export type AuthSession = {
  subject: string;
  displayName: string;
  mode: AuthMode;
};

type AuthListener = (session: AuthSession | null) => void;

const runtimeConfig = window.__CNTLP_RUNTIME_CONFIG__ ?? {};
const authMode = (runtimeConfig.authMode ?? import.meta.env.VITE_AUTH_MODE ?? "development") as AuthMode;
let manager: UserManager | null = null;
let signInCallback: Promise<string> | null = null;

export function getAuthMode(): AuthMode {
  return authMode;
}

export async function getSession(): Promise<AuthSession | null> {
  if (authMode === "development") return developmentSession();
  const user = await getUserManager().getUser();
  return activeSession(user);
}

export async function getAuthHeaders(): Promise<HeadersInit> {
  if (authMode === "development") {
    const session = developmentSession();
    return session ? { "X-Cantaloupe-Subject": session.subject } : {};
  }

  const userManager = getUserManager();
  let user = await userManager.getUser();
  if (user?.expired) {
    try {
      user = await userManager.signinSilent();
    } catch {
      await userManager.removeUser();
      return {};
    }
  }
  return user?.access_token ? { Authorization: `Bearer ${user.access_token}` } : {};
}

export async function beginSignIn(returnTo = "/discover"): Promise<void> {
  if (authMode !== "oidc") throw new Error("OIDC authentication is not enabled in this build.");
  await getUserManager().signinRedirect({ state: { returnTo: safeReturnPath(returnTo) } });
}

export async function completeSignIn(): Promise<string> {
  if (authMode !== "oidc") throw new Error("OIDC authentication is not enabled in this build.");
  if (!signInCallback) {
    signInCallback = getUserManager().signinRedirectCallback().then((user) => {
      const state = user.state as { returnTo?: unknown } | undefined;
      return safeReturnPath(typeof state?.returnTo === "string" ? state.returnTo : "/discover");
    });
  }
  return signInCallback;
}

export async function signOut(): Promise<void> {
  if (authMode === "development") return;
  await getUserManager().signoutRedirect();
}

export function subscribeAuth(listener: AuthListener): () => void {
  if (authMode !== "oidc") return () => undefined;
  const userManager = getUserManager();
  const onUserLoaded = (user: User) => listener(activeSession(user));
  const onUserUnloaded = () => listener(null);
  const onAccessTokenExpired = () => listener(null);
  userManager.events.addUserLoaded(onUserLoaded);
  userManager.events.addUserUnloaded(onUserUnloaded);
  userManager.events.addAccessTokenExpired(onAccessTokenExpired);
  return () => {
    userManager.events.removeUserLoaded(onUserLoaded);
    userManager.events.removeUserUnloaded(onUserUnloaded);
    userManager.events.removeAccessTokenExpired(onAccessTokenExpired);
  };
}

function getUserManager(): UserManager {
  if (manager) return manager;
  const authority = requiredSetting(
    "OIDC issuer URL",
    runtimeConfig.oidcIssuerUrl ?? import.meta.env.VITE_OIDC_ISSUER_URL,
  );
  const clientId = requiredSetting(
    "OIDC client ID",
    runtimeConfig.oidcClientId ?? import.meta.env.VITE_OIDC_CLIENT_ID,
  );
  const redirectURI = setting(runtimeConfig.oidcRedirectUri, import.meta.env.VITE_OIDC_REDIRECT_URI)
    || `${window.location.origin}/auth/callback`;
  const postLogoutRedirectURI = setting(
    runtimeConfig.oidcPostLogoutRedirectUri,
    import.meta.env.VITE_OIDC_POST_LOGOUT_REDIRECT_URI,
  ) || window.location.origin;

  manager = new UserManager({
    authority: authority.replace(/\/$/, ""),
    client_id: clientId,
    redirect_uri: redirectURI,
    post_logout_redirect_uri: postLogoutRedirectURI,
    response_type: "code",
    scope: setting(runtimeConfig.oidcScope, import.meta.env.VITE_OIDC_SCOPE) || "openid profile email",
    automaticSilentRenew: true,
    monitorSession: false,
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
  });
  return manager;
}

function developmentSession(): AuthSession | null {
  const subject = import.meta.env.VITE_DEV_SUBJECT?.trim();
  return subject ? { subject, displayName: subject, mode: "development" } : null;
}

function activeSession(user: User | null): AuthSession | null {
  if (!user || user.expired || !user.access_token) return null;
  const subject = user.profile.sub?.trim();
  if (!subject) return null;
  const displayName = user.profile.preferred_username?.trim()
    || user.profile.name?.trim()
    || user.profile.email?.trim()
    || subject;
  return { subject, displayName, mode: "oidc" };
}

function requiredSetting(name: string, value: string | undefined): string {
  const normalized = value?.trim();
  if (!normalized) throw new Error(`${name} is required when OIDC authentication is enabled.`);
  return normalized;
}

function setting(runtimeValue: string | undefined, buildValue: string | undefined): string {
  return runtimeValue?.trim() || buildValue?.trim() || "";
}

function safeReturnPath(path: string): string {
  return path.startsWith("/") && !path.startsWith("//") && !path.includes("\\") ? path : "/discover";
}
