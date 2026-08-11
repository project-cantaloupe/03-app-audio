import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { completeSignIn } from "../services/authService";
import { useAuthStore } from "../stores/authStore";

export function AuthCallbackPage() {
  const navigate = useNavigate();
  const refresh = useAuthStore((state) => state.refresh);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    void completeSignIn()
      .then(async (returnTo) => {
        await refresh();
        if (active) navigate(returnTo, { replace: true });
      })
      .catch((callbackError: unknown) => {
        if (active) setError(callbackError instanceof Error ? callbackError.message : "Sign-in could not be completed.");
      });
    return () => { active = false; };
  }, [navigate, refresh]);

  return (
    <main className="auth-page">
      <section className="auth-card" role="status">
        <p className="eyebrow">IDENTITY BOUNDARY</p>
        <h1>{error ? "Sign-in failed" : "Completing sign-in"}</h1>
        <p>{error ?? "Validating the Keycloak response and restoring your session…"}</p>
        {error ? <Link className="button button--primary button--lg" to="/sign-in">Try again</Link> : null}
      </section>
    </main>
  );
}
