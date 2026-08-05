import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useLocation } from "react-router-dom";
import { Outlet } from "react-router-dom";
import { PersistentPlayer } from "../audio/PersistentPlayer";
import { CinematicVideoBackground } from "../landing/CinematicVideoBackground";
import { MobileNavigation } from "./MobileNavigation";
import { MobileHeader } from "./MobileHeader";
import { Sidebar } from "./Sidebar";
import { TopHeader } from "./TopHeader";

export function AppShell() {
  const location = useLocation();
  const reducedMotion = useReducedMotion();

  return (
    <div className="app-shell">
      <CinematicVideoBackground reducedMotion={Boolean(reducedMotion)} variant="app" />
      <Sidebar />
      <div className="app-shell__body">
        <MobileHeader />
        <TopHeader />
        <main className="app-content">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div key={location.pathname} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -4 }} transition={{ duration: 0.18 }}>
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </main>
      </div>
      <PersistentPlayer />
      <MobileNavigation />
    </div>
  );
}
