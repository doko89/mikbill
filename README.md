# Mikbil

Billing hotspot voucher self-service + QRIS. Satu binary Go menyajikan backend API
dan frontend React (di-embed), dengan SQLite sebagai satu-satunya state eksternal.

Dokumen acuan: `docs/PRD.md`, `docs/tech_spec.md`, `docs/DESIGN_GUIDE.md`.

## Instalasi

Panduan lengkap: [docs/INSTALL.md](docs/INSTALL.md).

```sh
# Linux: pasang rilis terbaru (idempotent, aman untuk upgrade)
curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash
```

```powershell
# Windows (PowerShell sebagai Administrator): pasang rilis terbaru
irm https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.ps1 | iex
```

Selesai → buka `http://<ip-server>:8080/setup` untuk wizard penyiapan awal.

## Struktur

```
server/     # backend Go (modul github.com/mikbil/mikbil)
frontend/   # React + Vite + Tailwind v4
docs/       # PRD, tech spec, design guide
deploy/     # unit systemd (fase 6)
bin/        # hasil build (diabaikan git)
```

## Prasyarat

- Go 1.24+
- Node.js 20+ dan npm

## Cara build

```sh
# Build lengkap: frontend + satu binary di bin/mikbil
make build

# Hanya build ulang frontend ke server/web/dist
make frontend

# Hanya backend (memakai web/dist yang sudah ada)
make build-be

# Test backend
make test
```

## Cara jalan

```sh
# 1. Buat kunci enkripsi (32 byte hex) sekali saja, simpan aman
./bin/mikbil genkey

# 2. Jalankan server (auto-migrate saat start)
MIKBIL_ENC_KEY=<hasil-genkey> MIKBIL_DB=./mikbil.db MIKBIL_ADDR=:8080 ./bin/mikbil serve
```

Perintah lain: `./bin/mikbil migrate`, `./bin/mikbil version`.

| Env | Default | Ket |
|---|---|---|
| `MIKBIL_ADDR` | `:8080` | alamat listen |
| `MIKBIL_DB` | `./mikbil.db` | path file SQLite |
| `MIKBIL_ENC_KEY` | wajib | 32 byte hex (64 char), dari `genkey` |

Setelah jalan, buka `http://localhost:8080/setup` untuk wizard penyiapan awal.

## Mode dev

```sh
# Terminal 1: backend (port 8080)
make dev-be

# Terminal 2: frontend Vite (proxy /api ke :8080)
make dev-fe
```
