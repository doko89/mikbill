import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowSquareOut, CheckCircle, Copy, FloppyDisk, Trash, WifiHigh } from "@phosphor-icons/react";
import { ApiError, api, apiForm } from "../../lib/api";
import { Field, FormError, PageSkeleton, TextInput } from "../../components/ui";

type Settings = {
  deploy_mode: string;
  site_url: string;
  local_server_ip: string;
  hotspot_login_base: string;
  voucher_length: number;
  kriskris_api_key: string;
  kriskris_api_secret_set: boolean;
  kriskris_callback_secret_set: boolean;
  brand_name?: string;
  brand_tagline?: string;
  accent_color?: string;
  theme_mode?: string;
};

type Theme = {
  brand_name: string;
  brand_tagline: string;
  accent_color: string;
  theme_mode: string;
  logo_url: string | null;
};

type Form = {
  deployMode: string;
  siteUrl: string;
  localIp: string;
  loginBase: string;
  voucherLength: string;
  apiKey: string;
  apiSecret: string;
  callbackSecret: string;
  brandName: string;
  brandTagline: string;
  accent: string;
  themeMode: string;
};

const empty: Form = {
  deployMode: "vps",
  siteUrl: "",
  localIp: "",
  loginBase: "",
  voucherLength: "8",
  apiKey: "",
  apiSecret: "",
  callbackSecret: "",
  brandName: "Mikbil",
  brandTagline: "Beli voucher WiFi mandiri",
  accent: "#059669",
  themeMode: "system",
};

const DEFAULT_BRAND = "Mikbil";
const DEFAULT_TAGLINE = "Beli voucher WiFi mandiri";
const DEFAULT_ACCENT = "#059669";
const HEX_RE = /^#[0-9a-fA-F]{6}$/;
const THEME_MODES = new Set(["system", "light", "dark"]);
const MAX_LOGO_BYTES = 512 * 1024;

// Preset aksen: rona 700 agar teks putih tetap kontras,
// kecuali zamrud bawaan sesuai kontrak tema.
const ACCENTS = ["#059669", "#0f766e", "#0369a1", "#4338ca", "#be123c", "#c2410c"];

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan";
}

// Teks di atas aksen: aksen terang memakai tinta hijau tua,
// sisanya putih. Selaras aturan luminance kontrak tema.
function inkFor(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return lum > 0.6 ? "#052e16" : "#ffffff";
}

function Section({ title, desc, children }: { title: string; desc: string; children: React.ReactNode }) {
  return (
    <section className="card p-5">
      <h2 className="text-lg font-bold tracking-tight">{title}</h2>
      <p className="mb-4 text-sm" style={{ color: "var(--text-2)" }}>{desc}</p>
      <div className="grid gap-4">{children}</div>
    </section>
  );
}

