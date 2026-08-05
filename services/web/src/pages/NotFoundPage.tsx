import { useReducedMotion } from "framer-motion";
import { Link } from "react-router-dom";
import { CinematicVideoBackground } from "../components/landing/CinematicVideoBackground";
import { EmptyState } from "../components/ui/EmptyState";

export function NotFoundPage() {
  const reducedMotion = useReducedMotion();
  return <main className="not-found"><CinematicVideoBackground reducedMotion={Boolean(reducedMotion)} variant="app" /><EmptyState eyebrow="404 · LOST SIGNAL" title="This route does not exist" description="Return to discovery and continue from a known signal." action={<Link className="button button--primary button--md" to="/discover">Go to Discover</Link>} /></main>;
}
