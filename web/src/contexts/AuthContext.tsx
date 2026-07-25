import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { api, setToken, clearToken } from "@/lib/api";
import { authService, AdminProfile } from "@/services/auth";

// VITE_API_URL may be a relative path (e.g. /api/v1) for reverse-proxy setups.
// For Google OAuth we MUST redirect the browser to the real backend server
// (not the Vite dev server), so we use VITE_BACKEND_URL which is always absolute.
// In production behind nginx, fall back to the current site origin so other devices
// never get sent to localhost.
const BACKEND_ORIGIN =
  import.meta.env.VITE_BACKEND_URL ??
  (typeof window !== "undefined" ? window.location.origin : "http://localhost:3000");
const GOOGLE_AUTH_URL = `${BACKEND_ORIGIN}/api/v1/auth/google`;


interface Admin {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone: string;
  avatar_url?: string | null;
  created_at?: string;
  is_super?: boolean;
}

interface AuthContextType {
  isLoggedIn: boolean;
  admin: Admin | null;
  isLoading: boolean;
  register: (firstName: string, lastName: string, phone: string, email: string, password: string) => Promise<void>;
  sendOtp: (email: string, password: string) => Promise<void>;
  verifyOtp: (email: string, otp: string) => Promise<boolean>;
  loginWithGoogle: () => void;
  logout: () => void;
  updateProfile: (data: { first_name?: string; last_name?: string; phone?: string }) => Promise<AdminProfile>;
  uploadAvatar: (file: Blob) => Promise<string>;
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [admin, setAdmin] = useState<Admin | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // ── Google OAuth callback: ?token=<access_token> ──────────────────────────
    // After a successful Google login the backend redirects here with the token
    // in the URL so we can pick it up and store it in memory. We then immediately
    // clean the URL to avoid it leaking into history or being bookmarked.
    const params = new URLSearchParams(window.location.search);
    const googleToken = params.get("token");

    if (googleToken) {
      // Store the access token and fetch the admin profile.
      setToken(googleToken);
      params.delete("token");
      const cleanUrl = window.location.pathname + (params.toString() ? "?" + params.toString() : "");
      window.history.replaceState({}, "", cleanUrl);

      api.get("/auth/me")
        .then((res) => {
          setAdmin(res.data.data);
          setIsLoggedIn(true);
        })
        .catch(() => {
          clearToken();
        })
        .finally(() => setIsLoading(false));

      return; // skip the normal refresh-cookie flow below
    }

    // ── Normal page load: restore session from httpOnly refresh token cookie ──
    api.post("/auth/refresh")
      .then((res) => {
        const { token, admin: adminData } = res.data.data;
        setToken(token);
        setAdmin(adminData);
        setIsLoggedIn(true);
      })
      .catch(() => {
        clearToken();
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, []);

  const register = async (firstName: string, lastName: string, phone: string, email: string, password: string) => {
    await api.post("/auth/register", {
      first_name: firstName,
      last_name: lastName,
      phone,
      email,
      password,
    });
  };

  const sendOtp = async (email: string, password: string) => {
    try {
      await api.post("/auth/send-otp", { email, password });
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error || "Failed to send OTP";
      throw new Error(errorMsg);
    }
  };

  const verifyOtp = async (email: string, otp: string): Promise<boolean> => {
    try {
      const res = await api.post("/auth/verify-otp", { email, otp });
      const data = res.data?.data || res.data;
      const { token, admin: adminData } = data;
      if (!token || !adminData) {
        console.error("[Auth] Missing token or admin data in verify-otp response");
        return false;
      }
      setToken(token);
      setAdmin(adminData);
      setIsLoggedIn(true);
      return true;
    } catch (err: any) {
      console.error("[Auth] verifyOtp error:", err?.response?.data?.error || (err as Error)?.message);
      return false;
    }
  };

  /** Redirects the browser to the backend Google OAuth flow. */
  const loginWithGoogle = () => {
    window.location.href = GOOGLE_AUTH_URL;
  };

  const logout = () => {
    api.post("/auth/logout").catch(() => {});
    clearToken();
    setAdmin(null);
    setIsLoggedIn(false);
  };

  const updateProfile = async (data: { first_name?: string; last_name?: string; phone?: string }) => {
    const updated = await authService.updateProfile(data);
    setAdmin((prev) => (prev ? { ...prev, ...updated } : updated));
    return updated;
  };

  const uploadAvatar = async (file: Blob) => {
    const { avatar_url } = await authService.uploadAvatar(file);
    setAdmin((prev) => (prev ? { ...prev, avatar_url } : prev));
    return avatar_url;
  };

  const changePassword = async (currentPassword: string, newPassword: string) => {
    await authService.changePassword(currentPassword, newPassword);
  };

  return (
    <AuthContext.Provider value={{ isLoggedIn, admin, isLoading, register, sendOtp, verifyOtp, loginWithGoogle, logout, updateProfile, uploadAvatar, changePassword }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
