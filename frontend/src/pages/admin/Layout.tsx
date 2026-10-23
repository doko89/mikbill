import { NavLink, Outlet, useNavigate } from "react-router-dom";
import {
  GearSix,
  House,
  Network,
  Package,
  Receipt,
  SignOut,
  WifiHigh,
} from "@phosphor-icons/react";
import { useSession } from "../../store/session";

const NAV = [
  { to: "/admin", label: "Ringkasan", icon: House, end: true },
  { to: "/admin/transaksi", label: "Transaksi", icon: Receipt, end: false },
  { to: "/admin/paket", label: "Paket", icon: Package, end: false },
  { to: "/admin/router", label: "Router", icon: Network, end: false },
  { to: "/admin/pengaturan", label: "Pengaturan", icon: GearSix, end: false },
];

export default function AdminLayout() {
  const navigate = useNavigate();
  const admin = useSession((s) => s.admin);
  const logout = useSession((s) => s.logout);

  async function keluar() {
    await logout();
    navigate("/admin/login", { replace: true });
  }

  return (
    <div className="min-h-[100dvh] md:flex" style={{ background: "var(--surface)" }}>
      {/* Sidebar desktop */}
      <aside
        className="hidden w-60 shrink-0 flex-col border-r p-4 md:flex"
        style={{ borderColor: "var(--line)" }}
      >
        <div className="mb-6 flex items-center gap-2 px-2 pt-2">
          <span
            className="flex h-9 w-9 items-center justify-center rounded-[10px]"
            style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
          >
            <WifiHigh size={20} weight="bold" />
          </span>
          <span className="text-lg font-bold tracking-tight">Mikbil</span>
        </div>
        <nav className="grid gap-1" aria-label="Navigasi admin">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className="flex min-h-[44px] items-center gap-3 rounded-[10px] px-3 text-[15px] font-semibold"
              style={({ isActive }) =>
                isActive
                  ? { background: "var(--surface-2)", color: "var(--accent)" }
                  : { color: "var(--text-2)" }
              }
            >
              <item.icon size={20} />
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="mt-auto grid gap-2 px-2 pb-2">
          <p className="text-sm" style={{ color: "var(--text-2)" }}>
            Masuk sebagai <span className="font-semibold" style={{ color: "var(--text)" }}>{admin?.name}</span>
          </p>
          <button type="button" className="btn btn-secondary w-full" onClick={keluar}>
            <SignOut size={18} /> Keluar
          </button>
        </div>
      </aside>

      {/* Konten */}
      <div className="flex-1 pb-24 md:pb-0">
        <main className="mx-auto w-full max-w-6xl px-4 py-6">
          <Outlet />
        </main>
      </div>

      {/* Tab bawah mobile */}
      <nav
        className="fixed inset-x-0 bottom-0 grid grid-cols-5 border-t md:hidden"
        style={{
          borderColor: "var(--line)",
          background: "var(--surface)",
          paddingBottom: "env(safe-area-inset-bottom)",
        }}
        aria-label="Navigasi admin"
      >
        {NAV.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className="flex min-h-[56px] flex-col items-center justify-center gap-0.5 text-[11px] font-semibold"
            style={({ isActive }) => ({ color: isActive ? "var(--accent)" : "var(--text-2)" })}
          >
            <item.icon size={22} />
            {item.label}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
