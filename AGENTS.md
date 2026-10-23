# AGENTS.md — Aturan Orkestrasi Mikbil

## 1. Peran

Anda adalah **orkestrator**. Rencanakan pekerjaan, pecah menjadi subtugas independen, dan
kerjakan **secara paralel** via subagent (Work tool). Jangan kerjakan semuanya serial di
satu konteks bila bisa diparalelkan.

- Maksimal 15 subagent per workflow; batch pekerjaan sejenis dalam satu `parallel([...])`.
- Subagent baca-tulis kode lewat Work tools warisan; repo discovery dilakukan di sisi anak.
- Setiap anak wajib membuka body implementasi/test yang ia sitir sebelum submit; grep saja
  bukan bukti.
- Sintesis akhir: kumpulkan text/data tiap anak, laporkan gap yang belum tertutup.

## 2. Dokumen Acuan (wajib dibaca sebelum kerja)

| Dokumen | Isi |
|---|---|
| `docs/PRD.md` | Kebutuhan produk, alur, keputusan desain, kriteria penerimaan |
| `docs/tech_spec.md` | Kontrak implementasi: skema DB, API, struktur kode, integrasi |
| `docs/DESIGN_GUIDE.md` | Panduan desain frontend + aturan mobile-first + pre-flight check |

Bila dokumen dan permintaan lisan bertentangan, **dokumen menang** kecuali user
menyetujui perubahan — lalu perbarui dokumennya di turn yang sama.

## 3. Aturan Skill

- **Pekerjaan frontend APAPUN** (halaman, komponen, style, template hotspot):
  subagent yang mengerjakannya WAJIB `read_skill("design-taste-frontend")` lebih dulu,
  lalu patuhi `docs/DESIGN_GUIDE.md` + pre-flight check-nya.
- Template `login.html` hotspot: WAJIB juga `read_skill("hotspot-mikrotik")`
  (aturan self-contained, CHAP/PAP, budget ukuran).
- Satu skill dibaca sekali per sesi anak; pakai ulang hasilnya.

## 4. Struktur Repo

```
server/     # backend Go (modul terpisah): cmd/, internal/, web/dist (embed)
frontend/   # React + Vite + Tailwind v4 (di-copy ke server/web/dist saat build)
docs/       # PRD.md, tech_spec.md, DESIGN_GUIDE.md
Makefile    # orkestrasi build (root)
```

- Backend: clean architecture (`domain → usecase → repository/infra → delivery`).
  `domain` tanpa dependensi luar; `usecase` hanya via interface (ports).
- Satu binary hasil akhir: frontend di-embed via `go:embed`, SQLite pure-Go.
- Dilarang menambah dependensi eksternal runtime di jalur pembeli
  (semua aset dari origin Mikbil — lihat DESIGN_GUIDE §1).

## 5. Cara Kerja per Tugas

1. Tulis todo (`write_todos`) dengan fase dari `docs/tech_spec.md` §12 bila relevan.
2. Bagi tugas independen ke subagent paralel (misal: migrasi DB + handler HTTP +
   halaman React + template hotspot). Tugas dependen dirantai via pipeline.
3. Setiap unit kerja selesai bila: kode + test relevan hijau + pre-flight desain
   (untuk frontend) tercentang.
4. Verifikasi akhir di sisi orkestrator: `go build ./...`, `go test ./...`,
   `npm run build` (frontend), dan smoke test rute yang diubah.
5. Jangan commit/push kecuali user meminta eksplisit.

## 6. Standar Mutu

- Ikuti kontrak API di tech_spec §4 persis (path, body, envelope `{ok, data/error}`).
- Idempotency pembayaran→voucher tidak boleh rusak: satu-satunya pintu voucher
  adalah `processPaidOrder` (tech_spec §6).
- Setiap bug fix menyertakan test regresi; setiap endpoint baru menyertakan test.
- Bahasa UI dan string pengguna: Indonesia. Komentar kode: ringkas, tanpa deliberasi.
