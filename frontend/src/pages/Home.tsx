import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ArrowClockwise,
  Clock,
  Gauge,
  Package as PackageIcon,
  Question,
  Ticket,
  WifiHigh,
} from "@phosphor-icons/react";
import { ApiError, api } from "../lib/api";
import { rupiah } from "../lib/format";
import { useSetupStatus } from "../lib/useSetupStatus";

type Paket = {
  id: number;
  name: string;
  description: string;
  price_idr: number;
  validity_label: string;
  profile_name: string;
  rate_limit: string;
};

function SkeletonCard() {
  return (
    <div className="card grid gap-3 p-5" aria-hidden="true">
      <div className="skeleton h-6 w-2/3" />
      <div className="skeleton h-4 w-1/2" />
      <div className="skeleton h-9 w-1/3" />
      <div className="skeleton h-12 w-full" />
    </div>
  );
}

export default function Home() {
  const { status } = useSetupStatus();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [items, setItems] = useState<Paket[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [memesan, setMemesan] = useState<number | null>(null);
  const [pesanOrder, setPesanOrder] = useState<string | null>(null);

  const pilih = useCallback(
    async (p: Paket) => {
      setMemesan(p.id);
      setPesanOrder(null);
      try {
        const order = await api<{ order_id: string }>("/api/orders", {
          method: "POST",
          body: { package_id: p.id },
        });
        navigate(`/bayar/${order.order_id}`);
      } catch (err) {
        setPesanOrder(err instanceof ApiError ? err.message : "Gagal membuat pesanan");
      } finally {
        setMemesan(null);
      }
    },
    [navigate],
  );

  const fetchCatalog = useCallback(() => api<Paket[]>("/api/packages"), []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await fetchCatalog());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat katalog");
    } finally {
      setLoading(false);
    }
  }, [fetchCatalog]);

  useEffect(() => {
    let hidup = true;
    fetchCatalog()
      .then((data) => {
        if (!hidup) return;
        setItems(data);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (!hidup) return;
        setError(err instanceof ApiError ? err.message : "Gagal memuat katalog");
        setLoading(false);
      });
    return () => {
      hidup = false;
    };
  }, [fetchCatalog]);

  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-2xl flex-col px-4 py-6">
      <header className="mb-5 flex items-center gap-3">
        <span
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[10px]"
          style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
        >
          <WifiHigh size={24} weight="bold" />
        </span>
        <div className="min-w-0">
          <h1 className="text-xl font-bold tracking-tight">Mikbil</h1>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Beli voucher WiFi mandiri
          </p>
        </div>
        <Link
          to="/bantuan"
          className="btn btn-secondary ml-auto h-11 min-h-[44px] shrink-0 px-3 text-sm"
          aria-label="Bantuan"
        >
          <Question size={18} /> Bantuan
        </Link>
      </header>

      {status && !status.setup_completed && (
        <Link
          to="/setup"
          className="card mb-4 block p-4 font-semibold"
          style={{ borderColor: "var(--accent)", color: "var(--accent)" }}
        >
          Selesaikan penyiapan Mikbil lebih dulu
        </Link>
      )}

      {pesanOrder && (
        <p
          className="card mb-4 p-4 text-sm font-semibold"
          role="alert"
          style={{ borderColor: "var(--danger)", color: "var(--danger)" }}
        >
          {pesanOrder}
        </p>
      )}

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" aria-label="Memuat katalog">
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
        </div>
      ) : error ? (
        <div className="card grid flex-1 place-items-center gap-3 p-8 text-center">
          <WifiHigh size={40} style={{ color: "var(--danger)" }} />
          <p className="text-lg font-bold tracking-tight">Katalog tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary w-full sm:w-auto" onClick={load}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
        </div>
      ) : items.length === 0 ? (
        <div className="card grid flex-1 place-items-center gap-2 p-8 text-center">
          <Ticket size={40} style={{ color: "var(--accent)" }} />
          <p className="text-lg font-bold tracking-tight">Belum ada paket tersedia</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Paket voucher belum diatur oleh admin. Silakan kembali lagi nanti.
          </p>
        </div>
      ) : (
        <main className="grid grid-cols-1 gap-4 sm:grid-cols-2" aria-label="Katalog paket">
          {items.map((p) => (
            <article key={p.id} className="card flex flex-col gap-3 p-5">
              <div className="flex min-w-0 items-start gap-2">
                <PackageIcon size={22} className="mt-0.5 shrink-0" style={{ color: "var(--accent)" }} />
                <h2 className="min-w-0 flex-1 text-lg font-bold leading-snug tracking-tight">{p.name}</h2>
              </div>
              <div className="grid gap-1 text-sm" style={{ color: "var(--text-2)" }}>
                <p className="flex items-center gap-2">
                  <Clock size={16} className="shrink-0" /> {p.validity_label}
                </p>
                {p.rate_limit && (
                  <p className="flex items-center gap-2">
                    <Gauge size={16} className="shrink-0" /> {p.rate_limit}
                  </p>
                )}
              </div>
              {p.description && (
                <p className="text-sm" style={{ color: "var(--text-2)" }}>
                  {p.description}
                </p>
              )}
              <p className="mt-auto pt-1 text-2xl font-extrabold tracking-tight">{rupiah(p.price_idr)}</p>
              <button
                type="button"
                className="btn btn-primary w-full"
                onClick={() => void pilih(p)}
                disabled={memesan !== null}
              >
                {memesan === p.id ? "Memproses..." : "Pilih"}
              </button>
            </article>
          ))}
        </main>
      )}

      <footer className="flex items-center justify-center gap-4 pt-5 text-center text-sm">
        <Link to="/bantuan" className="font-semibold" style={{ color: "var(--accent)" }}>
          Cara beli dan login
        </Link>
      </footer>
    </div>
  );
}
