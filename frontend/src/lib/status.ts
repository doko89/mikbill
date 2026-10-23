// Helper status order dan format tanggal tampilan.
const LABEL: Record<string, string> = {
  pending: "Menunggu bayar",
  processing: "Memproses",
  paid: "Lunas",
  expired: "Kedaluarsa",
  cancelled: "Dibatalkan",
  failed: "Gagal",
};

export function labelStatus(status: string): string {
  return LABEL[status] ?? status;
}

export function formatTanggal(iso: string | null): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleString("id-ID", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}
