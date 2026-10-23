import { labelStatus } from "../lib/status";

function warna(status: string): { color: string; bg: string } {
  if (status === "paid") return { color: "var(--accent)", bg: "var(--surface-2)" };
  if (status === "failed" || status === "expired")
    return { color: "var(--danger)", bg: "var(--surface-2)" };
  return { color: "var(--text-2)", bg: "var(--surface-2)" };
}

export function StatusBadge({ status }: { status: string }) {
  const w = warna(status);
  return (
    <span
      className="inline-flex min-h-[28px] items-center rounded-full px-3 text-[13px] font-bold whitespace-nowrap"
      style={{ color: w.color, background: w.bg }}
    >
      {labelStatus(status)}
    </span>
  );
}
