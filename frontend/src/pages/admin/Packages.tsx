import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowClockwise,
  Package as PackageIcon,
  PencilSimple,
  Plus,
  Trash,
} from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
import { rupiah } from "../../lib/format";
import { Field, FormError, PageSkeleton, TextInput } from "../../components/ui";

type Paket = {
  id: number;
  name: string;
  description: string;
  price_idr: number;
  validity_seconds: number;
  validity_label: string;
  profile_name: string;
  rate_limit: string;
  session_timeout: number;
  shared_users: number;
  is_active: boolean;
  sort_order: number;
};

type Profile = {
  name: string;
  rate_limit: string;
  session_timeout: number;
  shared_users: number;
};

const emptyForm = {
  name: "",
  description: "",
  price: "",
  validitySecs: "",
  validityLabel: "",
  profileMode: "existing" as "existing" | "baru",
  profileName: "",
  rateLimit: "",
  timeout: "",
  shared: "1",
  isActive: true,
  sortOrder: "0",
};

function errMsg(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Terjadi kesalahan";
}

export default function Packages() {
  const [loading, setLoading] = useState(true);
  const [items, setItems] = useState<Paket[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [profilesError, setProfilesError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyForm);

  const set = (k: keyof typeof emptyForm, v: string | boolean) =>
    setForm((f) => ({ ...f, [k]: v }));

  const fetchAll = useCallback(async () => {
    const pkgs = await api<Paket[]>("/api/admin/packages");
    let profs: Profile[] = [];
    let profErr: string | null = null;
    try {
      profs = await api<Profile[]>("/api/admin/router/profiles");
    } catch (err) {
      profErr = errMsg(err);
    }
    return { pkgs, profs, profErr };
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setFormError(null);
    try {
      const { pkgs, profs, profErr } = await fetchAll();
      setItems(pkgs);
      setProfiles(profs);
      setProfilesError(profErr);
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setLoading(false);
    }
  }, [fetchAll]);

  useEffect(() => {
    let alive = true;
    fetchAll()
      .then(({ pkgs, profs, profErr }) => {
        if (!alive) return;
        setItems(pkgs);
        setProfiles(profs);
        setProfilesError(profErr);
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

  function startAdd() {
    setForm(emptyForm);
    setEditingId(null);
    setShowForm(true);
    setFormError(null);
    setNotice(null);
  }

  function startEdit(p: Paket) {
    setForm({
      name: p.name,
      description: p.description,
      price: String(p.price_idr),
      validitySecs: String(p.validity_seconds),
      validityLabel: p.validity_label,
      profileMode: "existing",
      profileName: p.profile_name,
      rateLimit: p.rate_limit,
      timeout: String(p.session_timeout),
      shared: String(p.shared_users),
      isActive: p.is_active,
      sortOrder: String(p.sort_order),
    });
    setEditingId(p.id);
    setShowForm(true);
    setFormError(null);
    setNotice(null);
  }

  async function save() {
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      const body = {
        name: form.name,
        description: form.description,
        price_idr: Number(form.price),
        validity_seconds: Number(form.validitySecs),
        validity_label: form.validityLabel,
        profile_name: form.profileName,
        rate_limit: form.rateLimit,
        session_timeout: form.timeout === "" ? 0 : Number(form.timeout),
        shared_users: form.shared === "" ? 1 : Number(form.shared),
        is_active: form.isActive,
        sort_order: form.sortOrder === "" ? 0 : Number(form.sortOrder),
        create_profile: form.profileMode === "baru",
      };
      if (editingId === null) {
        await api("/api/admin/packages", { method: "POST", body });
        setNotice("Paket tersimpan");
      } else {
        await api(`/api/admin/packages/${editingId}`, { method: "PUT", body });
        setNotice("Paket diperbarui");
      }
      setShowForm(false);
      setForm(emptyForm);
      setEditingId(null);
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  async function hapus(id: number) {
    if (!window.confirm("Hapus paket ini? Voucher yang sudah terjual tidak ikut terhapus.")) return;
    setBusy(true);
    setFormError(null);
    setNotice(null);
    try {
      await api(`/api/admin/packages/${id}`, { method: "DELETE" });
      setNotice("Paket dihapus");
      await load();
    } catch (err) {
      setFormError(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <div>
          <h1 className="text-xl font-bold tracking-tight">Paket</h1>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Kelola paket voucher dan profile hotspotnya
          </p>
        </div>
        <div className="ml-auto flex gap-2">
          <button type="button" className="btn btn-secondary" onClick={load} disabled={loading}>
            <ArrowClockwise size={18} /> Muat ulang
          </button>
          <button type="button" className="btn btn-primary" onClick={startAdd}>
            <Plus size={18} /> Tambah
          </button>
        </div>
      </div>

      <FormError message={formError} />
      {notice && (
        <p className="card p-3 text-sm font-semibold" role="status" style={{ borderColor: "var(--accent)" }}>
          {notice}
        </p>
      )}

      {loading ? (
        <PageSkeleton lines={4} />
      ) : items.length === 0 && !formError ? (
        <div className="card grid place-items-center gap-2 p-8 text-center">
          <PackageIcon size={40} style={{ color: "var(--accent)" }} />
          <p className="text-lg font-bold tracking-tight">Belum ada paket</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Tambahkan paket pertama agar tampil di katalog pembeli.
          </p>
          <button type="button" className="btn btn-primary mt-2" onClick={startAdd}>
            <Plus size={18} /> Tambah paket
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {items.map((p) => (
            <article key={p.id} className="card flex flex-col gap-2 p-5">
              <div className="flex items-start gap-2">
                <div className="min-w-0 flex-1">
                  <h2 className="text-lg font-bold leading-snug tracking-tight">{p.name}</h2>
                  <p className="text-sm" style={{ color: "var(--text-2)" }}>
                    {p.validity_label} {p.rate_limit ? `, ${p.rate_limit}` : ""} , profile {p.profile_name}
                  </p>
                </div>
                <span
                  className="shrink-0 rounded-full px-3 py-1 text-xs font-bold"
                  style={
                    p.is_active
                      ? { background: "var(--accent)", color: "var(--accent-ink)" }
                      : { background: "var(--surface-2)", color: "var(--text-2)" }
                  }
                >
                  {p.is_active ? "Aktif" : "Nonaktif"}
                </span>
              </div>
              <p className="text-xl font-extrabold tracking-tight">{rupiah(p.price_idr)}</p>
              <div className="mt-1 flex gap-2">
                <button
                  type="button"
                  className="btn btn-secondary min-h-[44px] flex-1"
                  onClick={() => startEdit(p)}
                  disabled={busy}
                >
                  <PencilSimple size={18} /> Ubah
                </button>
                <button
                  type="button"
                  className="btn btn-danger min-h-[44px] flex-1"
                  onClick={() => hapus(p.id)}
                  disabled={busy}
                >
                  <Trash size={18} /> Hapus
                </button>
              </div>
            </article>
          ))}
        </div>
      )}

      {showForm && (
        <section className="card grid gap-4 p-5" aria-label={editingId === null ? "Tambah paket" : "Ubah paket"}>
          <h2 className="text-lg font-bold tracking-tight">
            {editingId === null ? "Tambah paket" : "Ubah paket"}
          </h2>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <Field label="Nama paket" name="name">
              <TextInput
                id="name"
                name="name"
                value={form.name}
                maxLength={64}
                placeholder="Harian"
                onChange={(e) => set("name", e.target.value)}
              />
            </Field>
            <Field label="Harga (Rp, minimal 1000)" name="price">
              <TextInput
                id="price"
                name="price"
                value={form.price}
                inputMode="numeric"
                placeholder="5000"
                onChange={(e) => set("price", e.target.value)}
              />
            </Field>
          </div>
          <Field label="Deskripsi" name="description" hint="Tampil di katalog, opsional">
            <TextInput
              id="description"
              name="description"
              value={form.description}
              maxLength={500}
              placeholder="Internet 12 jam untuk 1 perangkat"
              onChange={(e) => set("description", e.target.value)}
            />
          </Field>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <Field label="Masa aktif (detik)" name="validitySecs" hint="Contoh: 43200 untuk 12 jam">
              <TextInput
                id="validitySecs"
                name="validitySecs"
                value={form.validitySecs}
                inputMode="numeric"
                placeholder="43200"
                onChange={(e) => set("validitySecs", e.target.value)}
              />
            </Field>
            <Field label="Label masa aktif" name="validityLabel" hint="Tampil di katalog, contoh: 12 Jam">
              <TextInput
                id="validityLabel"
                name="validityLabel"
                value={form.validityLabel}
                maxLength={32}
                placeholder="12 Jam"
                onChange={(e) => set("validityLabel", e.target.value)}
              />
            </Field>
          </div>

          <fieldset className="grid gap-3">
            <legend className="label">Profile hotspot</legend>
            <div className="flex gap-2" role="radiogroup" aria-label="Sumber profile">
              <button
                type="button"
                className={`btn min-h-[44px] flex-1 ${form.profileMode === "existing" ? "btn-primary" : "btn-secondary"}`}
                onClick={() => set("profileMode", "existing")}
              >
                Pilih yang ada
              </button>
              <button
                type="button"
                className={`btn min-h-[44px] flex-1 ${form.profileMode === "baru" ? "btn-primary" : "btn-secondary"}`}
                onClick={() => set("profileMode", "baru")}
              >
                Buat baru
              </button>
            </div>
            {form.profileMode === "existing" ? (
              <Field label="Profile dari router" name="profileName">
                <select
                  id="profileName"
                  name="profileName"
                  className="input"
                  value={form.profileName}
                  onChange={(e) => set("profileName", e.target.value)}
                >
                  <option value="">Pilih profile</option>
                  {profiles.map((p) => (
                    <option key={p.name} value={p.name}>
                      {p.name}
                      {p.rate_limit ? ` (${p.rate_limit})` : ""}
                    </option>
                  ))}
                </select>
                {profilesError && (
                  <p className="hint">
                    Daftar profile tidak dapat dimuat ({profilesError}). Periksa koneksi di{" "}
                    <Link to="/admin/router" className="font-semibold" style={{ color: "var(--accent)" }}>
                      halaman router
                    </Link>{" "}
                    atau ketik nama profile manual lewat mode Buat baru.
                  </p>
                )}
              </Field>
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Field label="Nama profile baru" name="newProfile">
                  <TextInput
                    id="newProfile"
                    name="newProfile"
                    value={form.profileName}
                    maxLength={64}
                    placeholder="harian"
                    onChange={(e) => set("profileName", e.target.value)}
                  />
                </Field>
                <Field label="Rate limit" name="rateLimit" hint="Contoh: 5M/10M, opsional">
                  <TextInput
                    id="rateLimit"
                    name="rateLimit"
                    value={form.rateLimit}
                    placeholder="5M/10M"
                    onChange={(e) => set("rateLimit", e.target.value)}
                  />
                </Field>
                <Field label="Batas sesi (detik)" name="timeout" hint="0 berarti tanpa batas">
                  <TextInput
                    id="timeout"
                    name="timeout"
                    value={form.timeout}
                    inputMode="numeric"
                    placeholder="0"
                    onChange={(e) => set("timeout", e.target.value)}
                  />
                </Field>
                <Field label="Shared users" name="shared">
                  <TextInput
                    id="shared"
                    name="shared"
                    value={form.shared}
                    inputMode="numeric"
                    placeholder="1"
                    onChange={(e) => set("shared", e.target.value)}
                  />
                </Field>
              </div>
            )}
            {form.profileMode === "existing" && (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Field label="Rate limit (info)" name="rateLimitInfo" hint="Tampil di katalog">
                  <TextInput
                    id="rateLimitInfo"
                    name="rateLimitInfo"
                    value={form.rateLimit}
                    placeholder="5M/10M"
                    onChange={(e) => set("rateLimit", e.target.value)}
                  />
                </Field>
                <Field label="Urutan tampil" name="sortOrder" hint="Kecil tampil lebih dulu">
                  <TextInput
                    id="sortOrder"
                    name="sortOrder"
                    value={form.sortOrder}
                    inputMode="numeric"
                    placeholder="0"
                    onChange={(e) => set("sortOrder", e.target.value)}
                  />
                </Field>
              </div>
            )}
            {form.profileMode === "baru" && (
              <Field label="Urutan tampil" name="sortOrderBaru" hint="Kecil tampil lebih dulu">
                <TextInput
                  id="sortOrderBaru"
                  name="sortOrderBaru"
                  value={form.sortOrder}
                  inputMode="numeric"
                  placeholder="0"
                  onChange={(e) => set("sortOrder", e.target.value)}
                />
              </Field>
            )}
          </fieldset>

          <label className="flex min-h-[44px] cursor-pointer items-center gap-3 text-[15px] font-semibold">
            <input
              type="checkbox"
              className="h-5 w-5 shrink-0"
              checked={form.isActive}
              onChange={(e) => set("isActive", e.target.checked)}
            />
            Paket aktif dan tampil di katalog
          </label>

          <div className="flex gap-2">
            <button
              type="button"
              className="btn btn-secondary flex-1"
              onClick={() => {
                setShowForm(false);
                setEditingId(null);
              }}
              disabled={busy}
            >
              Batal
            </button>
            <button type="button" className="btn btn-primary flex-1" onClick={save} disabled={busy}>
              {busy ? "Menyimpan" : "Simpan"}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
