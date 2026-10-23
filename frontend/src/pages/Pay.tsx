import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { QRCodeSVG } from "qrcode.react";
import {
  ArrowClockwise,
  ArrowLeft,
  CheckCircle,
  Clock,
  QrCode,
  WarningCircle,
  XCircle,
} from "@phosphor-icons/react";
import { ApiError, api } from "../lib/api";
import { rupiah } from "../lib/format";

type Paket = {
  id: number;
  name: string;
  validity_label: string;
  rate_limit: string;
};

type OrderDetail = {
  order_id: string;
  status: string;
  package: Paket;
  total_amount: number;
  expires_at: string | null;
  paid_at: string | null;
  qr_payload?: string;
};

function sisaDetik(expiresAt: string | null, sekarang: number): number {
  if (!expiresAt) return 0;
  return Math.max(0, Math.floor((new Date(expiresAt).getTime() - sekarang) / 1000));
}

function formatCountdown(detik: number): string {
  const m = Math.floor(detik / 60);
  const s = detik % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export default function Pay() {
  const { orderId } = useParams<{ orderId: string }>();
  const navigate = useNavigate();
  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [batalkan, setBatalkan] = useState(false);
  const [sekarang, setSekarang] = useState(() => Date.now());
  const timer = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!orderId) return;
    try {
      const data = await api<OrderDetail>(`/api/orders/${orderId}`);
      setOrder(data);
      setError(null);
      if (data.status === "paid") {
        navigate(`/voucher/${orderId}`, { replace: true });
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        setError("Pesanan tidak ditemukan");
      } else {
        setError(err instanceof ApiError ? err.message : "Gagal memuat pesanan");
      }
    }
  }, [orderId, navigate]);

  useEffect(() => {
    // Muat awal dijadwalkan async agar efek tidak memicu render beruntun.
    const kick = window.setTimeout(() => {
      void load();
    }, 0);
    timer.current = window.setInterval(() => {
      setSekarang(Date.now());
    }, 1000);
    const poll = window.setInterval(() => {
      void load();
    }, 5000);
    return () => {
      window.clearTimeout(kick);
      if (timer.current) window.clearInterval(timer.current);
      window.clearInterval(poll);
    };
  }, [load]);

  const batal = useCallback(async () => {
    if (!orderId || batalkan) return;
    setBatalkan(true);
    try {
      await api(`/api/orders/${orderId}/cancel`, { method: "POST", body: {} });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal membatalkan");
    } finally {
      setBatalkan(false);
    }
  }, [orderId, batalkan, load]);

  const sisa = sisaDetik(order?.expires_at ?? null, sekarang);
  const selesai = order && ["expired", "cancelled", "failed"].includes(order.status);

  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-md flex-col px-4 py-6">
      <header className="mb-4 flex items-center gap-2">
        <Link
          to="/"
          className="btn btn-secondary h-11 min-h-[44px] shrink-0 px-3 text-sm"
          aria-label="Kembali ke katalog"
        >
          <ArrowLeft size={18} /> Katalog
        </Link>
        <h1 className="min-w-0 flex-1 truncate text-lg font-bold tracking-tight">
          Pembayaran QRIS
        </h1>
      </header>

      {!order && !error && (
        <div className="grid flex-1 content-start gap-4" aria-label="Memuat pembayaran">
          <div className="card grid place-items-center p-6">
            <div className="skeleton h-[240px] w-[240px] rounded-[10px]" />
          </div>
          <div className="skeleton h-10 w-1/2 justify-self-center" />
          <div className="skeleton h-6 w-1/3 justify-self-center" />
        </div>
      )}

      {error && !order && (
        <div className="card grid flex-1 place-items-center gap-3 p-8 text-center">
          <WarningCircle size={40} style={{ color: "var(--danger)" }} />
          <p className="text-lg font-bold tracking-tight">Pesanan tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary w-full" onClick={() => void load()}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
        </div>
      )}

      {order && selesai && (
        <div
          className="card grid flex-1 content-center place-items-center gap-3 p-8 text-center"
          role="status"
          aria-live="polite"
        >
          {order.status === "cancelled" ? (
            <XCircle size={44} style={{ color: "var(--text-2)" }} />
          ) : (
            <Clock size={44} style={{ color: "var(--danger)" }} />
          )}
          <p className="text-lg font-bold tracking-tight">
            {order.status === "cancelled"
              ? "Pesanan dibatalkan"
              : order.status === "failed"
                ? "Pesanan gagal diproses"
                : "Waktu pembayaran habis"}
          </p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {order.status === "cancelled"
              ? "Pesanan ini sudah dibatalkan. Buat pesanan baru bila masih butuh voucher."
              : order.status === "failed"
                ? "Terjadi kendala saat menerbitkan voucher. Hubungi admin atau coba lagi."
                : "Kode QR sudah kedaluwarsa dan tidak bisa dipakai lagi."}
          </p>
          <Link to="/" className="btn btn-primary w-full">
            Buat pesanan baru
          </Link>
        </div>
      )}

      {order && !selesai && (
        <main className="grid flex-1 content-start gap-4" aria-label="Bayar pesanan">
          <p
            className="truncate text-center text-sm font-semibold"
            style={{ color: "var(--text-2)" }}
          >
            {order.package.name} &middot; {order.package.validity_label}
          </p>
          <div className="card grid place-items-center gap-3 p-6">
            <span className="flex items-center gap-2 text-sm" style={{ color: "var(--text-2)" }}>
              <QrCode size={18} /> Pindai dengan aplikasi pembayaran
            </span>
            <span
              className="grid place-items-center rounded-[10px] bg-white p-4"
              style={{ width: "min(72vw, 280px)", height: "min(72vw, 280px)" }}
            >
              <QRCodeSVG
                value={order.qr_payload ?? ""}
                size={248}
                style={{ width: "100%", height: "auto", maxWidth: 248 }}
                bgColor="#ffffff"
                fgColor="#000000"
                level="M"
                includeMargin={false}
                role="img"
                aria-label={`Kode QR pembayaran ${rupiah(order.total_amount)}`}
              />
            </span>
          </div>
          <p className="text-center text-3xl font-extrabold tracking-tight">
            {rupiah(order.total_amount)}
          </p>
          <p
            className="flex items-center justify-center gap-2 font-mono text-xl font-bold tabular-nums"
            aria-live="off"
            aria-label={`Sisa waktu ${formatCountdown(sisa)}`}
          >
            <Clock size={20} style={{ color: "var(--accent)" }} />
            {formatCountdown(sisa)}
          </p>
          <p
            className="flex items-center justify-center gap-2 text-center text-sm"
            style={{ color: "var(--text-2)" }}
            role="status"
            aria-live="polite"
          >
            <CheckCircle size={18} style={{ color: "var(--accent)" }} />
            Menunggu pembayaran, status diperbarui otomatis
          </p>
          {error && (
            <p className="text-center text-sm" style={{ color: "var(--danger)" }} role="alert">
              {error}
            </p>
          )}
          <div
            className="sticky bottom-0 grid gap-2 bg-transparent pt-2 pb-[env(safe-area-inset-bottom)]"
          >
            <button
              type="button"
              className="btn btn-secondary min-h-[48px] w-full"
              onClick={() => void batal()}
              disabled={batalkan}
            >
              {batalkan ? "Membatalkan..." : "Batalkan"}
            </button>
          </div>
        </main>
      )}
    </div>
  );
}
