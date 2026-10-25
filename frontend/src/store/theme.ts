import { create } from "zustand";
import { api } from "../lib/api";
import {
  THEME_DEFAULTS,
  applyTheme,
  readCachedTheme,
  sanitizeTheme,
  writeCachedTheme,
} from "../lib/theme";
import type { ThemeData } from "../lib/theme";

type ThemeStore = ThemeData & {
  ready: boolean;
  load: () => Promise<void>;
};

// useTheme menyimpan tema publik: server dulu, cache lalu default bila gagal.
export const useTheme = create<ThemeStore>((set) => ({
  ...THEME_DEFAULTS,
  ready: false,

  load: async () => {
    const cache = readCachedTheme();
    if (cache) {
      set({ ...cache, ready: true });
      applyTheme(cache);
    }
    try {
      const data = sanitizeTheme(await api<unknown>("/api/theme"));
      set({ ...data, ready: true });
      applyTheme(data);
      writeCachedTheme(data);
    } catch {
      if (!cache) {
        set({ ...THEME_DEFAULTS, ready: true });
        applyTheme(THEME_DEFAULTS);
      }
    }
  },
}));