export default function Settings() {
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState<Form>(empty);
  const [secretSet, setSecretSet] = useState({ api: false, callback: false });
  const [formError, setFormError] = useState<string | null>(null);
  const [brandError, setBrandError] = useState<string | null>(null);
  const [taglineError, setTaglineError] = useState<string | null>(null);
  const [accentError, setAccentError] = useState<string | null>(null);
  const [logoError, setLogoError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [logoUrl, setLogoUrl] = useState<string | null>(null);
  const [logoFile, setLogoFile] = useState<File | null>(null);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const [logoBusy, setLogoBusy] = useState(false);
  const previewRef = useRef<string | null>(null);

  const callbackUrl = `${form.siteUrl.replace(/\/+$/, "")}/api/webhooks/kriskris`;
  const accentTrim = form.accent.trim();
  const previewAccent = HEX_RE.test(accentTrim) ? accentTrim : DEFAULT_ACCENT;
  const shownLogo = logoPreview ?? logoUrl;

  function setPreview(url: string | null) {
    if (previewRef.current) URL.revokeObjectURL(previewRef.current);
    previewRef.current = url;
    setLogoPreview(url);
  }

  useEffect(() => {
    return () => {
      if (previewRef.current) URL.revokeObjectURL(previewRef.current);
    };
  }, []);

  async function copyCallbackUrl() {
    try {
      await navigator.clipboard.writeText(callbackUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setFormError("Gagal menyalin, salin manual URL callback di atas");
    }
  }

  const applySettings = useCallback((s: Settings) => {
    setForm({
      deployMode: s.deploy_mode || "vps",
      siteUrl: s.site_url,
      localIp: s.local_server_ip,
      loginBase: s.hotspot_login_base,
      voucherLength: String(s.voucher_length || 8),
      apiKey: s.kriskris_api_key,
      apiSecret: "",
      callbackSecret: "",
      brandName: s.brand_name?.trim() ? s.brand_name : DEFAULT_BRAND,
      brandTagline: s.brand_tagline ?? DEFAULT_TAGLINE,
      accent: s.accent_color && HEX_RE.test(s.accent_color.trim()) ? s.accent_color.trim() : DEFAULT_ACCENT,
      themeMode: s.theme_mode && THEME_MODES.has(s.theme_mode) ? s.theme_mode : "system",
    });
    setSecretSet({ api: s.kriskris_api_secret_set, callback: s.kriskris_callback_secret_set });
  }, []);

  useEffect(() => {
    let alive = true;
    api<Settings>("/api/admin/settings")
      .then((s) => {
        if (!alive) return;
        applySettings(s);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setFormError(errMsg(err));
        setLoading(false);
      });
    api<Theme>("/api/theme")
      .then((t) => {
        if (alive && t && t.logo_url) setLogoUrl(t.logo_url);
      })
      .catch(() => {
        // Logo opsional, abaikan bila gagal dimuat.
      });
    return () => {
      alive = false;
    };
  }, [applySettings]);

  function set<K extends keyof Form>(key: K, value: Form[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  function pickLogo(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0] ?? null;
    e.target.value = "";
    setLogoError(null);
    if (!f) return;
    if (f.type !== "image/png" && f.type !== "image/jpeg") {
      setLogoError("Logo harus berkas PNG atau JPEG.");
      return;
    }
    if (f.size > MAX_LOGO_BYTES) {
      setLogoError("Ukuran logo maks 512 KB.");
      return;
    }
    setLogoFile(f);
    setPreview(URL.createObjectURL(f));
  }

  function clearLogoFile() {
    setLogoFile(null);
    setPreview(null);
  }

  async function removeLogo() {
    if (!logoUrl && logoFile) {
      clearLogoFile();
      return;
    }
    if (!logoUrl) return;
    setLogoBusy(true);
    setLogoError(null);
    setNotice(null);
    try {
      await api<unknown>("/api/admin/theme/logo", { method: "DELETE" });
      setLogoUrl(null);
      clearLogoFile();
      setNotice("Logo dihapus.");
    } catch (err) {
      setLogoError(errMsg(err));
    } finally {
      setLogoBusy(false);
    }
  }

  async function save() {
    const name = form.brandName.trim();
    const tag = form.brandTagline.trim();
    const accent = form.accent.trim();
    let valid = true;
    if (name.length < 1 || name.length > 40) {
      setBrandError("Nama merek wajib 1 sampai 40 karakter.");
      valid = false;
    } else {
      setBrandError(null);
    }
    if (tag.length > 80) {
      setTaglineError("Slogan maks 80 karakter.");
      valid = false;
    } else {
      setTaglineError(null);
    }
    if (!HEX_RE.test(accent)) {
      setAccentError("Kode warna harus format #rrggbb, misal #059669.");
      valid = false;
    } else {
      setAccentError(null);
    }
    if (!valid) return;

    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const body: Record<string, unknown> = {
        deploy_mode: form.deployMode,
        site_url: form.siteUrl,
        local_server_ip: form.localIp,
        hotspot_login_base: form.loginBase,
        voucher_length: Number(form.voucherLength),
        kriskris_api_key: form.apiKey,
        brand_name: name,
        brand_tagline: tag,
        accent_color: accent.toLowerCase(),
        theme_mode: form.themeMode,
      };
      if (form.apiSecret) body.kriskris_api_secret = form.apiSecret;
      if (form.callbackSecret) body.kriskris_callback_secret = form.callbackSecret;
      const s = await api<Settings>("/api/admin/settings", { method: "PUT", body });
      applySettings(s);
      if (logoFile) {
        try {
          const fd = new FormData();
          fd.append("logo", logoFile);
          const r = await apiForm<{ logo_url: string }>("/api/admin/theme/logo", fd, "PUT");
          setLogoUrl(r.logo_url);
          clearLogoFile();
        } catch (err) {
          setLogoError(errMsg(err));
        }
      }
      setNotice("Pengaturan tersimpan.");
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <PageSkeleton lines={6} />;

  return (
    <div className="grid gap-4">
      <div>
        <h1 className="text-2xl font-extrabold tracking-tight">Pengaturan</h1>
        <p className="text-sm" style={{ color: "var(--text-2)" }}>
          Alamat situs, panjang kode voucher, kredensial KrisKris, serta tema dan merek.
        </p>
      </div>

      <FormError message={formError} />
      {notice && (
        <p className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--accent)" }} role="status">
          <CheckCircle size={18} /> {notice}
        </p>
      )}

      <Section title="Situs" desc="Alamat Mikbil dan halaman login hotspot.">
        <Field label="Mode deploy" name="deployMode">
          <select
            id="deployMode"
            name="deployMode"
            className="input"
            value={form.deployMode}
            onChange={(e) => set("deployMode", e.target.value)}
          >
            <option value="vps">VPS (domain publik, HTTPS)</option>
            <option value="local">Server lokal (domain lokal, HTTP)</option>
          </select>
        </Field>
        <Field label="Alamat situs" name="siteUrl" hint="Dipakai tombol Beli Voucher dan callback.">
          <TextInput
            id="siteUrl"
            name="siteUrl"
            type="url"
            inputMode="url"
            autoComplete="url"
            placeholder="https://wifi.contoh.com"
            value={form.siteUrl}
            onChange={(e) => set("siteUrl", e.target.value)}
          />
        </Field>
        <Field label="IP server lokal" name="localIp" hint="Wajib bila mode server lokal.">
          <TextInput
            id="localIp"
            name="localIp"
            inputMode="decimal"
            autoComplete="off"
            placeholder="10.10.10.2"
            value={form.localIp}
            onChange={(e) => set("localIp", e.target.value)}
          />
        </Field>
        <Field label="Alamat login hotspot" name="loginBase" hint="Dasar URL halaman login MikroTik.">
          <TextInput
            id="loginBase"
            name="loginBase"
            type="url"
            inputMode="url"
            autoComplete="url"
            placeholder="http://192.168.88.1/login"
            value={form.loginBase}
            onChange={(e) => set("loginBase", e.target.value)}
          />
        </Field>
      </Section>

      <Section title="Voucher" desc="Panjang kode acak untuk voucher baru.">
        <Field label="Panjang kode" name="voucherLength" hint="6 sampai 32 karakter, tanpa huruf ambigu.">
          <TextInput
            id="voucherLength"
            name="voucherLength"
            inputMode="numeric"
            autoComplete="off"
            value={form.voucherLength}
            onChange={(e) => set("voucherLength", e.target.value)}
          />
        </Field>
      </Section>

      <Section title="Tema dan Merek" desc="Nama, warna aksen, mode gelap terang, dan logo halaman pembeli.">
        <Field
          label="Nama merek"
          name="brandName"
          error={brandError ?? undefined}
          hint={`${form.brandName.length}/40 karakter. Tampil di judul halaman pembeli.`}
        >
          <TextInput
            id="brandName"
            name="brandName"
            autoComplete="off"
            maxLength={40}
            placeholder="Mikbil"
            value={form.brandName}
            onChange={(e) => set("brandName", e.target.value)}
            error={brandError ?? undefined}
          />
        </Field>
        <Field
          label="Slogan"
          name="brandTagline"
          error={taglineError ?? undefined}
          hint={`${form.brandTagline.length}/80 karakter. Boleh kosong.`}
        >
          <TextInput
            id="brandTagline"
            name="brandTagline"
            autoComplete="off"
            maxLength={80}
            placeholder="Beli voucher WiFi mandiri"
            value={form.brandTagline}
            onChange={(e) => set("brandTagline", e.target.value)}
            error={taglineError ?? undefined}
          />
        </Field>
        <Field
          label="Warna aksen"
          name="accent"
          error={accentError ?? undefined}
          hint="Dipakai tombol dan penanda di halaman pembeli."
        >
          <div className="flex flex-wrap gap-2" role="radiogroup" aria-label="Pilihan warna aksen">
            {ACCENTS.map((c) => {
              const active = accentTrim.toLowerCase() === c;
              return (
                <button
                  key={c}
                  type="button"
                  role="radio"
                  aria-checked={active}
                  aria-label={`Aksen ${c}`}
                  title={c}
                  onClick={() => {
                    set("accent", c);
                    setAccentError(null);
                  }}
                  className="h-11 w-11 rounded-[10px]"
                  style={{
                    background: c,
                    border: "1px solid var(--line)",
                    boxShadow: active ? `0 0 0 2px var(--surface), 0 0 0 4px ${c}` : "none",
                  }}
                />
              );
            })}
          </div>
          <TextInput
            id="accent"
            name="accent"
            autoComplete="off"
            spellCheck={false}
            maxLength={7}
            placeholder="#059669"
            value={form.accent}
            onChange={(e) => set("accent", e.target.value)}
            error={accentError ?? undefined}
          />
        </Field>
        <Field label="Mode tampilan" name="themeMode" hint="Ikuti sistem memakai tema perangkat pembeli.">
          <select
            id="themeMode"
            name="themeMode"
            className="input"
            value={form.themeMode}
            onChange={(e) => set("themeMode", e.target.value)}
          >
            <option value="system">Ikuti sistem</option>
            <option value="light">Terang</option>
            <option value="dark">Gelap</option>
          </select>
        </Field>
        <Field
          label="Logo"
          name="logo"
          error={logoError ?? undefined}
          hint="PNG atau JPEG, maks 512 KB, sisi maks 1024 px."
        >
          <input
            id="logo"
            name="logo"
            type="file"
            accept="image/png,image/jpeg"
            onChange={pickLogo}
            className="input"
            aria-invalid={logoError ? true : undefined}
            aria-describedby={logoError ? "logo-error" : undefined}
          />
          {shownLogo && (
            <div className="mt-3 flex items-center gap-3">
              <img
                src={shownLogo}
                alt="Pratinjau logo"
                className="h-16 w-16 rounded-[10px] object-contain"
                style={{ background: "var(--surface-2)", border: "1px solid var(--line)" }}
              />
              {logoFile && (
                <p className="text-sm" style={{ color: "var(--text-2)" }}>
                  {logoFile.name} tersimpan saat Anda menekan Simpan.
                </p>
              )}
            </div>
          )}
          {(logoUrl || logoFile) && (
            <button
              type="button"
              className="btn btn-secondary mt-2 w-full sm:w-auto"
              onClick={removeLogo}
              disabled={logoBusy}
            >
              <Trash size={18} /> {logoUrl ? (logoBusy ? "Menghapus" : "Hapus logo") : "Batalkan pilihan"}
            </button>
          )}
        </Field>
        <div>
          <p className="label" aria-hidden="true">Pratinjau</p>
          <div className="card p-4" style={{ background: "var(--surface-2)" }} aria-label="Pratinjau tema">
            <div className="flex items-center gap-3">
              {shownLogo ? (
                <img
                  src={shownLogo}
                  alt=""
                  className="h-10 w-10 rounded-[10px] object-contain"
                  style={{ background: "var(--surface)", border: "1px solid var(--line)" }}
                />
              ) : (
                <span
                  className="flex h-10 w-10 items-center justify-center rounded-[10px]"
                  style={{ background: previewAccent, color: inkFor(previewAccent) }}
                >
                  <WifiHigh size={22} />
                </span>
              )}
              <div className="min-w-0">
                <p className="truncate font-bold tracking-tight">
                  {form.brandName.trim() || DEFAULT_BRAND}
                </p>
                <p className="truncate text-sm" style={{ color: "var(--text-2)" }}>
                  {form.brandTagline.trim() || DEFAULT_TAGLINE}
                </p>
              </div>
            </div>
            <span
              className="btn mt-3 w-full"
              aria-hidden="true"
              style={{ background: previewAccent, color: inkFor(previewAccent) }}
            >
              Beli voucher
            </span>
          </div>
        </div>
      </Section>

      <Section title="KrisKris" desc="Kredensial pembayaran QRIS. Secret tidak pernah ditampilkan ulang.">
        <p className="text-sm" style={{ color: "var(--text-2)" }}>
          Untuk mendapatkan kredensial, silakan buka halaman{" "}
          <a
            href="https://kriskris.id"
            target="_blank"
            rel="noreferrer"
            className="font-semibold"
            style={{ color: "var(--accent)" }}
          >
            kriskris.id <ArrowSquareOut size={14} className="inline" />
          </a>
          .
        </p>
        <Field label="API key" name="apiKey">
          <TextInput
            id="apiKey"
            name="apiKey"
            autoComplete="off"
            value={form.apiKey}
            onChange={(e) => set("apiKey", e.target.value)}
          />
        </Field>
        <Field
          label="API secret"
          name="apiSecret"
          hint={secretSet.api ? "Sudah terisi. Kosongkan bila tidak diubah." : "Belum diisi."}
        >
          <TextInput
            id="apiSecret"
            name="apiSecret"
            type="password"
            autoComplete="new-password"
            placeholder={secretSet.api ? "••••••••" : "Isi API secret"}
            value={form.apiSecret}
            onChange={(e) => set("apiSecret", e.target.value)}
          />
        </Field>
        <Field
          label="Callback secret"
          name="callbackSecret"
          hint={
            secretSet.callback
              ? "Sudah terisi. Kosongkan bila tidak diubah. (Wajib diisi di mode VPS)"
              : "Belum diisi. (Wajib diisi di mode VPS)"
          }
        >
          <TextInput
            id="callbackSecret"
            name="callbackSecret"
            type="password"
            autoComplete="new-password"
            placeholder={secretSet.callback ? "••••••••" : "Isi callback secret"}
            value={form.callbackSecret}
            onChange={(e) => set("callbackSecret", e.target.value)}
          />
        </Field>
        {form.deployMode === "vps" && form.siteUrl ? (
          <div>
            <p className="label">URL callback</p>
            <p className="hint">Tempel URL ini di dasbor KrisKris bersama callback secret.</p>
            <p className="mt-1 break-all rounded-[10px] p-3 font-mono text-sm" style={{ background: "var(--surface-2)" }}>
              {callbackUrl}
            </p>
            <button type="button" className="btn btn-secondary mt-2 w-full sm:w-auto" onClick={copyCallbackUrl}>
              <Copy size={18} /> {copied ? "Tersalin" : "Salin URL"}
            </button>
          </div>
        ) : (
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Mode server lokal tidak memakai webhook. Status pembayaran dicek otomatis lewat polling tiap 5 detik.
          </p>
        )}
      </Section>

      <div>
        <button type="button" className="btn btn-primary w-full sm:w-auto" onClick={save} disabled={busy}>
          <FloppyDisk size={18} /> {busy ? "Menyimpan" : "Simpan pengaturan"}
        </button>
      </div>
    </div>
  );
}
