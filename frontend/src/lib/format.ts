// Helper format tampilan: rupiah dengan pemisah ribuan.
export function rupiah(n: number): string {
  return "Rp" + n.toLocaleString("id-ID");
}
