import {
  Compass,
  Heart,
  Library,
  ListMusic,
  RadioTower,
  Settings,
  Upload,
  UserRoundCheck,
} from "lucide-react";
import { NavLink } from "react-router-dom";
import { useAuthStore } from "../../stores/authStore";

const links = [
  { to: "/discover", label: "Discover", icon: Compass },
  { to: "/library?view=following", label: "Following", icon: UserRoundCheck },
  { to: "/library", label: "Library", icon: Library },
  { to: "/library?view=playlists", label: "Playlists", icon: ListMusic },
  { to: "/library?view=liked", label: "Liked tracks", icon: Heart },
  { to: "/upload", label: "Upload", icon: Upload },
  { to: "/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const authMode = useAuthStore((state) => state.mode);
  const session = useAuthStore((state) => state.session);
  const publicOnly = authMode === "disabled";
  return (
    <aside className="sidebar" aria-label="Primary navigation">
      <NavLink to="/" className="brand" aria-label="Cantaloupe home">
        <RadioTower size={24} aria-hidden="true" />
        <span>Cantaloupe</span>
      </NavLink>
      <nav className="sidebar__nav">
        {links.map(({ to, label, icon: Icon }) => (
          <NavLink key={label} to={to} className={({ isActive }) => `sidebar__link ${isActive ? "is-active" : ""}`}>
            <Icon size={18} aria-hidden="true" />
            <span>{label}</span>
          </NavLink>
        ))}
      </nav>
      <div className="sidebar__account">
        <div className="avatar" aria-hidden="true">{session?.subject.slice(0, 1).toUpperCase() ?? (publicOnly ? "P" : "?")}</div>
        <div>
          <strong>{session?.displayName ?? (publicOnly ? "Public browsing" : "Signed out")}</strong>
          <span>{session ? `${session.mode} session` : publicOnly ? "Accounts unavailable" : "Connect identity provider"}</span>
        </div>
      </div>
    </aside>
  );
}
