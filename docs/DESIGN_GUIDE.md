# DESIGN GUIDE — Mikbil

> Panduan desain dan implementasi frontend. Berdampingan dengan [PRD](PRD.md) dan [tech_spec](tech_spec.md).
> Bahasa UI: Indonesia.

**Design read:** produk payment self-service mobile-first untuk pembeli hotspot Indonesia di HP entry-level, dengan bahasa trust-first, berbasis Tailwind v4 utilities + sans self-hosted + satu aksen emerald.

**DIAL:** `DESIGN_VARIANCE: 4 / MOTION_INTENSITY: 3 / VISUAL_DENSITY: 5`
Alasan: aplikasi pembayaran butuh prediktabilitas (variance rendah), gerak minimal agar terasa aman dan ringan di HP lemah (motion rendah), dan kepadatan sedang agar info penting (harga, QR, kode) muat tanpa scroll berlebihan.

## 1. Batasan Keras: Walled Garden

Pembeli membuka Mikbil **sebelum login internet**. Hanya domain Mikbil yang lolos walled garden. Akibatnya:

- **Semua aset harus dari origin Mikbil.** Dilarang: Google Fonts via link, CDN JS/CSS, gambar eksternal, analytics pihak ketiga.
- Font di-self-host via Fontsource (dibundel Vite) atau `@font-face` dengan file lokal.
- Ikon: library ikon yang dibundel (bukan CDN).
- Tidak ada request ke `kriskris.id` dari browser (semua via backend).
- Total bundle JS awal harus kecil (target < 200 KB gzip untuk rute publik). Lazy-load rute admin.

## 2. Fondasi

- **Styling:** Tailwind v4 (sudah terpasang via `@tailwindcss/postcss`). Token warna via CSS variables agar dual-mode rapi.
- **Font:** `Outfit` via `@fontsource/outfit` (400/500/600/700/800), self-hosted. Voucher/kode pakai system mono (`ui-monospace, monospace`).
- **Ikon:** `@phosphor-icons/react` (sudah terpasang) sebagai satu-satunya keluarga ikon. `lucide-react` yang masih ada di `package.json` **dihapus** agar tidak campur. `strokeWidth` seragam (regular).
- **Animasi:** `framer-motion` yang sudah ada dipakai seperlunya saja (transisi masuk, skeleton). Tidak ada GSAP/Three.js.
- **State:** `zustand` untuk sesi admin + cache paket.

## 3. Tema, Warna, Bentuk (Lock)

Satu tema lock per halaman: dukung **light + dark via `prefers-color-scheme`** (default ikut sistem). Token:

```css
:root {
  --surface: #ffffff;
  --surface-2: #f4f4f5;        /* zinc-100 */
  --text: #18181b;             /* zinc-900 */
  --text-2: #52525b;           /* zinc-600 */
  --line: #e4e4e7;             /* zinc-200 */
  --accent: #059669;           /* emerald-600 */
  --accent-ink: #ffffff;       /* teks di atas aksen */
  --danger: #dc2626;
}
@media (prefers-color-scheme: dark) {
  :root {
    --surface: #09090b;
    --surface-2: #18181b;
    --text: #fafafa;
    --text-2: #a1a1aa;
    --line: #27272a;
    --accent: #10b981;         /* emerald-500 agar kontras di gelap */
    --accent-ink: #052e16;
  }
}
```

- **Aksen tunggal: emerald.** Terkunci di seluruh halaman (CTA, harga aktif, status sukses, link). Dilarang aksen kedua (tidak ada ungu/biru neon).
- **Shape lock:** radius `12px` kartu, `10px` tombol/input. Dokumentasikan bila tombol pill, lalu konsisten.
- Dilarang `#000000`/`#ffffff` murni sebagai background besar (pakai zinc-950 / white dengan tint).

## 4. Tipografi

- Display/harga besar: `Outfit 700/800`, `tracking-tight`.
- Body: `text-[15px] md:text-base leading-relaxed`, tidak lebih sempit dari `text-sm` untuk info penting (nominal, kode voucher minimal `text-lg`).
- Kode voucher: mono, `text-2xl md:text-3xl`, `tracking-[0.2em]`, `font-bold`, selectable.
- Dilarang serif, dilarang Inter, dilarang gradient text untuk headline.

## 5. Mobile-First Responsive (Prioritas Utama)

Mayoritas pembeli memakai HP Android layar 360-400px, sering di luar ruangan (silau) dan satu tangan.

### 5.1 Aturan global

- Desain dari **360px ke atas**. Breakpoint: `sm 640 / md 768 / lg 1024 / xl 1280`.
- Konten publik: `max-w-md mx-auto` (katalog boleh `max-w-2xl` di desktop). Admin: `max-w-6xl mx-auto`.
- Tinggi hero/penuh: `min-h-[100dvh]`, tidak pernah `h-screen`.
- Grid, bukan flex-math: `grid grid-cols-1 sm:grid-cols-2 gap-4`.
- **Touch target minimal 44px** (tombol utama 48-52px, full-width di mobile).
- CTA primer di alur beli/voucher: **sticky bottom bar** dengan `padding-bottom: env(safe-area-inset-bottom)`.
- Kontras tinggi untuk pemakaian luar ruangan: teks penting minimal `text-2` (4.5:1), nominal dan tombol primer 7:1.
- Input font-size minimal **16px** agar iOS tidak auto-zoom.
- Setiap layout multi-kolom wajib mendeklarasikan fallback `< 768px` (single column).

