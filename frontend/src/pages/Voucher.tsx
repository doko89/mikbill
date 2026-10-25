import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ArrowClockwise,
  CheckCircle,
  Clock,
  Copy,
  DownloadSimple,
  Keyboard,
  Ticket,
  WarningCircle,
  WifiHigh,
} from "@phosphor-icons/react";
import { ApiError, api } from "../lib/api";
import { rupiah } from "../lib/format";
import { useTheme } from "../store/theme";

type Paket = {
  id: number;
  name: string;
  validity_label: string;
};

type Voucher = {
  username: string;
  password: string;
  profile_name: string;
};

type OrderDetail = {
  order_id: string;
  status: string;
  package: Paket;
  total_amount: number;
  expires_at: string | null;
  paid_at: string | null;
  voucher?: Voucher;
  login_url?: string;
};

type FormatGambar = "png" | "jpeg";

async function salinTeks(teks: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(teks);
    return true;
  } catch {
    // Fallback untuk konteks non-HTTPS (mode server lokal).
    try {
      const area = document.createElement("textarea");
      area.value = teks;
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.appendChild(area);
      area.select();
      const ok = document.execCommand("copy");
      document.body.removeChild(area);
      return ok;
    } catch {
      return false;
    }
  }
}

// token membaca variabel warna tema agar gambar unduhan ikut token desain.
function token(nama: string, ganti: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(nama).trim();
  return v || ganti;
}

// unduhVoucher merender kartu voucher ke canvas lalu mengunduhnya.
function unduhVoucher(order: OrderDetail, format: FormatGambar) {
  const kode = order.voucher?.password ?? "";
  const aksen = token("--accent", "#059669");
  const teks = token("--text", "#18181b");
  const teks2 = token("--text-2", "#52525b");
  const alas = token("--surface", "#ffffff");
  const alas2 = token("--surface-2", "#f4f4f5");
  const W = 900;
  const H = 1150;
  const canvas = document.createElement("canvas");
  canvas.width = W;
  canvas.height = H;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  ctx.fillStyle = alas;
  ctx.fillRect(0, 0, W, H);
  ctx.strokeStyle = aksen;
  ctx.lineWidth = 10;
  ctx.strokeRect(24, 24, W - 48, H - 48);

  ctx.textAlign = "center";
  ctx.fillStyle = aksen;
  ctx.font = "800 64px Outfit, system-ui, sans-serif";
  ctx.fillText(useTheme.getState().brand_name, W / 2, 140, W - 160);
  ctx.fillStyle = teks2;
  ctx.font = "500 34px Outfit, system-ui, sans-serif";
  ctx.fillText("Voucher WiFi", W / 2, 195);

  ctx.fillStyle = teks;
  ctx.font = "700 44px Outfit, system-ui, sans-serif";
  ctx.fillText(order.package.name, W / 2, 300, W - 160);
  ctx.fillStyle = teks2;
  ctx.font = "400 32px Outfit, system-ui, sans-serif";
  ctx.fillText(`Masa berlaku ${order.package.validity_label}`, W / 2, 350);

  ctx.fillStyle = alas2;
  const boxY = 420;
  ctx.fillRect(90, boxY, W - 180, 230);
  ctx.fillStyle = teks;
  const ukuran = kode.length > 10 ? 84 : 110;
  ctx.font = `700 ${ukuran}px ui-monospace, Menlo, monospace`;
  ctx.fillText(kode.split("").join(" "), W / 2, boxY + 150, W - 220);

  ctx.fillStyle = teks2;
  ctx.font = "400 30px Outfit, system-ui, sans-serif";
  ctx.fillText("Kode dipakai sebagai nama pengguna", W / 2, 730);
  ctx.fillText("dan kata sandi saat login WiFi.", W / 2, 775);

  ctx.fillStyle = teks;
  ctx.font = "700 40px Outfit, system-ui, sans-serif";
  ctx.fillText(rupiah(order.total_amount), W / 2, 880);

  if (order.login_url) {
    ctx.fillStyle = teks2;
    ctx.font = "400 26px ui-monospace, Menlo, monospace";
    ctx.fillText(order.login_url, W / 2, 980, W - 140);
  }
  ctx.fillStyle = teks2;
  ctx.font = "400 26px Outfit, system-ui, sans-serif";
  ctx.fillText(`Pesanan ${order.order_id}`, W / 2, 1050);

  const mime = format === "png" ? "image/png" : "image/jpeg";
  const unduh = (url: string) => {
    const a = document.createElement("a");
    a.href = url;
    a.download = `voucher-${order.order_id}.${format === "png" ? "png" : "jpg"}`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  };
  if (typeof canvas.toBlob === "function") {
    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        const url = URL.createObjectURL(blob);
        unduh(url);
        window.setTimeout(() => URL.revokeObjectURL(url), 5000);
      },
      mime,
      0.92,
    );
  } else {
    unduh(canvas.toDataURL(mime, 0.92));
  }
}

