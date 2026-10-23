import { Link } from "react-router-dom";

export default function NotFound() {
  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-md flex-col justify-center px-4 py-10">
      <div className="card p-6 text-center">
        <p className="text-4xl font-extrabold tracking-tight">404</p>
        <p className="mt-2 font-semibold">Halaman tidak ditemukan</p>
        <p className="hint">Alamat yang Anda buka tidak tersedia.</p>
        <Link to="/" className="btn btn-primary mt-4 w-full">
          Kembali ke beranda
        </Link>
      </div>
    </div>
  );
}
