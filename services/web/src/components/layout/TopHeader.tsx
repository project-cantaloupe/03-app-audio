import { ArrowLeft, ArrowRight, Bell, Search, Upload } from "lucide-react";
import { useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "../ui/Button";
import { useAuthStore } from "../../stores/authStore";

export function TopHeader() {
  const navigate = useNavigate();
  const authMode = useAuthStore((state) => state.mode);
  const session = useAuthStore((state) => state.session);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "/" && !(event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement)) {
        event.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <header className="top-header">
      <div className="top-header__history" aria-label="Navigation history">
        <button className="icon-button" onClick={() => navigate(-1)} aria-label="Go back"><ArrowLeft size={18} /></button>
        <button className="icon-button" onClick={() => navigate(1)} aria-label="Go forward"><ArrowRight size={18} /></button>
      </div>
      <form
        className="global-search"
        role="search"
        onSubmit={(event) => {
          event.preventDefault();
          const query = new FormData(event.currentTarget).get("q")?.toString().trim();
          if (query) navigate(`/search?q=${encodeURIComponent(query)}`);
        }}
      >
        <Search size={17} aria-hidden="true" />
        <input ref={searchRef} name="q" placeholder="Search tracks, creators, genres, or moods" aria-label="Search Cantaloupe" />
        <kbd>/</kbd>
      </form>
      <div className="top-header__actions">
        {!session && authMode !== "disabled" ? <Button size="sm" variant="secondary" onClick={() => navigate("/sign-in")}>Sign in</Button> : null}
        <Button size="sm" onClick={() => navigate("/upload")}><Upload size={16} /> Upload</Button>
        <button className="icon-button" onClick={() => navigate("/notifications")} aria-label="Notifications"><Bell size={18} /></button>
      </div>
    </header>
  );
}
