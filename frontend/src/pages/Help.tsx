import { Link } from "react-router-dom";
import { ArrowLeft, Keyboard, QrCode, WifiHigh } from "@phosphor-icons/react";
import { useTheme } from "../store/theme";

const caraBeli = (merek: string) => [
  `Hubungkan HP ke WiFi hotspot, lalu buka halaman katalog ${merek}.`,
  "Pilih paket sesuai kebutuhan, lalu tekan tombol Pilih.",
  "Pindai kode QRIS yang tampil dan selesaikan pembayaran sebelum waktu habis.",
  "Kode voucher tampil di layar. Salin atau unduh sebagai gambar agar tidak hilang.",
];

const LOGIN_MANUAL = [
  "Buka halaman login hotspot (biasanya muncul otomatis saat membuka internet).",
  "Masukkan kode voucher sebagai nama pengguna dan kata sandi.",
  "Tekan tombol login dan tunggu sampai status tersambung.",
  "Bila halaman login tidak muncul, buka alamat yang tertera di voucher.",
];

export default function Bantuan() {
  const brand_name = useTheme((s) => s.brand_name);
  const langkahBeli = caraBeli(brand_name);
  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-md flex-col px-4 py-6">
      <header className="mb-5 flex items-center gap-3">
        <Link
          to="/"
          className="btn btn-secondary h-11 min-h-[44px] shrink-0 px-3 text-sm"
          aria-label="Kembali ke katalog"
        >
          <ArrowLeft size={18} /> Katalog
        </Link>
        <h1 className="text-xl font-bold tracking-tight">Bantuan</h1>
      </header>

      <main className="grid gap-4">
        <section className="card p-5" aria-labelledby="cara-beli">
          <h2 id="cara-beli" className="flex items-center gap-2 text-lg font-bold tracking-tight">
            <QrCode size={22} style={{ color: "var(--accent)" }} /> Cara beli voucher
          </h2>
          <ol className="mt-3 grid gap-2 text-[15px] leading-relaxed">
            {langkahBeli.map((langkah, i) => (
              <li key={langkah} className="flex gap-3">
                <span
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-sm font-bold"
                  style={{ background: "var(--surface-2)", color: "var(--accent)" }}
                  aria-hidden="true"
                >
                  {i + 1}
                </span>
                <span>{langkah}</span>
              </li>
            ))}
          </ol>
        </section>

        <section className="card p-5" aria-labelledby="login-manual">
          <h2 id="login-manual" className="flex items-center gap-2 text-lg font-bold tracking-tight">
            <Keyboard size={22} style={{ color: "var(--accent)" }} /> Cara login manual
          </h2>
          <ol className="mt-3 grid gap-2 text-[15px] leading-relaxed">
            {LOGIN_MANUAL.map((langkah, i) => (
              <li key={langkah} className="flex gap-3">
                <span
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-sm font-bold"
                  style={{ background: "var(--surface-2)", color: "var(--accent)" }}
                  aria-hidden="true"
                >
                  {i + 1}
                </span>
                <span>{langkah}</span>
              </li>
            ))}
          </ol>
        </section>

        <section className="card p-5" aria-labelledby="kendala">
          <h2 id="kendala" className="flex items-center gap-2 text-lg font-bold tracking-tight">
            <WifiHigh size={22} style={{ color: "var(--accent)" }} /> Bila ada kendala
          </h2>
          <p className="mt-2 text-[15px] leading-relaxed" style={{ color: "var(--text-2)" }}>
            Pastikan kode voucher diketik sama persis, termasuk huruf besar dan kecil. Bila kode hilang
            sebelum dipakai, hubungi pemilik hotspot dengan menunjukkan bukti pembayaran.
          </p>
        </section>
      </main>
    </div>
  );
}
