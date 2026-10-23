import { useSession } from "../store/session";
import { CSRF_HEADER } from "./config";

export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

type ApiOptions = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
};

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

// api memanggil backend dan otomatis menyertakan header CSRF
// untuk request mutasi ke /api/admin/*.
export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (MUTATING.has(method) && path.startsWith("/api/admin")) {
    const csrf = useSession.getState().csrfToken;
    if (csrf) headers[CSRF_HEADER] = csrf;
  }
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: "same-origin",
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    });
  } catch {
    throw new ApiError("Tidak dapat menghubungi server", 0);
  }
  let json: { ok: boolean; data?: T; error?: string; code?: string };
  try {
    json = (await res.json()) as typeof json;
  } catch {
    throw new ApiError("Respons server tidak valid", res.status);
  }
  if (!json.ok) {
    throw new ApiError(json.error || "Kesalahan server", res.status, json.code);
  }
  return json.data as T;
}
