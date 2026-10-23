import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { useSession } from "../store/session";
import { useSetupStatus } from "../lib/useSetupStatus";
import { PageSkeleton } from "../components/ui";

// SetupGuard hanya mengizinkan wizard saat belum ada admin dan penyiapan belum selesai.
export function SetupGuard({ children }: { children: ReactNode }) {
  const { status, error } = useSetupStatus();

  if (error) {
    return (
      <div className="mx-auto w-full max-w-md px-4 py-10">
        <div className="card p-6 text-center">
          <p className="font-semibold">Tidak dapat menghubungi server</p>
          <p className="hint">{error}</p>
          <button type="button" className="btn btn-secondary mt-4 w-full" onClick={() => location.reload()}>
            Coba lagi
          </button>
        </div>
      </div>
    );
  }
  if (!status) {
    return (
      <div className="mx-auto w-full max-w-md px-4 py-10">
        <PageSkeleton lines={5} />
      </div>
    );
  }
  if (status.setup_completed || status.has_admin) {
    return <Navigate to="/admin/login" replace />;
  }
  return <>{children}</>;
}

// RequireAuth melindungi area admin; butuh sesi login.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { admin, ready, refresh } = useSession();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!ready) {
      refresh().catch(() => setFailed(true));
    }
  }, [ready, refresh]);

  if (!ready) {
    return (
      <div className="mx-auto w-full max-w-6xl px-4 py-10">
        <PageSkeleton lines={4} />
      </div>
    );
  }
  if (failed) {
    return (
      <div className="mx-auto w-full max-w-md px-4 py-10">
        <div className="card p-6 text-center">
          <p className="font-semibold">Tidak dapat menghubungi server</p>
          <button type="button" className="btn btn-secondary mt-4 w-full" onClick={() => location.reload()}>
            Coba lagi
          </button>
        </div>
      </div>
    );
  }
  if (!admin) {
    return <Navigate to="/admin/login" replace />;
  }
  return <>{children}</>;
}
