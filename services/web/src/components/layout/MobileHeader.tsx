import { Bell, RadioTower } from "lucide-react";
import { Link } from "react-router-dom";

export function MobileHeader() {
  return (
    <header className="mobile-header">
      <Link to="/" className="brand"><RadioTower size={21} /><span>Cantaloupe</span></Link>
      <Link to="/notifications" className="icon-button" aria-label="Notifications"><Bell size={18} /></Link>
    </header>
  );
}
