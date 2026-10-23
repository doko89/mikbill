import { useCallback, useEffect, useState } from "react";
import { ArrowSquareOut, CheckCircle, Copy, FloppyDisk } from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
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
};

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan";
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
  const [notice, setNotice] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);

  const callbackUrl = `${form.siteUrl.replace(/\/+$/, "")}/api/webhooks/kriskris`;

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
    return () => {
      alive = false;
    };
  }, [applySettings]);

  function set<K extends keyof Form>(key: K, value: Form[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function save() {
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
      };
      if (form.apiSecret) body.kriskris_api_secret = form.apiSecret;
      if (form.callbackSecret) body.kriskris_callback_secret = form.callbackSecret;
      const s = await api<Settings>("/api/admin/settings", { method: "PUT", body });
      applySettings(s);
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
          Alamat situs, panjang kode voucher, dan kredensial KrisKris.
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
