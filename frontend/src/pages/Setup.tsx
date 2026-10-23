import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, ArrowRight, ArrowSquareOut, CheckCircle, Copy, WifiHigh } from "@phosphor-icons/react";
import { ApiError, api } from "../lib/api";
import { Field, FormError, TextInput } from "../components/ui";

const STEPS = ["Akun admin", "Alamat Mikbil", "Router MikroTik", "KrisKris", "Aktivasi"];

type Draft = {
  name: string;
  username: string;
  password: string;
  confirm: string;
  deployMode: "vps" | "local";
  siteUrl: string;
  localIp: string;
  loginBase: string;
  routerHost: string;
  routerPort: string;
  routerUser: string;
  routerPass: string;
  routerTls: boolean;
  kkKey: string;
  kkSecret: string;
  kkCallback: string;
};

const initialDraft: Draft = {
  name: "",
  username: "",
  password: "",
  confirm: "",
  deployMode: "vps",
  siteUrl: "",
  localIp: "",
  loginBase: "http://192.168.88.1/login",
  routerHost: "",
  routerPort: "",
  routerUser: "",
  routerPass: "",
  routerTls: false,
  kkKey: "",
  kkSecret: "",
  kkCallback: "",
};

function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan";
}

export default function Setup() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<Draft>(initialDraft);
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [provSteps, setProvSteps] = useState<{ step: string; ok: boolean; message: string }[]>([]);

  const callbackUrl = `${draft.siteUrl.replace(/\/+$/, "")}/api/webhooks/kriskris`;

  async function copyCallbackUrl() {
    try {
      await navigator.clipboard.writeText(callbackUrl);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setFormError("Gagal menyalin, salin manual URL callback di atas");
    }
  }

  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));

  async function saveStep(): Promise<boolean> {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      if (step === 0) {
        if (draft.password !== draft.confirm) {
          setFormError("Konfirmasi password tidak sama");
          return false;
        }
        await api("/api/setup/admin", {
          method: "POST",
          body: { name: draft.name, username: draft.username, password: draft.password },
        });
      } else if (step === 1) {
        await api("/api/setup/site", {
          method: "POST",
          body: {
            deploy_mode: draft.deployMode,
            site_url: draft.siteUrl,
            local_server_ip: draft.localIp,
            hotspot_login_base: draft.loginBase,
          },
        });
      } else if (step === 2) {
        await api("/api/setup/connection", {
          method: "POST",
          body: {
            host: draft.routerHost,
            port: draft.routerPort === "" ? 0 : Number(draft.routerPort),
            username: draft.routerUser,
            password: draft.routerPass,
            use_tls: draft.routerTls,
          },
        });
      } else if (step === 3) {
        await api("/api/setup/kriskris", {
          method: "POST",
          body: { api_key: draft.kkKey, api_secret: draft.kkSecret, callback_secret: draft.kkCallback },
        });
      }
      return true;
    } catch (err) {
      setFormError(errorMessage(err));
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function next() {
    if (step === 4) return;
    if (await saveStep()) {
      setStep((s) => s + 1);
      window.scrollTo({ top: 0 });
    }
  }

  async function testConnection() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      await api("/api/setup/connection/test", {
        method: "POST",
        body: {
          host: draft.routerHost,
          port: draft.routerPort === "" ? 0 : Number(draft.routerPort),
          username: draft.routerUser,
          password: draft.routerPass,
          use_tls: draft.routerTls,
        },
      });
      setNotice("Koneksi router berhasil");
    } catch (err) {
      setFormError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function provision() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const res = await api<{ steps: { step: string; ok: boolean; message: string }[] }>(
        "/api/setup/provision",
        { method: "POST" },
      );
      setProvSteps(res.steps);
      setNotice(
        res.steps.every((s) => s.ok)
          ? "Provisioning router selesai"
          : "Provisioning selesai dengan catatan, cek tiap langkah",
      );
    } catch (err) {
      setFormError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function finish() {
    setBusy(true);
    setFormError(null);
    try {
      await api("/api/setup/finish", { method: "POST" });
      navigate("/admin/login", { replace: true });
    } catch (err) {
      setFormError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-md px-4 py-8">
      <header className="mb-6 flex items-center gap-3">
        <span
          className="flex h-11 w-11 items-center justify-center rounded-[10px]"
          style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
        >
          <WifiHigh size={24} weight="bold" />
        </span>
        <div>
          <h1 className="text-xl font-bold tracking-tight">Penyiapan Mikbil</h1>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Langkah {step + 1} dari 5: {STEPS[step]}
          </p>
        </div>
      </header>

      <div
        className="mb-6 h-2 w-full overflow-hidden rounded-full"
        style={{ background: "var(--surface-2)" }}
        role="progressbar"
        aria-valuenow={step + 1}
        aria-valuemin={1}
        aria-valuemax={5}
        aria-label="Kemajuan penyiapan"
      >
        <div
          className="h-full rounded-full transition-transform"
          style={{
            width: `${((step + 1) / 5) * 100}%`,
            background: "var(--accent)",
          }}
        />
      </div>

      <motion.div
        key={step}
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className="card p-5"
      >
        {step === 0 && (
          <div className="grid gap-4">
            <Field label="Nama pemilik" name="name">
              <TextInput
                id="name"
                name="name"
                autoComplete="name"
                placeholder="cth. Budi Santoso"
                value={draft.name}
                onChange={(e) => set({ name: e.target.value })}
              />
            </Field>
            <Field label="Username admin" name="username" hint="Minimal 3 karakter, dipakai untuk login.">
              <TextInput
                id="username"
                name="username"
                autoComplete="username"
                placeholder="cth. owner"
                value={draft.username}
                onChange={(e) => set({ username: e.target.value })}
              />
            </Field>
            <Field label="Password" name="password" hint="Minimal 8 karakter.">
              <TextInput
                id="password"
                name="password"
                type="password"
                autoComplete="new-password"
                value={draft.password}
                onChange={(e) => set({ password: e.target.value })}
              />
            </Field>
            <Field label="Konfirmasi password" name="confirm">
              <TextInput
                id="confirm"
                name="confirm"
                type="password"
                autoComplete="new-password"
                value={draft.confirm}
                onChange={(e) => set({ confirm: e.target.value })}
              />
            </Field>
          </div>
        )}

        {step === 1 && (
          <div className="grid gap-4">
            <fieldset>
              <legend className="label">Mode pemasangan</legend>
              <div className="grid grid-cols-2 gap-2">
                {(["vps", "local"] as const).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => set({ deployMode: mode })}
                    aria-pressed={draft.deployMode === mode}
                    className="btn btn-secondary"
                    style={
                      draft.deployMode === mode
                        ? { borderColor: "var(--accent)", color: "var(--accent)" }
                        : undefined
                    }
                  >
                    {mode === "vps" ? "VPS" : "Server lokal"}
                  </button>
                ))}
              </div>
            </fieldset>
            <Field
              label="Alamat Mikbil"
              name="siteUrl"
              hint={draft.deployMode === "vps" ? "Domain publik HTTPS, cth. https://wifi.contoh.com" : "Domain lokal, cth. http://mikbil.lan"}
            >
              <TextInput
                id="siteUrl"
                name="siteUrl"
                inputMode="url"
                autoComplete="url"
                placeholder={draft.deployMode === "vps" ? "https://wifi.contoh.com" : "http://mikbil.lan"}
                value={draft.siteUrl}
                onChange={(e) => set({ siteUrl: e.target.value })}
              />
            </Field>
            {draft.deployMode === "local" && (
              <Field label="IP server lokal" name="localIp" hint="IP LAN mesin Mikbil, cth. 10.10.10.2">
                <TextInput
                  id="localIp"
                  name="localIp"
                  inputMode="decimal"
                  placeholder="10.10.10.2"
                  value={draft.localIp}
                  onChange={(e) => set({ localIp: e.target.value })}
                />
              </Field>
            )}
            <Field label="Alamat login hotspot" name="loginBase" hint="Dipakai tombol Login WiFi di halaman voucher.">
              <TextInput
                id="loginBase"
                name="loginBase"
                inputMode="url"
                value={draft.loginBase}
                onChange={(e) => set({ loginBase: e.target.value })}
              />
            </Field>
          </div>
        )}

        {step === 2 && (
          <div className="grid gap-4">
            <Field label="Host router" name="routerHost" hint="IP atau hostname RouterOS API.">
              <TextInput
                id="routerHost"
                name="routerHost"
                placeholder="192.168.88.1"
                value={draft.routerHost}
                onChange={(e) => set({ routerHost: e.target.value })}
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label="Port" name="routerPort" hint="Kosongkan untuk default.">
                <TextInput
                  id="routerPort"
                  name="routerPort"
                  inputMode="numeric"
                  placeholder={draft.routerTls ? "8729" : "8728"}
                  value={draft.routerPort}
                  onChange={(e) => set({ routerPort: e.target.value })}
                />
              </Field>
              <Field label="TLS (API-SSL)" name="routerTls">
                <button
                  type="button"
                  id="routerTls"
                  role="switch"
                  aria-checked={draft.routerTls}
                  onClick={() => set({ routerTls: !draft.routerTls })}
                  className="btn btn-secondary w-full"
                >
                  {draft.routerTls ? "Aktif" : "Mati"}
                </button>
              </Field>
            </div>
            <Field label="Username router" name="routerUser">
              <TextInput
                id="routerUser"
                name="routerUser"
                autoComplete="username"
                value={draft.routerUser}
                onChange={(e) => set({ routerUser: e.target.value })}
              />
            </Field>
            <Field label="Password router" name="routerPass">
              <TextInput
                id="routerPass"
                name="routerPass"
                type="password"
                autoComplete="current-password"
                value={draft.routerPass}
                onChange={(e) => set({ routerPass: e.target.value })}
              />
            </Field>
            <button type="button" className="btn btn-secondary w-full" onClick={testConnection} disabled={busy}>
              Tes koneksi
            </button>
          </div>
        )}

        {step === 3 && (
          <div className="grid gap-4">
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
            <Field label="API key KrisKris" name="kkKey">
              <TextInput
                id="kkKey"
                name="kkKey"
                autoComplete="off"
                value={draft.kkKey}
                onChange={(e) => set({ kkKey: e.target.value })}
              />
            </Field>
            <Field label="API secret" name="kkSecret">
              <TextInput
                id="kkSecret"
                name="kkSecret"
                type="password"
                autoComplete="off"
                value={draft.kkSecret}
                onChange={(e) => set({ kkSecret: e.target.value })}
              />
            </Field>
            {draft.deployMode === "vps" ? (
              <>
                <Field label="Callback secret" name="kkCallback" hint="Diisi juga di dasbor KrisKris untuk webhook.">
                  <TextInput
                    id="kkCallback"
                    name="kkCallback"
                    type="password"
                    autoComplete="off"
                    value={draft.kkCallback}
                    onChange={(e) => set({ kkCallback: e.target.value })}
                  />
                </Field>
                <div>
                  <p className="label">URL callback</p>
                  <p className="hint">Tempel URL ini di dasbor KrisKris bersama callback secret di atas.</p>
                  <p className="mt-1 break-all rounded-[10px] p-3 font-mono text-sm" style={{ background: "var(--surface-2)" }}>
                    {callbackUrl}
                  </p>
                  <button type="button" className="btn btn-secondary mt-2 w-full" onClick={copyCallbackUrl}>
                    <Copy size={18} /> {copied ? "Tersalin" : "Salin URL"}
                  </button>
                </div>
              </>
            ) : (
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Mode server lokal tidak memakai webhook. Status pembayaran dicek otomatis lewat polling tiap 5 detik.
              </p>
            )}
          </div>
        )}

        {step === 4 && (
          <div className="grid gap-4">
            <p className="text-sm" style={{ color: "var(--text-2)" }}>
              Aktivasi menulis walled garden dan mengaktifkan login PAP di router, lalu menandai penyiapan selesai.
            </p>
            <button type="button" className="btn btn-secondary w-full" onClick={provision} disabled={busy}>
              Aktifkan akses
            </button>
            {provSteps.length > 0 && (
              <ul className="grid gap-2">
                {provSteps.map((s) => (
                  <li key={s.step} className="flex items-start gap-2 text-sm">
                    <CheckCircle
                      size={20}
                      style={{ color: s.ok ? "var(--accent)" : "var(--danger)", flexShrink: 0 }}
                    />
                    <span>
                      <span className="font-bold">{s.step}: </span>
                      <span style={{ color: "var(--text-2)" }}>{s.message}</span>
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <div className="card grid gap-2 p-4" style={{ background: "var(--surface-2)" }}>
              <p className="flex items-center gap-2 font-semibold">
                <CheckCircle size={20} style={{ color: "var(--accent)" }} />
                Ringkasan
              </p>
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Admin {draft.username || "(belum diisi)"} di {draft.siteUrl || "(alamat belum diisi)"}.
              </p>
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Template login.html dan tombol Beli Voucher tersedia di menu Router setelah Anda masuk.
              </p>
            </div>
          </div>
        )}

        <FormError message={formError} />
        {notice && (
          <p className="mt-2 text-sm font-semibold" style={{ color: "var(--accent)" }} role="status">
            {notice}
          </p>
        )}

        <div className="mt-5 flex gap-2">
          {step > 0 && (
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setStep((s) => s - 1)} disabled={busy}>
              <ArrowLeft size={18} /> Kembali
            </button>
          )}
          {step < 4 ? (
            <button type="button" className="btn btn-primary flex-1" onClick={next} disabled={busy}>
              {busy ? "Menyimpan" : "Lanjut"} <ArrowRight size={18} />
            </button>
          ) : (
            <button type="button" className="btn btn-primary flex-1" onClick={finish} disabled={busy}>
              {busy ? "Menyelesaikan" : "Selesai"}
            </button>
          )}
        </div>
      </motion.div>
    </div>
  );
}