### 5.2 Spesifikasi per halaman

**Katalog `/` (mobile-first):**
Header ringkas (nama layanan + jam) → grid 1 kolom kartu paket (2 kolom di `sm+`) → tiap kartu: nama, durasi + kecepatan, harga besar, tombol "Pilih" full-width. Tanpa hero besar, tanpa logo wall, tanpa marquee. Skeleton kartu saat loading.

**Bayar `/bayar/:id`:**
Kartu QR di tengah (QR 240-280px, `min(72vw, 280px)`), nominal total besar di bawah QR, countdown `mm:ss` mono yang jelas, nama paket + tombol "Batalkan". Polling status otomatis; transisi ke `/voucher` tanpa klik saat lunas. State expired: pesan + tombol "Buat pesanan baru".

**Voucher `/voucher/:id`:**
Kode raksasa di kartu (copy-on-tap + tombol Salin eksplisit), tombol **Login WiFi** primer sticky-bottom, tombol sekunder **Unduh PNG** (canvas) dan pengaturan format JPG. Info paket ringkas + masa berlaku. Link "Cara login manual" ke `/bantuan`.

**Setup wizard `/setup` + Admin:**
Form label-di-atas, error-di-bawah, satu kolom di mobile. Tabel transaksi/admin di mobile runtuh menjadi **daftar kartu** (bukan scroll-x tabel). Navigasi admin: bottom-tab di `< md`, sidebar di `md+`.

**Template hotspot `login.html` (di router, bukan di app):**
Self-contained total (CSS+JS inline, tanpa font kustom, system stack), satu kolom `max-width 420px`, tombol "Beli Voucher" 48px mengarah ke Mikbil. Lihat skill hotspot-mikrotik untuk detail CHAP/PAP.

### 5.3 Matriks uji wajib

Render dan klik tiap halaman pada: **360px, 375px, 390px, 768px, 1024px, 1440px**, dalam **light + dark**, dengan isi panjang (nama paket 40 char, kode 12 char). Tidak boleh ada: teks terpotong, CTA wrap 2 baris di desktop, scroll-x, QR terpotong, sticky bar menutup konten (beri `padding-bottom` kompensasi).

## 6. Komponen

- **Tombol:** primer (accent bg + accent-ink text, cek kontras AA), sekunder (outline `line`), danger terpisah. `:active scale-[0.98]`. Label 1-2 kata, satu baris di desktop.
- **Kartu paket:** border `line`, radius 12, variasi visual secukupnya (paket unggulan: border aksen + badge "Populer", maks 1 per halaman).
- **Countdown:** mono tabular, berubah tiap detik tanpa layout-shift (`tabular-nums`).
- **Toast:** hanya untuk transient (tersalin, tersimpan). Error form inline, bukan toast.
- **State wajib:** loading skeleton mengikuti bentuk akhir (tanpa spinner generik), empty state dengan ajakan aksi, error state dengan tombol coba lagi.
- **QR:** render dari `qr_payload` via `qrcode.react` (atau setara, dibundel). Latar QR selalu putih dengan padding quiet-zone, juga dalam dark mode.

## 7. Motion (Minimal, Termotivasi)

- Hanya: fade/slide-up saat masuk (≤ 300ms), shimmer skeleton, transisi status bayar. Tanpa parallax, tanpa scroll-hijack, tanpa loop tak berujung (kecuali shimmer loading).
- Semua animasi di atas level statis wajib honor `prefers-reduced-motion` (jatuh ke tampil instan).
- Animasikan hanya `transform`/`opacity`.

## 8. Aksesibilitas

- Semua input: `<label>` terasosiasi, `autocomplete` yang tepat, pesan error deskriptif + `aria-describedby`.
- Fokus terlihat (ring aksen) di semua elemen interaktif; urutan tab logis; QR dan kode voucher punya alternatif teks.
- Countdown memakai `aria-live="off"` (di-update tiap detik, jangan spam screen reader); perubahan status bayar memakai `aria-live="polite"` sekali.
- Bahasa sederhana, tanpa jargon; nominal selalu format `Rp` dengan pemisah ribuan.

## 9. Pre-Flight Check (sebelum merge halaman)

- [ ] Lolos matriks §5.3 (6 lebar × 2 tema), tanpa scroll-x.
- [ ] CTA primer terlihat tanpa scroll di `/bayar` dan `/voucher` pada 360px.
- [ ] Semua aset dari origin sendiri (cek Network: tidak ada request font/CDN/gambar eksternal).
- [ ] Satu aksen emerald di semua komponen; satu sistem radius.
- [ ] Tidak ada em-dash di seluruh string tampilan.
- [ ] Skeleton/empty/error state ada untuk tiap fetch data.
- [ ] `prefers-reduced-motion` diuji (animasi mati total).
- [ ] Bundle rute publik < 200 KB gzip (cek `vite build` report).
