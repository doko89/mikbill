import { create } from "zustand";
import { ApiError, api } from "../lib/api";

export type Admin = {
  id: number;
  name: string;
  username: string;
};

type MeData = {
  admin: Admin;
  csrf_token: string;
};

type SessionState = {
  admin: Admin | null;
  csrfToken: string | null;
  ready: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

export const useSession = create<SessionState>((set) => ({
  admin: null,
  csrfToken: null,
  ready: false,

  login: async (username, password) => {
    const data = await api<MeData>("/api/admin/login", {
      method: "POST",
      body: { username, password },
    });
    set({ admin: data.admin, csrfToken: data.csrf_token, ready: true });
  },

  logout: async () => {
    try {
      await api("/api/admin/logout", { method: "POST" });
    } catch {
      // Abaikan: sesi lokal tetap dibersihkan.
    }
    set({ admin: null, csrfToken: null, ready: true });
  },

  refresh: async () => {
    try {
      const data = await api<MeData>("/api/admin/me");
      set({ admin: data.admin, csrfToken: data.csrf_token, ready: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        set({ admin: null, csrfToken: null, ready: true });
        return;
      }
      set({ ready: true });
      throw err;
    }
  },
}));
