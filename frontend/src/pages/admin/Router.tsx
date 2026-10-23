import { useCallback, useEffect, useState } from "react";
import {
  ArrowClockwise,
  CheckCircle,
  Copy,
  DownloadSimple,
  PencilSimple,
  PlugsConnected,
  Plus,
  Trash,
  XCircle,
} from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
import { Field, FormError, PageSkeleton, TextInput } from "../../components/ui";

type Connection = {
  id: number;
  name: string;
  host: string;
  port: number;
  username: string;
  use_tls: boolean;
  is_active: boolean;
  last_test_at: string | null;
  last_test_ok: boolean | null;
  last_test_msg: string;
};

type Profile = {
  name: string;
  rate_limit: string;
  session_timeout: number;
  shared_users: number;
};

type Check = { name: string; ok: boolean; message: string };
type Step = { step: string; ok: boolean; message: string };
type ProvStatus = { deploy_mode: string; site_url: string; checks: Check[] };

const emptyConn = { name: "", host: "", port: "", username: "", password: "", useTls: false, active: true };
const emptyProfile = { name: "", rateLimit: "", timeout: "", shared: "1" };

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan";
}

function Section({ title, desc, children }: { title: string; desc: string; children: React.ReactNode }) {
  return (
    <section className="card p-5">
      <h2 className="text-lg font-bold tracking-tight">{title}</h2>
      <p className="mb-4 text-sm" style={{ color: "var(--text-2)" }}>{desc}</p>
      {children}
    </section>
  );
}

