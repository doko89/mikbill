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

// parseDurasiMikrotik mengubah format durasi MikroTik menjadi detik.
// Menerima satuan w/d/h/m/s yang boleh digabung ("1d12h") atau angka
// detik polos ("3600"). Kosong atau tidak valid menghasilkan null.
export function parseDurasiMikrotik(teks: string): number | null {
  const s = teks.trim();
  if (s === "") return null;
  if (/^\d+$/.test(s)) return Number(s);
  const pola = /(\d+)\s*([wdhms])/gi;
  let termakan = "";
  let total = 0;
  for (;;) {
    const cocok = pola.exec(s);
    if (cocok === null) break;
    termakan += cocok[0];
    const nilai = Number(cocok[1]);
    switch (cocok[2].toLowerCase()) {
      case "w":
        total += nilai * 604800;
        break;
      case "d":
        total += nilai * 86400;
        break;
      case "h":
        total += nilai * 3600;
        break;
      case "m":
        total += nilai * 60;
        break;
      case "s":
        total += nilai;
        break;
    }
  }
  if (termakan.replace(/\s+/g, "") !== s.replace(/\s+/g, "")) return null;
  return total;
}
