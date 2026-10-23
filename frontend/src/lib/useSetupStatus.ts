import { useEffect, useState } from "react";
import { ApiError, api } from "./api";

export type SetupStatus = {
  setup_completed: boolean;
  has_admin: boolean;
};

export function useSetupStatus() {
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    api<SetupStatus>("/api/setup/status")
      .then((s) => {
        if (alive) setStatus(s);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof ApiError ? err.message : "Gagal memuat status");
      });
    return () => {
      alive = false;
    };
  }, []);

  return { status, error };
}
