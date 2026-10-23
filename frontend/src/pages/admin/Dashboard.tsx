import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowClockwise,
  ArrowRight,
  Receipt,
  SignOut,
  WarningCircle,
} from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
import { rupiah } from "../../lib/format";
import { PageSkeleton } from "../../components/ui";
import { StatusBadge } from "../../components/orders";
import { formatTanggal } from "../../lib/status";
import { useSession } from "../../store/session";

type RingkasanItem = {
  order_id: string;
  status: string;
  package_name: string;
  total_amount: number;
  created_at: string;
};

type Ringkasan = {
  revenue_today: number;
  pending_count: number;
  failed_count: number;
  recent: RingkasanItem[];
};

type Koneksi = {
  id: number;
  name: string;
  host: string;
  is_active: boolean;
  last_test_at: string | null;
  last_test_ok: boolean | null;
  last_test_msg: string;
};

function statusRouter(aktif: Koneksi | null): string {
  if (!aktif) return "Belum ada koneksi";
  if (aktif.last_test_ok === true) return "Terhubung";
  if (aktif.last_test_ok === false) return "Gagal";
  return "Belum dicek";
}

export default function Dashboard() {
  const navigate = useNavigate();
  const admin = useSession((s) => s.admin);
  const logout = useSession((s) => s.logout);
  const [data, setData] = useState<Ringkasan | null>(null);
  const [koneksi, setKoneksi] = useState<Koneksi[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [sum, conns] = await Promise.all([
        api<Ringkasan>("/api/admin/orders/summary"),
        api<Koneksi[]>("/api/admin/connections"),
      ]);
      setData(sum);
      setKoneksi(conns);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat ringkasan");
    }
  }, []);

  useEffect(() => {
    const kick = window.setTimeout(() => {
      void load();
    }, 0);
    return () => window.clearTimeout(kick);
  }, [load]);

  async function keluar() {
    await logout();
    navigate("/admin/login", { replace: true });
  }

  const aktif = koneksi.find((k) => k.is_active) ?? null;

  return (
    <div className="grid gap-4">
      <header className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Ringkasan</h1>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Halo {admin?.name}, pantau penjualan hari ini di sini.
          </p>
        </div>
        <button type="button" className="btn btn-secondary md:hidden" onClick={keluar} aria-label="Keluar">
          <SignOut size={18} />
        </button>
      </header>

      {!data && !error && <PageSkeleton lines={4} />}

      {error && !data && (
        <div className="card grid place-items-center gap-3 p-8 text-center">
          <WarningCircle size={40} style={{ color: "var(--danger)" }} />
          <p className="font-bold">Ringkasan tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary w-full sm:w-auto" onClick={() => void load()}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
        </div>
      )}

      {data && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="card p-4">
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Omzet hari ini
              </p>
              <p className="text-2xl font-bold tracking-tight">{rupiah(data.revenue_today)}</p>
            </div>
            <Link to="/admin/transaksi?status=pending" className="card block p-4">
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Order pending
              </p>
              <p className="text-2xl font-bold tracking-tight">{data.pending_count}</p>
              {data.failed_count > 0 && (
                <p className="mt-1 text-sm font-semibold" style={{ color: "var(--danger)" }}>
                  {data.failed_count} gagal perlu perhatian
                </p>
              )}
            </Link>
            <Link to="/admin/router" className="card block p-4">
              <p className="text-sm" style={{ color: "var(--text-2)" }}>
                Status router
              </p>
              <p className="text-2xl font-bold tracking-tight">{statusRouter(aktif)}</p>
              {aktif && (
                <p className="mt-1 truncate text-sm" style={{ color: "var(--text-2)" }}>
                  {aktif.name} ({aktif.host})
                </p>
              )}
            </Link>
          </div>

          <section className="card" aria-label="Transaksi terbaru">
            <div className="flex items-center justify-between gap-3 border-b p-4" style={{ borderColor: "var(--line)" }}>
              <h2 className="flex items-center gap-2 font-bold">
                <Receipt size={20} style={{ color: "var(--accent)" }} /> Transaksi terbaru
              </h2>
              <Link
                to="/admin/transaksi"
                className="flex min-h-[44px] items-center gap-1 text-sm font-bold"
                style={{ color: "var(--accent)" }}
              >
                Lihat semua <ArrowRight size={16} />
              </Link>
            </div>
            {data.recent.length === 0 ? (
              <div className="p-5 text-center">
                <p className="font-semibold">Belum ada transaksi</p>
                <p className="hint">Transaksi terbaru tampil di sini setelah ada pembelian.</p>
              </div>
            ) : (
              <ul className="divide-y" style={{ borderColor: "var(--line)" }}>
                {data.recent.map((t) => (
                  <li key={t.order_id}>
                    <Link
                      to={`/admin/transaksi/${t.order_id}`}
                      className="flex min-h-[56px] items-center gap-3 p-4"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-bold">{t.package_name || t.order_id}</span>
                        <span className="block truncate text-sm" style={{ color: "var(--text-2)" }}>
                          {t.order_id} &middot; {formatTanggal(t.created_at)}
                        </span>
                      </span>
                      <span className="text-right">
                        <span className="block font-bold">{rupiah(t.total_amount)}</span>
                        <StatusBadge status={t.status} />
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}
