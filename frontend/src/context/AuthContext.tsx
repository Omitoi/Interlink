/* eslint-disable react-refresh/only-export-components */
import React, { createContext, useContext, useEffect, useMemo, useState } from "react";
import { isAxiosError } from "axios";
import api from "../api/axios";

// Shapes returned by the backend
type MeResponse = {
  id: number;
  display_name: string;
  profile_picture: string;
  last_online?: string; // optional: backend may omit
};

// Frontend internal shape (camelCase)
export type User = {
  id: number;
  displayName: string;
  profilePicture: string;
  lastOnline: string;
};

function mapMe(r: MeResponse): User {
  return {
    id: r.id,
    displayName: r.display_name,
    profilePicture: r.profile_picture,
    lastOnline: r.last_online ?? "",
  };
}

// Minimal JWT decoder to extract user_id without adding deps
function decodeUserIdFromToken(token: string | null): number | null {
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length < 2) return null;
  try {
    const payload = parts[1];
    if (!payload) return null;
    const json = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
    const id = json?.user_id;
    if (typeof id === "number") return id;
    if (typeof id === "string") return parseInt(id, 10) || null;
    return null;
  } catch {
    return null;
  }
}

type AuthContextType = {
  user: User | null;
  loading: boolean;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshMe: () => Promise<void>;
  refreshAuth: () => Promise<void>;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<React.PropsWithChildren> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [token, setToken] = useState<string | null>(() => localStorage.getItem("token"));

  // Bootstrap on load if a token exists
  useEffect(() => {
    const boot = async () => {
      if (!token) {
        setLoading(false);
        return;
      }
      try {
        const { data } = await api.get<MeResponse>("/me");
        setUser(mapMe(data));
      } catch (err: unknown) {
        // If profile not found yet, keep token and set a minimal user so app can navigate to /profile
        if (isAxiosError(err) && err.response?.status === 404) {
          const id = decodeUserIdFromToken(token);
          if (id) {
            setUser({ id, displayName: "", profilePicture: "", lastOnline: "" });
            // Keep token; let ProfileGate redirect user to /profile to complete
            return;
          }
        }
        // Other errors: treat as invalid session
        localStorage.removeItem("token");
        setToken(null);
        setUser(null);
      } finally {
        setLoading(false);
      }
    };
    boot();
  }, [token]);

  const login = async (email: string, password: string) => {
    const { data } = await api.post<{ token: string; id: number }>("/login", { email, password });
    localStorage.setItem("token", data.token);
    setToken(data.token);
    try {
      // Try to load full user
      const me = await api.get<MeResponse>("/me").then((r) => r.data);
      setUser(mapMe(me));
    } catch (err: unknown) {
      if (isAxiosError(err) && err.response?.status === 404) {
        // Profile not yet created — set minimal user so ProtectedRoute lets user access /profile
        setUser({ id: data.id, displayName: "", profilePicture: "", lastOnline: "" });
        return;
      }
      throw err;
    }
  };

  const logout = () => {
    localStorage.removeItem("token");
    setToken(null);
    setUser(null);
  };

  const refreshMe = async () => {
    const { data } = await api.get<MeResponse>("/me");
    setUser(mapMe(data));
  };

  const refreshAuth = async () => {
    const storedToken = localStorage.getItem("token");
    setToken(storedToken);
    if (storedToken) {
      try {
        const { data } = await api.get<MeResponse>("/me");
        setUser(mapMe(data));
      } catch (err: unknown) {
        if (isAxiosError(err) && err.response?.status === 404) {
          const id = decodeUserIdFromToken(storedToken);
          if (id) {
            setUser({ id, displayName: "", profilePicture: "", lastOnline: "" });
            return;
          }
        }
        // Other errors: treat as invalid session
        localStorage.removeItem("token");
        setToken(null);
        setUser(null);
      }
    } else {
      setUser(null);
    }
  };

  const value = useMemo(
    () => ({ user, loading, token, login, logout, refreshMe, refreshAuth }),
    [user, loading, token]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
};