export default function Voucher() {
  const { orderId } = useParams<{ orderId: string }>();
  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tersalin, setTersalin] = useState(false);
  const [format, setFormat] = useState<FormatGambar>("png");
  const toastTimer = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!orderId) return;
    try {
      const data = await api<OrderDetail>(`/api/orders/${orderId}`);
      setOrder(data);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        setError("Pesanan tidak ditemukan");
      } else {
        setError(err instanceof ApiError ? err.message : "Gagal memuat voucher");
      }
    }
  }, [orderId]);

  useEffect(() => {
    const kick = window.setTimeout(() => {
      void load();
    }, 0);
    return () => window.clearTimeout(kick);
  }, [load]);

  // Poll saat pembayaran belum lunas agar halaman tampil otomatis.
  useEffect(() => {
    if (!order || order.status === "paid") return;
    if (order.status !== "pending" && order.status !== "processing") return;
    const poll = window.setInterval(() => {
      void load();
    }, 5000);
    return () => window.clearInterval(poll);
  }, [order, load]);

  useEffect(
    () => () => {
      if (toastTimer.current) window.clearTimeout(toastTimer.current);
    },
    [],
  );

  const salin = useCallback(async () => {
    const kode = order?.voucher?.password;
    if (!kode) return;
    const ok = await salinTeks(kode);
    setTersalin(ok);
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setTersalin(false), 2000);
  }, [order]);

  const menunggu =
    order && (order.status === "pending" || order.status === "processing");
  const selesaiGagal =
    order && ["expired", "cancelled", "failed"].includes(order.status);

  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-md flex-col px-4 py-6">
      <header className="mb-4 flex items-center gap-2">
        <span
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px]"
          style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
        >
          <Ticket size={22} weight="bold" />
        </span>
        <h1 className="text-lg font-bold tracking-tight">Voucher WiFi</h1>
      </header>

      {!order && !error && (
        <div className="grid flex-1 content-start gap-4" aria-label="Memuat voucher">
          <div className="card grid place-items-center gap-3 p-8">
            <div className="skeleton h-14 w-3/4" />
            <div className="skeleton h-10 w-full" />
          </div>
          <div className="skeleton h-12 w-full" />
        </div>
      )}

      {error && !order && (
        <div className="card grid flex-1 content-center place-items-center gap-3 p-8 text-center">
          <WarningCircle size={40} style={{ color: "var(--danger)" }} />
          <p className="text-lg font-bold tracking-tight">Voucher tidak dapat dimuat</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            {error}
          </p>
          <button type="button" className="btn btn-primary w-full" onClick={() => void load()}>
            <ArrowClockwise size={18} /> Coba lagi
          </button>
          <Link to="/" className="btn btn-secondary w-full">
            Kembali ke katalog
          </Link>
        </div>
      )}

      {menunggu && (
        <div
          className="card grid flex-1 content-center place-items-center gap-3 p-8 text-center"
          role="status"
          aria-live="polite"
        >
          <Clock size={44} style={{ color: "var(--accent)" }} />
          <p className="text-lg font-bold tracking-tight">Menunggu pembayaran</p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Voucher terbit otomatis setelah pembayaran lunas. Halaman ini diperbarui sendiri.
          </p>
          <Link to={`/bayar/${orderId}`} className="btn btn-primary w-full">
            Buka halaman bayar
          </Link>
        </div>
      )}

      {selesaiGagal && (
        <div className="card grid flex-1 content-center place-items-center gap-3 p-8 text-center">
          <WarningCircle size={44} style={{ color: "var(--danger)" }} />
          <p className="text-lg font-bold tracking-tight">
            {order.status === "cancelled"
              ? "Pesanan dibatalkan"
              : order.status === "failed"
                ? "Pesanan gagal diproses"
                : "Waktu pembayaran habis"}
          </p>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Voucher tidak tersedia untuk pesanan ini. Buat pesanan baru bila masih butuh voucher.
          </p>
          <Link to="/" className="btn btn-primary w-full">
            Buat pesanan baru
          </Link>
        </div>
      )}

      {order?.status === "paid" && order.voucher && (
        <main className="grid flex-1 content-start gap-4" aria-label="Voucher lunas">
          <p
            className="flex items-center justify-center gap-2 text-center text-sm font-semibold"
            style={{ color: "var(--accent)" }}
            role="status"
            aria-live="polite"
          >
            <CheckCircle size={18} /> Pembayaran lunas
          </p>

          <section className="card grid gap-3 p-6 text-center" aria-label="Kode voucher">
            <p className="text-sm font-semibold" style={{ color: "var(--text-2)" }}>
              Kode voucher
            </p>
            <button
              type="button"
              onClick={() => void salin()}
              className="rounded-[10px] px-2 py-3 font-mono text-3xl font-bold tracking-[0.2em] break-all select-all md:text-4xl"
              style={{ background: "var(--surface-2)", color: "var(--text)" }}
              aria-label={`Salin kode ${order.voucher.password}`}
              title="Ketuk untuk menyalin"
            >
              {order.voucher.password}
            </button>
            <button
              type="button"
              className="btn btn-secondary min-h-[48px] w-full"
              onClick={() => void salin()}
            >
              <Copy size={18} /> Salin
            </button>
            <p className="text-sm" style={{ color: "var(--text-2)" }}>
              Kode dipakai sebagai nama pengguna dan kata sandi.
            </p>
          </section>

          <section className="card grid gap-2 p-5" aria-label="Ringkasan paket">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm" style={{ color: "var(--text-2)" }}>
                Paket
              </span>
              <span className="text-right font-bold">{order.package.name}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm" style={{ color: "var(--text-2)" }}>
                Masa berlaku
              </span>
              <span className="font-bold">{order.package.validity_label}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm" style={{ color: "var(--text-2)" }}>
                Total bayar
              </span>
              <span className="font-bold">{rupiah(order.total_amount)}</span>
            </div>
          </section>

          <section className="card grid gap-3 p-5" aria-label="Unduh voucher">
            <div
              className="grid grid-cols-2 gap-1 rounded-[10px] p-1"
              style={{ background: "var(--surface-2)" }}
              role="group"
              aria-label="Format gambar"
            >
              {(["png", "jpeg"] as FormatGambar[]).map((f) => (
                <button
                  key={f}
                  type="button"
                  onClick={() => setFormat(f)}
                  aria-pressed={format === f}
                  className="min-h-[44px] rounded-[8px] text-[15px] font-bold"
                  style={
                    format === f
                      ? { background: "var(--surface)", color: "var(--text)" }
                      : { color: "var(--text-2)" }
                  }
                >
                  {f === "png" ? "PNG" : "JPG"}
                </button>
              ))}
            </div>
            <button
              type="button"
              className="btn btn-secondary min-h-[48px] w-full"
              onClick={() => unduhVoucher(order, format)}
            >
              <DownloadSimple size={18} /> Unduh {format === "png" ? "PNG" : "JPG"}
            </button>
          </section>

          <Link
            to="/bantuan"
            className="flex min-h-[44px] items-center justify-center gap-2 text-[15px] font-semibold"
            style={{ color: "var(--accent)" }}
          >
            <Keyboard size={18} /> Cara login manual
          </Link>

          <div
            className="sticky bottom-0 grid gap-2 pt-2 pb-[env(safe-area-inset-bottom)]"
            style={{ background: "var(--surface)" }}
          >
            {order.login_url ? (
              <a href={order.login_url} className="btn btn-primary min-h-[52px] w-full">
                <WifiHigh size={20} weight="bold" /> Login WiFi
              </a>
            ) : (
              <p className="text-center text-sm" style={{ color: "var(--danger)" }} role="alert">
                Alamat login hotspot belum dikonfigurasi. Gunakan cara login manual.
              </p>
            )}
          </div>
        </main>
      )}

      {tersalin && (
        <p
          className="fixed bottom-6 left-1/2 z-50 -translate-x-1/2 rounded-[10px] px-4 py-2 text-sm font-bold whitespace-nowrap"
          style={{ background: "var(--text)", color: "var(--surface)" }}
          role="status"
        >
          Kode tersalin
        </p>
      )}
    </div>
  );
}
