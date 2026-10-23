import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { WifiHigh } from "@phosphor-icons/react";
import { ApiError } from "../../lib/api";
import { useSession } from "../../store/session";
import { Field, FormError, TextInput } from "../../components/ui";

export default function AdminLogin() {
  const navigate = useNavigate();
  const login = useSession((s) => s.login);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(username, password);
      navigate("/admin", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login gagal");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-[100dvh] w-full max-w-md flex-col justify-center px-4 py-8">
      <div className="mb-6 flex items-center gap-3">
        <span
          className="flex h-11 w-11 items-center justify-center rounded-[10px]"
          style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
        >
          <WifiHigh size={24} weight="bold" />
        </span>
        <div>
          <h1 className="text-xl font-bold tracking-tight">Masuk Admin</h1>
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Kelola hotspot voucher Anda
          </p>
        </div>
      </div>

      <form className="card grid gap-4 p-5" onSubmit={submit}>
        <Field label="Username" name="username">
          <TextInput
            id="username"
            name="username"
            autoComplete="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
        </Field>
        <Field label="Password" name="password">
          <TextInput
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>
        <FormError message={error} />
        <button type="submit" className="btn btn-primary w-full" disabled={busy}>
          {busy ? "Memeriksa" : "Masuk"}
        </button>
      </form>

      <p className="mt-4 text-center text-sm" style={{ color: "var(--text-2)" }}>
        Belum menyiapkan Mikbil?{" "}
        <Link to="/setup" className="font-semibold" style={{ color: "var(--accent)" }}>
          Buka wizard penyiapan
        </Link>
      </p>
    </div>
  );
}