export default function Router() {
  const [loading, setLoading] = useState(true);
  const [conns, setConns] = useState<Connection[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [provStatus, setProvStatus] = useState<ProvStatus | null>(null);
  const [steps, setSteps] = useState<Step[]>([]);
  const [snippet, setSnippet] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [connForm, setConnForm] = useState(emptyConn);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [showConnForm, setShowConnForm] = useState(false);
  const [profForm, setProfForm] = useState(emptyProfile);
  const [copied, setCopied] = useState(false);

  const fetchAll = useCallback(async () => {
    const [c, p, s, sn] = await Promise.all([
      api<Connection[]>("/api/admin/connections"),
      api<Profile[]>("/api/admin/router/profiles").catch(() => [] as Profile[]),
      api<ProvStatus>("/api/admin/provisioning/status").catch(() => null),
      api<{ snippet: string }>("/api/admin/hotspot/button-snippet")
        .then((d) => d.snippet)
        .catch(() => ""),
    ]);
    return { c, p, s, sn };
  }, []);

  useEffect(() => {
    let alive = true;
    fetchAll()
      .then(({ c, p, s, sn }) => {
        if (!alive) return;
        setConns(c);
        setProfiles(p);
        setProvStatus(s);
        setSnippet(sn);
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
  }, [fetchAll]);

  async function load() {
    setLoading(true);
    setFormError(null);
    try {
      const { c, p, s, sn } = await fetchAll();
      setConns(c);
      setProfiles(p);
      setProvStatus(s);
      setSnippet(sn);
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setLoading(false);
    }
  }

  async function saveConn() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const body = {
        name: connForm.name,
        host: connForm.host,
        port: connForm.port === "" ? 0 : Number(connForm.port),
        username: connForm.username,
        password: connForm.password,
        use_tls: connForm.useTls,
        is_active: connForm.active,
      };
      if (editingId === null) {
        await api("/api/admin/connections", { method: "POST", body });
        setNotice("Koneksi tersimpan");
      } else {
        await api(`/api/admin/connections/${editingId}`, { method: "PUT", body });
        setNotice("Koneksi diperbarui");
      }
      setConnForm(emptyConn);
      setEditingId(null);
      setShowConnForm(false);
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function testConn(id: number) {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const res = await api<{ success: boolean; message: string }>(`/api/admin/connections/${id}/test`, {
        method: "POST",
      });
      if (res.success) setNotice(res.message);
      else setFormError(res.message);
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function deleteConn(id: number) {
    if (!window.confirm("Hapus koneksi ini?")) return;
    setBusy(true);
    setFormError(null);
    try {
      await api(`/api/admin/connections/${id}`, { method: "DELETE" });
      setNotice("Koneksi dihapus");
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  function startEdit(c: Connection) {
    setEditingId(c.id);
    setConnForm({
      name: c.name,
      host: c.host,
      port: String(c.port),
      username: c.username,
      password: "",
      useTls: c.use_tls,
      active: c.is_active,
    });
    setShowConnForm(true);
    window.scrollTo({ top: 0 });
  }

  async function createProfile() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      await api("/api/admin/router/profiles", {
        method: "POST",
        body: {
          name: profForm.name,
          rate_limit: profForm.rateLimit,
          session_timeout: profForm.timeout === "" ? 0 : Number(profForm.timeout),
          shared_users: Number(profForm.shared) || 1,
        },
      });
      setNotice("Profile terbuat di router");
      setProfForm(emptyProfile);
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function applyProvision() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const res = await api<{ steps: Step[] }>("/api/admin/provisioning/apply", { method: "POST" });
      setSteps(res.steps);
      const st = await api<ProvStatus>("/api/admin/provisioning/status");
      setProvStatus(st);
      setNotice(res.steps.every((s) => s.ok) ? "Provisioning selesai" : "Provisioning selesai dengan catatan, cek tiap langkah");
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function copySnippet() {
    try {
      await navigator.clipboard.writeText(snippet);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setFormError("Gagal menyalin, salin manual dari kotak teks");
    }
  }

  if (loading) {
    return (
      <div className="grid gap-4">
        <h1 className="text-xl font-bold tracking-tight">Router</h1>
        <PageSkeleton lines={6} />
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-xl font-bold tracking-tight">Router</h1>
        <button type="button" className="btn btn-secondary" onClick={load} disabled={busy}>
          <ArrowClockwise size={18} /> Muat ulang
        </button>
      </div>

      <FormError message={formError} />
      {notice && (
        <p className="text-sm font-semibold" style={{ color: "var(--accent)" }} role="status">
          {notice}
        </p>
      )}

      <Section title="Koneksi" desc="Kredensial RouterOS API. Password tulis saja, tidak pernah ditampilkan balik.">
        {conns.length === 0 ? (
          <p className="mb-3 text-sm" style={{ color: "var(--text-2)" }}>
            Belum ada koneksi. Tambahkan koneksi ke router Anda.
          </p>
        ) : (
          <ul className="mb-4 grid gap-2">
            {conns.map((c) => (
              <li key={c.id} className="card grid gap-2 p-4" style={{ background: "var(--surface-2)" }}>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-bold">{c.name}</span>
                  {c.is_active && (
                    <span
                      className="rounded-full px-2 py-0.5 text-xs font-bold"
                      style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
                    >
                      Aktif
                    </span>
                  )}
                  {c.use_tls && (
                    <span
                      className="rounded-full px-2 py-0.5 text-xs font-semibold"
                      style={{ border: "1px solid var(--line)", color: "var(--text-2)" }}
                    >
                      TLS
                    </span>
                  )}
                </div>
                <p className="font-mono text-sm" style={{ color: "var(--text-2)" }}>
                  {c.username}@{c.host}:{c.port}
                </p>
                <p className="flex items-center gap-1 text-sm" style={{ color: "var(--text-2)" }}>
                  {c.last_test_ok === true ? (
                    <CheckCircle size={18} style={{ color: "var(--accent)" }} />
                  ) : c.last_test_ok === false ? (
                    <XCircle size={18} style={{ color: "var(--danger)" }} />
                  ) : (
                    <PlugsConnected size={18} />
                  )}
                  {c.last_test_ok === null ? "Belum pernah dites" : c.last_test_msg}
                </p>
                <div className="flex flex-wrap gap-2">
                  <button type="button" className="btn btn-secondary" onClick={() => testConn(c.id)} disabled={busy}>
                    Tes
                  </button>
                  <button type="button" className="btn btn-secondary" onClick={() => startEdit(c)} disabled={busy}>
                    <PencilSimple size={18} /> Ubah
                  </button>
                  <button type="button" className="btn btn-secondary" onClick={() => deleteConn(c.id)} disabled={busy}>
                    <Trash size={18} /> Hapus
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}

        {!showConnForm ? (
          <button
            type="button"
            className="btn btn-primary w-full sm:w-auto"
            onClick={() => {
              setEditingId(null);
              setConnForm(emptyConn);
              setShowConnForm(true);
            }}
          >
            <Plus size={18} /> Tambah koneksi
          </button>
        ) : (
          <div className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Nama" name="connName">
                <TextInput
                  id="connName"
                  name="connName"
                  placeholder="cth. utama"
                  value={connForm.name}
                  onChange={(e) => setConnForm({ ...connForm, name: e.target.value })}
                />
              </Field>
              <Field label="Host router" name="connHost">
                <TextInput
                  id="connHost"
                  name="connHost"
                  placeholder="192.168.88.1"
                  value={connForm.host}
                  onChange={(e) => setConnForm({ ...connForm, host: e.target.value })}
                />
              </Field>
              <Field label="Port" name="connPort" hint="Kosongkan untuk default 8728 atau 8729.">
                <TextInput
                  id="connPort"
                  name="connPort"
                  inputMode="numeric"
                  value={connForm.port}
                  onChange={(e) => setConnForm({ ...connForm, port: e.target.value })}
                />
              </Field>
              <Field label="Username router" name="connUser">
                <TextInput
                  id="connUser"
                  name="connUser"
                  autoComplete="username"
                  value={connForm.username}
                  onChange={(e) => setConnForm({ ...connForm, username: e.target.value })}
                />
              </Field>
              <Field
                label="Password router"
                name="connPass"
                hint={editingId === null ? undefined : "Kosongkan untuk memakai password lama."}
              >
                <TextInput
                  id="connPass"
                  name="connPass"
                  type="password"
                  autoComplete={editingId === null ? "new-password" : "current-password"}
                  value={connForm.password}
                  onChange={(e) => setConnForm({ ...connForm, password: e.target.value })}
                />
              </Field>
              <div className="grid grid-cols-2 gap-3">
                <Field label="TLS" name="connTls">
                  <button
                    type="button"
                    id="connTls"
                    role="switch"
                    aria-checked={connForm.useTls}
                    onClick={() => setConnForm({ ...connForm, useTls: !connForm.useTls })}
                    className="btn btn-secondary w-full"
                  >
                    {connForm.useTls ? "Aktif" : "Mati"}
                  </button>
                </Field>
                <Field label="Aktif" name="connActive">
                  <button
                    type="button"
                    id="connActive"
                    role="switch"
                    aria-checked={connForm.active}
                    onClick={() => setConnForm({ ...connForm, active: !connForm.active })}
                    className="btn btn-secondary w-full"
                  >
                    {connForm.active ? "Ya" : "Tidak"}
                  </button>
                </Field>
              </div>
            </div>
            <div className="flex gap-2">
              <button type="button" className="btn btn-primary flex-1" onClick={saveConn} disabled={busy}>
                {busy ? "Menyimpan" : "Simpan"}
              </button>
              <button
                type="button"
                className="btn btn-secondary flex-1"
                onClick={() => {
                  setShowConnForm(false);
                  setEditingId(null);
                }}
                disabled={busy}
              >
                Batal
              </button>
            </div>
          </div>
        )}
      </Section>

      <Section title="Profile hotspot" desc="Daftar user profile langsung dari router aktif.">
        {profiles.length === 0 ? (
          <p className="mb-3 text-sm" style={{ color: "var(--text-2)" }}>
            Tidak ada profile atau router belum terhubung.
          </p>
        ) : (
          <ul className="mb-4 grid gap-2 sm:grid-cols-2">
            {profiles.map((p) => (
              <li key={p.name} className="card p-3" style={{ background: "var(--surface-2)" }}>
                <p className="font-bold">{p.name}</p>
                <p className="text-sm" style={{ color: "var(--text-2)" }}>
                  {p.rate_limit || "Tanpa batas"} | {p.shared_users} pengguna
                </p>
              </li>
            ))}
          </ul>
        )}
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Nama profile baru" name="profName">
            <TextInput
              id="profName"
              name="profName"
              placeholder="cth. paket-5m"
              value={profForm.name}
              onChange={(e) => setProfForm({ ...profForm, name: e.target.value })}
            />
          </Field>
          <Field label="Rate limit" name="profRate" hint="Format up/down, cth. 5M/5M.">
            <TextInput
              id="profRate"
              name="profRate"
              placeholder="5M/5M"
              value={profForm.rateLimit}
              onChange={(e) => setProfForm({ ...profForm, rateLimit: e.target.value })}
            />
          </Field>
          <Field label="Batas sesi (detik)" name="profTimeout" hint="0 berarti tanpa batas.">
            <TextInput
              id="profTimeout"
              name="profTimeout"
              inputMode="numeric"
              placeholder="0"
              value={profForm.timeout}
              onChange={(e) => setProfForm({ ...profForm, timeout: e.target.value })}
            />
          </Field>
          <Field label="Shared users" name="profShared">
            <TextInput
              id="profShared"
              name="profShared"
              inputMode="numeric"
              value={profForm.shared}
              onChange={(e) => setProfForm({ ...profForm, shared: e.target.value })}
            />
          </Field>
        </div>
        <button type="button" className="btn btn-primary mt-4 w-full sm:w-auto" onClick={createProfile} disabled={busy}>
          <Plus size={18} /> Buat profile
        </button>
      </Section>

      <Section title="Provisioning" desc="Walled garden, login PAP, dan DNS statik untuk mode lokal.">
        {provStatus === null ? (
          <p className="mb-3 text-sm" style={{ color: "var(--text-2)" }}>
            Status belum bisa dibaca. Pastikan koneksi aktif dan alamat situs sudah diatur.
          </p>
        ) : (
          <ul className="mb-4 grid gap-2">
            {provStatus.checks.map((c) => (
              <li key={c.name} className="flex items-start gap-2 text-sm">
                {c.ok ? (
                  <CheckCircle size={20} style={{ color: "var(--accent)", flexShrink: 0 }} />
                ) : (
                  <XCircle size={20} style={{ color: "var(--danger)", flexShrink: 0 }} />
                )}
                <span>
                  <span className="font-bold">{c.name}: </span>
                  <span style={{ color: "var(--text-2)" }}>{c.message}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
        <button type="button" className="btn btn-primary w-full sm:w-auto" onClick={applyProvision} disabled={busy}>
          {busy ? "Menerapkan" : "Terapkan provisioning"}
        </button>
        {steps.length > 0 && (
          <ul className="mt-4 grid gap-2">
            {steps.map((s) => (
              <li key={s.step} className="flex items-start gap-2 text-sm">
                {s.ok ? (
                  <CheckCircle size={20} style={{ color: "var(--accent)", flexShrink: 0 }} />
                ) : (
                  <XCircle size={20} style={{ color: "var(--danger)", flexShrink: 0 }} />
                )}
                <span>
                  <span className="font-bold">{s.step}: </span>
                  <span style={{ color: "var(--text-2)" }}>{s.message}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Template hotspot" desc="Unduh halaman login jadi atau tempel tombol beli ke login.html milik Anda.">
        <div className="grid gap-3">
          <a className="btn btn-primary w-full sm:w-auto" href="/api/admin/hotspot/login-template">
            <DownloadSimple size={18} /> Unduh login.html
          </a>
          {snippet !== "" && (
            <>
              <label className="label" htmlFor="snippet">
                Potongan tombol Beli Voucher
              </label>
              <pre
                id="snippet"
                className="card overflow-x-auto p-3 font-mono text-xs"
                style={{ background: "var(--surface-2)", whiteSpace: "pre-wrap", wordBreak: "break-word" }}
              >
                {snippet}
              </pre>
              <button type="button" className="btn btn-secondary w-full sm:w-auto" onClick={copySnippet}>
                <Copy size={18} /> {copied ? "Tersalin" : "Salin"}
              </button>
            </>
          )}
        </div>
      </Section>
    </div>
  );
}
