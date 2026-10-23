import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ArrowClockwise,
  ArrowLeft,
  Ticket,
  WarningCircle,
} from "@phosphor-icons/react";
import { ApiError, api } from "../../lib/api";
import { rupiah } from "../../lib/format";
import { PageSkeleton } from "../../components/ui";
import { StatusBadge } from "../../components/orders";
import { formatTanggal } from "../../lib/status";

type Paket = {
  name: string;
  validity_label: string;
  profile_name: string;
};

type Voucher = {
  username: string;
  password: string;
  profile_name: string;
};

type Detail = {
  order_id: string;
  status: string;
  package: Paket;
  base_amount: number;
  unique_code: number;
  total_amount: number;
  kriskris_invoice_id: string;
  expires_at: string | null;
  paid_at: string | null;
  failed_reason: string;
  created_at: string;
  voucher?: Voucher;
  login_url?: string;
};

function Baris({ label, isi, mono }: { label: string; isi: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="shrink-0 text-sm" style={{ color: "var(--text-2)" }}>
        {label}
      </span>
      <span className={`text-right font-semibold break-all ${mono ? "font-mono text-sm" : ""}`}>
        {isi}
      </span>
    </div>
  );
}

export default function TransactionDetail() {
  const { orderId } = useParams<{ orderId: string }>();
  const [data, setData] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const [retryError, setRetryError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!orderId) return;
    try {
      const res = await api<Detail>(`/api/admin/orders/${orderId}`);
      setData(res);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat detail");
    }
  }, [orderId]);

  useEffect(() => {
    const kick = window.setTimeout(() => {
      void load();
    }, 0);
    return () => window.clearTimeout(kick);
  }, [load]);

  const retry = useCallback(async () => {
    if (!orderId || retrying) return;
    setRetrying(true);
    setRetryError(null);
    try {
      const res = await api<Detail>(`/api/admin/orders/${orderId}/retry`, {
        method: "POST",
        body: {},
      });
      setData(res);
    } catch (err) {
      setRetryError(err instanceof ApiError ? err.message : "Gagal mengulang");
    } finally {
      setRetrying(false);
    }
  }, [orderId, retrying]);

  return (
    <div className="grid gap-4">
      <header className="flex items-center gap-3">
        <Link
          to="/admin/transaksi"
          className="btn btn-secondary h-11 min-h-[44px] shrink-0 px-3 text-sm"
          aria-label="Kembali ke daftar transaksi"
        >
          <ArrowLeft size={18} /> Transaksi
        </Link>
        <h1 className="min-w-0 flex-1 truncate font-mono text-lg font-bold">{orderId}</h1>
      </header>

      {!data && !error && <PageSkeleton lines={5} />}

      {error && !data && (
        <div className="card grid place-items-center gap-3 p-8 text-center">
          <WarningCircle size={40} style={{ color: "var(--danger)" }} />
          <p className="font-bold">Detail tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary" onClick={() => void load()}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
        </div>
      )}

      {data && (
        <main className="grid gap-4" aria-label="Detail transaksi">
          <section className="card grid gap-3 p-5" aria-label="Status pesanan">
            <div className="flex items-center justify-between gap-3">
              <StatusBadge status={data.status} />
              <span className="text-sm" style={{ color: "var(--text-2)" }}>
                {formatTanggal(data.created_at)}
              </span>
            </div>
            {data.status === "failed" && (
              <div className="grid gap-3">
                <p className="text-sm" style={{ color: "var(--danger)" }} role="alert">
                  {data.failed_reason || "Penerbitan voucher gagal."} Periksa koneksi router lalu
                  ulangi penerbitan.
                </p>
                {retryError && (
                  <p className="text-sm" style={{ color: "var(--danger)" }} role="alert">
                    {retryError}
                  </p>
                )}
                <button
                  type="button"
                  className="btn btn-primary w-full sm:w-auto"
                  onClick={() => void retry()}
                  disabled={retrying}
                >
                  <ArrowClockwise size={18} />
                  {retrying ? "Mengulang..." : "Ulangi penerbitan"}
                </button>
              </div>
            )}
          </section>

          <section className="card grid gap-3 p-5" aria-label="Ringkasan pesanan">
            <Baris label="Paket" isi={data.package.name} />
            <Baris label="Masa berlaku" isi={data.package.validity_label} />
            <Baris label="Harga dasar" isi={rupiah(data.base_amount)} />
            <Baris label="Kode unik" isi={String(data.unique_code)} />
            <Baris label="Total" isi={rupiah(data.total_amount)} />
            <Baris label="Invoice" isi={data.kriskris_invoice_id || "-"} mono />
            <Baris label="Kedaluarsa" isi={formatTanggal(data.expires_at)} />
            <Baris label="Lunas" isi={formatTanggal(data.paid_at)} />
          </section>

          {data.voucher ? (
            <section className="card grid gap-3 p-5" aria-label="Voucher terbit">
              <h2 className="flex items-center gap-2 font-bold">
                <Ticket size={20} style={{ color: "var(--accent)" }} /> Voucher terbit
              </h2>
              <Baris label="Username" isi={data.voucher.username} mono />
              <Baris label="Password" isi={data.voucher.password} mono />
              <Baris label="Profile" isi={data.voucher.profile_name} mono />
              {data.login_url && (
                <a
                  href={data.login_url}
                  className="btn btn-secondary w-full sm:w-auto"
                  target="_blank"
                  rel="noreferrer"
                >
                  Buka tautan login
                </a>
              )}
            </section>
          ) : (
            <section className="card p-5" aria-label="Voucher belum terbit">
              <p className="font-semibold">Voucher belum terbit</p>
              <p className="hint">Voucher tampil di sini setelah pembayaran lunas dan terbit di router.</p>
            </section>
          )}
        </main>
      )}
    </div>
  );
}
