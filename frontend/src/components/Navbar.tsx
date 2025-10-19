import { Link, NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  User,
  MessageCircle,
  Users,
} from 'lucide-react'
import { useEffect } from 'react';
import api from "../api/axios"
import s from "./Navbar.module.css";
import { useAuth } from "../context/AuthContext";
import Logo from './Logo';

function Navbar() {
  const { user } = useAuth();
  const cls = (isActive: boolean) => `${s.link} ${isActive ? s.active : ""}`;

  // Global heartbeat for ONLINE indicator
  useEffect(() => {
    // Do nothing, if not logged in
    if (!user) return;

    let timer: number | null = null;
    const inFlightRef = { current: false };

    const ping = () => {
        if (inFlightRef.current) return;
        inFlightRef.current = true;
        api.post("/me/ping")
          .catch(() => {})
          .finally(() => { inFlightRef.current = false; });
      };

    const start = () => {
      if (timer != null) return;            
      ping(); 
      timer = window.setInterval(ping, 30_000) as unknown as number;
    };

    const stop = () => {
      if (timer != null) {
        clearInterval(timer);
        timer = null;
      }
    };

    const onVisibility = () => {
      if (document.visibilityState === "visible") {
        start();
      } else {
        stop();
      }
    };

    // Small hooks for good behavior in all browsers
    const onFocus = start;
    const onBlur = stop;
    const onOnline = start;
    const onOffline = stop;

    onVisibility();

    window.addEventListener("focus", onFocus);
    window.addEventListener("blur", onBlur);
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      stop();
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("blur", onBlur);
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  // Restart when the user changes or the token changes
  }, [user]);


  return (
    <header className={s.navbar}>
      <Link to="/dashboard" className={s.brand}>
        <Logo size="small" />
        <span className={s.brandText}>Interlink</span>
      </Link>

      <nav className={s.nav} aria-label="Main">
        <NavLink to="/dashboard" className={({ isActive }) => cls(isActive)}>
          <LayoutDashboard size={18} /> <span>Dashboard</span>
        </NavLink>
        <NavLink to="/recommendations" className={({ isActive }) => cls(isActive)}>
          <Users size={18} /> <span>Recommendations</span>
        </NavLink>
        
        {/* Center logo for mobile/tablet */}
        <Link to="/dashboard" className={s.centerLogo}>
          <Logo size="small" />
        </Link>
        
        <NavLink to="/chat" className={({ isActive }) => cls(isActive)}>
          <MessageCircle size={18} /> <span>Chat</span>
        </NavLink>
        <NavLink to="/profile" className={({ isActive }) => cls(isActive)}>
          <User size={18} /> <span>Profile</span>
        </NavLink>
      </nav>

    </header>
  )
}

export default Navbar
