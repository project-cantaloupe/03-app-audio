import { Compass, Search, Upload, UserRound } from "lucide-react";
import { NavLink } from "react-router-dom";
import { useAuthStore } from "../../stores/authStore";

const publicLinks = [
  { to: "/discover", label: "Home", icon: Compass },
  { to: "/search", label: "Search", icon: Search },
  { to: "/settings", label: "Profile", icon: UserRound },
];
const uploadLink = { to: "/upload", label: "Upload", icon: Upload };

export function MobileNavigation() {
  const authDisabled = useAuthStore((state) => state.mode === "disabled");
  const links = authDisabled ? publicLinks : [publicLinks[0], publicLinks[1], uploadLink, publicLinks[2]];
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
