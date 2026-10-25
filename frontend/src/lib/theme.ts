// Helper tema publik: default kontrak, cache, dan penerapan token ke DOM.
export type ThemeMode = "system" | "light" | "dark";

export type ThemeData = {
  brand_name: string;
  brand_tagline: string;
  accent_color: string;
  theme_mode: ThemeMode;
  logo_url: string | null;
};

export const THEME_CACHE_KEY = "mikbil-theme";

export const THEME_DEFAULTS: ThemeData = {
  brand_name: "Mikbil",
  brand_tagline: "Beli voucher WiFi mandiri",
  accent_color: "#059669",
  theme_mode: "system",
  logo_url: null,
};

const HEX_RE = /^#[0-9a-fA-F]{6}$/;

// sanitizeTheme memaksa data asing (server/cache) ke bentuk kontrak yang valid.
export function sanitizeTheme(input: unknown): ThemeData {
  const d =
    typeof input === "object" && input !== null
      ? (input as Partial<Record<keyof ThemeData, unknown>>)
      : {};
  const merek = typeof d.brand_name === "string" ? d.brand_name.trim().slice(0, 40) : "";
  const slogan = typeof d.brand_tagline === "string" ? d.brand_tagline.trim().slice(0, 80) : "";
  const aksen = typeof d.accent_color === "string" ? d.accent_color : "";
  const mode = d.theme_mode;
  return {
    brand_name: merek ? merek : THEME_DEFAULTS.brand_name,
    brand_tagline: slogan,
    accent_color: HEX_RE.test(aksen) ? aksen : THEME_DEFAULTS.accent_color,
    theme_mode: mode === "light" || mode === "dark" || mode === "system" ? mode : "system",
    logo_url: typeof d.logo_url === "string" && d.logo_url ? d.logo_url : null,
  };
}

export function readCachedTheme(): ThemeData | null {
  try {
    const raw = localStorage.getItem(THEME_CACHE_KEY);
    if (!raw) return null;
    return sanitizeTheme(JSON.parse(raw) as unknown);
  } catch {
    return null;
  }
}

export function writeCachedTheme(data: ThemeData): void {
  try {
    localStorage.setItem(THEME_CACHE_KEY, JSON.stringify(data));
  } catch {
    // Abaikan: cache hanya optimasi.
  }
}

function hexKeRgb(hex: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [number, number, number];
}

function rgbKeHex(r: number, g: number, b: number): string {
  const h = (n: number) =>
    Math.round(Math.min(255, Math.max(0, n)))
      .toString(16)
      .padStart(2, "0");
  return `#${h(r)}${h(g)}${h(b)}`;
}

// luminanceRelatif menghitung luminance WCAG (0..1) untuk satu warna hex.
function luminanceRelatif(hex: string): number {
  const lin = (n: number) => {
    const c = n / 255;
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  const [r, g, b] = hexKeRgb(hex);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

// cerahkan menggeser warna ke putih sebesar porsi (0..1).
function cerahkan(hex: string, porsi: number): string {
  const [r, g, b] = hexKeRgb(hex);
  const naik = (n: number) => n + (255 - n) * porsi;
  return rgbKeHex(naik(r), naik(g), naik(b));
}

// efektifGelap menentukan mode gelap dari pilihan tema + preferensi sistem.
export function efektifGelap(mode: ThemeMode): boolean {
  if (mode === "dark") return true;
  if (mode === "light") return false;
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  );
}

// tintaAksen memilih teks di atas aksen dari luminance aksen final.
export function tintaAksen(aksenFinal: string): string {
  return luminanceRelatif(aksenFinal) > 0.25 ? "#052e16" : "#ffffff";
}

// applyTheme menerapkan tema ke <html>: data-theme, --accent/--accent-ink, judul.
export function applyTheme(data: ThemeData): void {
  const root = document.documentElement;
  root.setAttribute("data-theme", data.theme_mode);
  const aksen = efektifGelap(data.theme_mode) ? cerahkan(data.accent_color, 0.15) : data.accent_color;
  root.style.setProperty("--accent", aksen);
  root.style.setProperty("--accent-ink", tintaAksen(aksen));
  document.title = data.brand_tagline ? `${data.brand_name} - ${data.brand_tagline}` : data.brand_name;
}
