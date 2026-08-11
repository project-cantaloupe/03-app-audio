import { useReducedMotion } from "framer-motion";
import { ArrowLeft, RadioTower } from "lucide-react";
import { Link } from "react-router-dom";
import { CinematicVideoBackground } from "../components/landing/CinematicVideoBackground";
import { useAuthStore } from "../stores/authStore";

export function SignInPage() {
  const session = useAuthStore((state) => state.session);
  const mode = useAuthStore((state) => state.mode);
  const loading = useAuthStore((state) => state.loading);
  const error = useAuthStore((state) => state.error);
  const signIn = useAuthStore((state) => state.signIn);
  const signOut = useAuthStore((state) => state.signOut);
  const reducedMotion = useReducedMotion();

  const heading = session ? `Signed in as ${session.displayName}` : "Sign in to publish and save audio";
  const description = session
    ? session.mode === "development"
      ? `The local API subject is ${session.subject}. This mode must not be used for public deployment.`
      : "Your Keycloak session is active. The API uses the signed token subject for ownership checks."
    : mode === "oidc"
      ? "Continue to Keycloak using Authorization Code with PKCE. No password is handled by Cantaloupe."
      : "Set VITE_DEV_SUBJECT for local development or build the Web with OIDC settings.";

  return <main className="auth-page"><CinematicVideoBackground reducedMotion={Boolean(reducedMotion)} variant="app" /><Link to="/" className="auth-page__back"><ArrowLeft size={17} /> Back to Cantaloupe</Link><section className="auth-card"><RadioTower size={30} /><p className="eyebrow">IDENTITY BOUNDARY</p><h1>{heading}</h1><p>{description}</p>{error ? <p className="inline-error" role="alert">{error}</p> : null}{session ? <><Link className="button button--primary button--lg" to="/discover">Continue to Cantaloupe</Link>{session.mode === "oidc" ? <button className="button button--secondary button--lg" disabled={loading} onClick={() => { void signOut(); }}>Sign out</button> : null}</> : <button className="button button--primary button--lg" disabled={loading || mode !== "oidc"} onClick={() => { void signIn("/discover"); }}>{loading ? "Connecting…" : "Continue with Keycloak"}</button>}<span>No client secret, password, or access token is stored in source code.</span></section></main>;
}
