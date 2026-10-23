import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  ArrowClockwise,
  ArrowRight,
  CaretLeft,
  CaretRight,
  MagnifyingGlass,
  Receipt,
  WarningCircle,
} from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
import { rupiah } from "../../lib/format";
import { PageSkeleton } from "../../components/ui";
import { StatusBadge } from "../../components/orders";
import { formatTanggal } from "../../lib/status";

type Item = {
  order_id: string;
  status: string;
  package_name: string;
  total_amount: number;
  created_at: string;
  failed_reason: string;
};

type Daftar = {
  items: Item[];
  page: number;
  per_page: number;
  total: number;
};

const STATUS = [
  { nilai: "", label: "Semua status" },
  { nilai: "pending", label: "Menunggu bayar" },
  { nilai: "processing", label: "Memproses" },
  { nilai: "paid", label: "Lunas" },
  { nilai: "expired", label: "Kedaluarsa" },
  { nilai: "cancelled", label: "Dibatalkan" },
  { nilai: "failed", label: "Gagal" },
];

export default function Transactions() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const status = params.get("status") ?? "";
  const q = params.get("q") ?? "";
  const page = Math.max(1, Number(params.get("page") ?? "1") || 1);
  const [data, setData] = useState<Daftar | null>(null);
  const [error, setError] = useState<string | null>(null);
  const cariRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    try {
      const query = new URLSearchParams();
      if (status) query.set("status", status);
      if (q) query.set("q", q);
      query.set("page", String(page));
      const res = await api<Daftar>(`/api/admin/orders?${query.toString()}`);
      setData(res);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat transaksi");
    }
  }, [status, q, page]);

  useEffect(() => {
    const kick = window.setTimeout(() => {
      void load();
    }, 0);
    return () => window.clearTimeout(kick);
  }, [load]);

  function ubah(next: { status?: string; q?: string; page?: number }) {
    const query = new URLSearchParams(params);
    if (next.status !== undefined) {
      if (next.status) query.set("status", next.status);
      else query.delete("status");
    }
    if (next.q !== undefined) {
      if (next.q) query.set("q", next.q);
      else query.delete("q");
    }
    query.set("page", String(next.page ?? 1));
    setParams(query, { replace: false });
  }

  const totalHalaman = data ? Math.max(1, Math.ceil(data.total / data.per_page)) : 1;

  return (
    <div className="grid gap-4">
      <header>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
          <Receipt size={24} style={{ color: "var(--accent)" }} /> Transaksi
        </h1>
        <p className="text-sm" style={{ color: "var(--text-2)" }}>
          Riwayat pembelian voucher, terbaru di atas.
        </p>
      </header>

      <form
        className="grid gap-2 sm:grid-cols-[1fr_220px_auto]"
        onSubmit={(e) => {
          e.preventDefault();
          ubah({ q: (cariRef.current?.value ?? "").trim(), page: 1 });
        }}
      >
        <label className="relative block">
          <span className="sr-only">Cari pesanan</span>
          <MagnifyingGlass
            size={18}
            className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2"
            style={{ color: "var(--text-2)" }}
          />
          <input
            key={q}
            ref={cariRef}
            className="input pl-10"
            defaultValue={q}
            placeholder="Cari id pesanan atau invoice"
            autoComplete="off"
          />
        </label>
        <label className="block">
          <span className="sr-only">Filter status</span>
          <select
            className="input"
            value={STATUS.some((s) => s.nilai === status) ? status : ""}
            onChange={(e) => ubah({ status: e.target.value, page: 1 })}
          >
            {STATUS.map((s) => (
              <option key={s.nilai} value={s.nilai}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <button type="submit" className="btn btn-secondary">
          Cari
        </button>
      </form>

      {!data && !error && <PageSkeleton lines={5} />}

      {error && !data && (
        <div className="card grid place-items-center gap-3 p-8 text-center">
          <WarningCircle size={40} style={{ color: "var(--danger)" }} />
          <p className="font-bold">Transaksi tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary" onClick={() => void load()}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
        </div>
      )}

      {data && data.total === 0 && (
        <div className="card grid place-items-center gap-2 p-8 text-center">
          <p className="font-bold">Belum ada transaksi</p>
          <p className="hint">
            {q || status ? "Tidak ada yang cocok dengan filter. Ubah filter pencarian." : "Transaksi tampil di sini setelah ada pembelian."}
          </p>
          {(q || status) && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setParams({}, { replace: true })}
            >
              Bersihkan filter
            </button>
          )}
        </div>
      )}

      {data && data.total > 0 && (
        <>
          {/* Kartu di mobile, tabel di md ke atas */}
          <ul className="grid gap-3 md:hidden">
            {data.items.map((t) => (
              <li key={t.order_id} className="card">
                <Link
                  to={`/admin/transaksi/${t.order_id}`}
                  className="grid gap-2 p-4"
                  aria-label={`Detail ${t.order_id}`}
                >
                  <span className="flex items-center justify-between gap-2">
                    <StatusBadge status={t.status} />
                    <span className="text-sm" style={{ color: "var(--text-2)" }}>
                      {formatTanggal(t.created_at)}
                    </span>
                  </span>
                  <span className="truncate font-bold">{t.package_name || t.order_id}</span>
                  <span className="flex items-center justify-between gap-2">
                    <span className="truncate font-mono text-sm" style={{ color: "var(--text-2)" }}>
                      {t.order_id}
                    </span>
                    <span className="font-bold">{rupiah(t.total_amount)}</span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>

          <div className="card hidden overflow-hidden md:block">
            <table className="w-full text-left text-[15px]">
              <thead>
                <tr className="border-b" style={{ borderColor: "var(--line)" }}>
                  <th className="p-3 font-bold">Pesanan</th>
                  <th className="p-3 font-bold">Paket</th>
                  <th className="p-3 text-right font-bold">Total</th>
                  <th className="p-3 font-bold">Status</th>
                  <th className="p-3 font-bold">Waktu</th>
                  <th className="p-3" aria-label="Aksi" />
                </tr>
              </thead>
              <tbody>
                {data.items.map((t) => (
                  <tr
                    key={t.order_id}
                    className="cursor-pointer border-b last:border-0"
                    style={{ borderColor: "var(--line)" }}
                    onClick={() => navigate(`/admin/transaksi/${t.order_id}`)}
                  >
                    <td className="max-w-[160px] truncate p-3 font-mono text-sm">{t.order_id}</td>
                    <td className="max-w-[200px] truncate p-3 font-semibold">{t.package_name || "-"}</td>
                    <td className="p-3 text-right font-bold whitespace-nowrap">{rupiah(t.total_amount)}</td>
                    <td className="p-3">
                      <StatusBadge status={t.status} />
                    </td>
                    <td className="p-3 whitespace-nowrap" style={{ color: "var(--text-2)" }}>
                      {formatTanggal(t.created_at)}
                    </td>
                    <td className="p-3 text-right">
                      <ArrowRight size={18} style={{ color: "var(--text-2)" }} aria-hidden="true" />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <nav className="flex items-center justify-between gap-3" aria-label="Paginasi transaksi">
            <p className="text-sm" style={{ color: "var(--text-2)" }}>
              Halaman {data.page} dari {totalHalaman} ({data.total} transaksi)
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                className="btn btn-secondary min-h-[44px] px-3"
                disabled={data.page <= 1}
                onClick={() => ubah({ page: data.page - 1 })}
                aria-label="Halaman sebelumnya"
              >
                <CaretLeft size={18} />
                <span className="hidden sm:inline">Sebelumnya</span>
              </button>
              <button
                type="button"
                className="btn btn-secondary min-h-[44px] px-3"
                disabled={data.page >= totalHalaman}
                onClick={() => ubah({ page: data.page + 1 })}
                aria-label="Halaman berikutnya"
              >
                <span className="hidden sm:inline">Berikutnya</span> <CaretRight size={18} />
              </button>
            </div>
          </nav>
        </>
      )}
    </div>
  );
}
