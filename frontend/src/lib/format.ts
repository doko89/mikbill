// Helper format tampilan: rupiah dengan pemisah ribuan.
export function rupiah(n: number): string {
  return "Rp" + n.toLocaleString("id-ID");
}

// durasiMikrotik mengubah detik menjadi format durasi MikroTik.
// Contoh: 43200 -> "12h", 2592000 -> "30d", 0 -> "tanpa batas".
export function durasiMikrotik(detik: number): string {
  if (!Number.isFinite(detik) || detik <= 0) return "tanpa batas";
  let sisa = Math.floor(detik);
  const bagian: string[] = [];
  const satuan: [number, string][] = [
    [86400, "d"],
    [3600, "h"],
    [60, "m"],
    [1, "s"],
  ];
  for (const [nilai, simbol] of satuan) {
    const jumlah = Math.floor(sisa / nilai);
    if (jumlah > 0) {
      bagian.push(`${jumlah}${simbol}`);
      sisa -= jumlah * nilai;
    }
  }
  return bagian.join("") || "0s";
}
