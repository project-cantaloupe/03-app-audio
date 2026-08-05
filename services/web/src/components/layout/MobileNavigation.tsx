import { Compass, Library, Search, Upload, UserRound } from "lucide-react";
import { NavLink } from "react-router-dom";

const links = [
  { to: "/discover", label: "Home", icon: Compass },
  { to: "/search", label: "Search", icon: Search },
  { to: "/library", label: "Library", icon: Library },
  { to: "/upload", label: "Upload", icon: Upload },
  { to: "/settings", label: "Profile", icon: UserRound },
];

export function MobileNavigation() {
  return (
    <nav className="mobile-nav" aria-label="Mobile navigation">
      {links.map(({ to, label, icon: Icon }) => (
        <NavLink key={label} to={to} className={({ isActive }) => (isActive ? "is-active" : "")}>
          <Icon size={20} aria-hidden="true" />
          <span>{label}</span>
        </NavLink>
      ))}
    </nav>
  );
}
