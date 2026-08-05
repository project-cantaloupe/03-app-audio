import { useReducedMotion } from "framer-motion";
import { ArrowLeft, RadioTower } from "lucide-react";
import { Link } from "react-router-dom";
import { CinematicVideoBackground } from "../components/landing/CinematicVideoBackground";
import { useAuthStore } from "../stores/authStore";

export function SignInPage() {
  const session = useAuthStore((state) => state.session);
  const reducedMotion = useReducedMotion();

  return <main className="auth-page"><CinematicVideoBackground reducedMotion={Boolean(reducedMotion)} variant="app" /><Link to="/" className="auth-page__back"><ArrowLeft size={17} /> Back to Cantaloupe</Link><section className="auth-card"><RadioTower size={30} /><p className="eyebrow">IDENTITY BOUNDARY</p><h1>{session ? "Development session active" : "Sign in to publish and save audio"}</h1><p>{session ? `The local API subject is ${session.subject}. This mode must not be used for public deployment.` : "Cognito sign-in, sign-up, password reset, and email verification will be connected here when the identity resources are available."}</p><button className="button button--primary button--lg" disabled>Continue with identity provider</button><span>No credentials are hardcoded or simulated in this interface.</span></section></main>;
}
