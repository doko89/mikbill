# Tech Spec — Mikbil

> Pendamping [PRD](PRD.md). Dokumen ini adalah kontrak implementasi: skema DB, API, struktur kode, dan perilaku integrasi.
> Stack: Go (backend) + React (frontend) → satu binary. DB: SQLite. Router: ROS v6 & v7 via API klasik.

## 1. Arsitektur

```
                    ┌──────────────────────────────────┐
                    │         Binary Mikbil (Go)        │
 Pembeli ──HTTP──▶  │  static web/dist (embed)          │
                    │  /api/* (public+admin+webhook)    │──▶ kriskris.id (HTTPS, server-side)
 Admin ──HTTP──▶    │  poller 5s (goroutine)            │──▶ MikroTik API 8728/8729
                    │  SQLite (.db file)                │
                    └──────────────────────────────────┘
```

- Satu proses Go: serve frontend statis + API + worker polling.
- Semua rahasia (password router, API secret KrisKris, password voucher) terenkripsi AES-GCM di DB; kunci dari env `MIKBIL_ENC_KEY`.
- Frontend tidak pernah memanggil kriskris.id atau router secara langsung.

## 2. Struktur Repo

```
server/                    # backend Go (modul Go terpisah)
  cmd/mikbil/main.go
  internal/
    domain/                # entity + error; tanpa import ke layer lain
      admin.go package.go order.go voucher.go connection.go webhook.go settings.go
    usecase/               # interface Port (repo/gateway) + Interactor per fitur
      ports.go setup.go auth.go connection.go profile.go package.go
      order.go payment.go voucher.go provisioning.go
    repository/sqlite/     # migrasi + implementasi repo (database/sql, driver modernc.org/sqlite)
      migrations/ sqlite.go admin_repo.go package_repo.go order_repo.go ...
    infra/
      kriskris/            # client.go (invoice CRUD), sign.go (HMAC), webhook.go (verify)
      mikrotik/            # client.go (dial, TLS), profile.go, user.go, walledgarden.go, dns.go
      crypto/              # aes-gcm seal/open, password bcrypt, random voucher
    delivery/http/         # router, handler, dto, middleware (session, csrf, ratelimit)
      router.go public_handler.go admin_handler.go setup_handler.go webhook_handler.go
      session.go csrf.go ratelimit.go render.go
    delivery/web/          # embed.go → web/dist
  web/dist/                # hasil build React (di-copy saat build, di-embed ke binary)
  go.mod go.sum
frontend/                  # kode React (src/ saat ini dipindah ke sini bertahap)
docs/                      # PRD.md, tech_spec ini
Makefile                   # orkestrasi build frontend + backend
```

Module Go: `github.com/<owner>/mikbil` (disesuaikan saat init).

Dependency utama (diusulkan): `chi` (router), `modernc.org/sqlite` (pure-Go SQLite), client RouterOS API Go (dipilih saat implementasi, wajib support v6+v7), `qrcode` tidak perlu di backend (render di frontend).

## 3. Skema Database (SQLite)

```sql
CREATE TABLE admins (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- keys: setup_completed(0/1), deploy_mode(vps|local),
--   site_url(https://wifi.contoh.com | http://mikbil.lan),
--   local_server_ip(10.10.10.2, mode lokal),
--   hotspot_login_base(http://192.168.88.1/login),
--   voucher_length(8), kriskris_api_key, kriskris_api_secret(enc),
--   kriskris_callback_secret(enc)

CREATE TABLE mikrotik_connections (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL DEFAULT 'default',
  host TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 8728,
  username TEXT NOT NULL,
  enc_password TEXT NOT NULL,
  use_tls INTEGER NOT NULL DEFAULT 0,
  is_active INTEGER NOT NULL DEFAULT 1,
  last_test_at TEXT,
  last_test_ok INTEGER,
  last_test_msg TEXT
);

CREATE TABLE packages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_idr INTEGER NOT NULL,
  validity_seconds INTEGER NOT NULL,      -- dipakai untuk limit-uptime voucher
  validity_label TEXT NOT NULL,           -- tampil di katalog ("12 Jam", "7 Hari")
  profile_name TEXT NOT NULL,             -- user profile di router
  rate_limit TEXT NOT NULL DEFAULT '',    -- "5M/10M" (info + dipakai saat create profile)
  session_timeout INTEGER NOT NULL DEFAULT 0, -- detik, 0 = tanpa batas
  shared_users INTEGER NOT NULL DEFAULT 1,
  is_active INTEGER NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE orders (
  id TEXT PRIMARY KEY,                    -- ord_<12 char base32>
  package_id INTEGER NOT NULL REFERENCES packages(id),
  base_amount INTEGER NOT NULL,
  unique_code INTEGER NOT NULL DEFAULT 0,
  total_amount INTEGER NOT NULL,
  kriskris_invoice_id TEXT UNIQUE,
  status TEXT NOT NULL DEFAULT 'pending', -- pending|processing|paid|expired|cancelled|failed
  qr_payload TEXT NOT NULL DEFAULT '',
  expires_at TEXT,
  paid_at TEXT,
  failed_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_orders_status ON orders(status, expires_at);

CREATE TABLE vouchers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id TEXT NOT NULL UNIQUE REFERENCES orders(id),
  username TEXT NOT NULL UNIQUE,
  enc_password TEXT NOT NULL,
  profile_name TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE webhook_deliveries (
  delivery_id TEXT PRIMARY KEY,
  event TEXT NOT NULL,
  invoice_id TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor TEXT NOT NULL DEFAULT 'system',
  action TEXT NOT NULL,
  entity TEXT NOT NULL DEFAULT '',
  entity_id TEXT NOT NULL DEFAULT '',
  meta_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
```

Migrasi: file SQL berurutan di `internal/repository/sqlite/migrations/*.sql`, dijalankan saat start (`./mikbil serve` auto-migrate; `./mikbil migrate` manual).

## 4. API Contract Internal

Base: `<site_url>/api`. Format: JSON. Error: `{ok:false, error:"pesan", code?}` dengan HTTP status sesuai.

### 4.1 Public (tanpa auth, rate-limit ketat)

| Method & Path | Body / Query | Respons |
|---|---|---|
| `GET /api/packages` | — | `{ok, data:[{id,name,description,price_idr,validity_label,profile_name}]}` (hanya `is_active=1`, urut `sort_order`) |
| `POST /api/orders` | `{package_id}` | `201 {ok, data:{order_id, qr_payload, base_amount, unique_code, total_amount, expires_at}}` |
| `GET /api/orders/:id` | — | `{ok, data:{order_id,status,package:{...},total_amount,expires_at,paid_at, voucher?:{username,password,profile_name}, login_url?}}`. `voucher`+`login_url` hanya bila `paid`. Rate-limit: 30 req/menit per IP per order |
| `POST /api/orders/:id/cancel` | — | `{ok, data:{status:"cancelled"}}` (hanya bila `pending`; backend juga cancel invoice KrisKris, best-effort) |
| `GET /api/health` | — | `{ok, data:{time, db:"ok", clock_skew_sec, router_last_test}}` |

`login_url` = `<hotspot_login_base>?username=<u>&password=<p>` (URL-encoded).

Aturan `POST /api/orders`:
- Paket harus aktif; `amount` = `price_idr` (validasi min 1000 — harga paket < 1000 ditolak saat simpan paket).
- `label` invoice: `Mikbil <nama_paket> <order_id>` (dipangkas maks 64 char).
- Sukses KrisKris → simpan invoice id + qr + expired. Gagal `duplicate_pending`/`429` → retry otomatis di backend (maks 3x, hormati `retry_after`) sebelum balas error.
- Rate-limit global create-order: 10 req/menit per IP.

### 4.2 Webhook KrisKris

`POST /api/webhooks/kriskris`
- Header: `X-Kriskris-Event`, `X-Kriskris-Delivery`, `X-Kriskris-Signature: sha256=<hex>`.
- Verifikasi: HMAC-SHA256 body mentah dengan `kriskris_callback_secret` (perbandingan constant-time). Gagal → `401`, tidak diproses.
- `test.ping` → catat delivery, balas `200 {ok:true}`.
- `invoice.paid` / `invoice.expired` → cek `delivery_id` di `webhook_deliveries`; duplikat → `200` tanpa proses. Baru → simpan delivery, update order, enqueue pemrosesan async, balas `200` < 10 detik (jangan tunggu pembuatan voucher router).
- Body sesuai panduan KrisKris (`event, delivery_id, invoice_id, merchant_id, amount, base_amount, unique_code, label, status, paid_at?`).

### 4.3 Setup (hanya bila `setup_completed=0`; setelah selesai semua endpoint ini 404)

| Method & Path | Fungsi |
|---|---|
| `POST /api/setup/admin {name,username,password}` | Buat admin pertama (bcrypt, min 8 char) |
| `POST /api/setup/site {deploy_mode, site_url, local_server_ip?, hotspot_login_base}` | Simpan alamat |
| `POST /api/setup/connection {host,port,username,password,use_tls}` | Simpan koneksi router (password dienkripsi) |
| `POST /api/setup/connection/test` | Test koneksi tanpa simpan |
| `POST /api/setup/kriskris {api_key, api_secret, callback_secret}` | Simpan kredensial (secret dienkripsi) |
| `POST /api/setup/kriskris/test` | Validasi signature via batch status id dummy (harus dapat respons envelope valid, bukan 401) |
| `POST /api/setup/provision` | Jalankan provisioning (§8.3), kembalikan ringkasan hasil per langkah |
| `POST /api/setup/finish` | Set `setup_completed=1` (wajib: admin ada + test router OK) |

### 4.4 Admin (sesi cookie HttpOnly + header CSRF `X-CSRF-Token`)

Auth: `POST /api/admin/login {username,password}` → set cookie sesi; `POST /api/admin/logout`; `GET /api/admin/me`.

| Method & Path | Fungsi |
|---|---|
| `GET/POST /api/admin/connections`, `PUT/DELETE /api/admin/connections/:id` | CRUD koneksi (password tulis-saja, tak pernah dibaca balik) |
| `POST /api/admin/connections/:id/test` | Test + simpan hasil |
| `GET /api/admin/router/profiles` | Live dari router: `[{name, rate_limit, session_timeout, shared_users}]` |
| `POST /api/admin/router/profiles {name, rate_limit?, session_timeout?, shared_users?}` | Buat profile di router |
| `GET/POST /api/admin/packages`, `PUT/DELETE /api/admin/packages/:id` | CRUD paket. `POST/PUT` dengan `create_profile:true` → buat profile di router dulu (gagal = paket tidak disimpan). Validasi: `price_idr >= 1000` |
| `GET /api/admin/orders?status=&q=&page=` | List transaksi (paginasi 20) |
| `GET /api/admin/orders/:id` | Detail + voucher (password didekripsi server-side) |
| `GET /api/admin/provisioning/status` | Verify: walled garden ada? DNS statik ada (lokal)? http-pap aktif? |
| `POST /api/admin/provisioning/apply` | Terapkan ulang provisioning |
| `GET /api/admin/hotspot/login-template` | Unduh `login.html` (attachment) |
| `GET /api/admin/hotspot/button-snippet` | `{snippet}` — HTML tombol dengan site_url terisi |
| `GET/PUT /api/admin/settings` | site_url, hotspot_login_base, voucher_length, kredensial kriskris (tulis-saja) |

## 5. Frontend (React)

Route:

| Route | Guard | Halaman |
|---|---|---|
| `/` | — | Katalog paket (grid kartu: nama, durasi, kecepatan, harga, tombol Pilih) |
| `/bayar/:orderId` | — | QR (render `qr_payload` via lib QR, e.g. `qrcode.react`), nominal total besar, countdown, status; poll `GET order` 5 detik; tombol Batal |
| `/voucher/:orderId` | — | Kode besar + Salin + Unduh PNG/JPG (canvas) + Login WiFi + info paket |
| `/bantuan` | — | Cara beli & cara login manual |
| `/setup` | first-run | Wizard 5 langkah (§6.1 PRD) |
| `/admin/login` | — | Login |
| `/admin` | sesi | Ringkasan (omzet hari ini, order pending, status router) + transaksi terbaru |
| `/admin/transaksi` | sesi | Tabel + filter + detail |
| `/admin/paket` | sesi | CRUD paket + pilih/buat profile |
| `/admin/router` | sesi | Koneksi + profile + provisioning + snippet/template |
| `/admin/pengaturan` | sesi | Site, kriskris, voucher, hotspot login base |

State: `zustand` (sudah ada) untuk sesi admin + cache paket. API client `fetch` + CSRF header otomatis untuk `/api/admin/*`.

Build: `npm run build` di `frontend/` → output dipindah ke `server/web/dist/` → `go build` embed. Dev: Vite proxy `/api` ke Go (`localhost:8080`).

## 6. Integrasi KrisKris (detail implementasi)

Signer (outgoing):
```
bodyHash  = hex(sha256(rawBody))
toSign    = timestamp + "\n" + METHOD + "\n" + path + "\n" + rawQuery + "\n" + bodyHash
signature = "v1=" + hex(hmac_sha256(apiSecret, toSign))
```
- `timestamp`: UTC RFC3339 detik (`2006-01-02T15:04:05Z`).
- `rawQuery`: string query mentah tanpa `?` (batch status), else `""`.
- Clock skew dijaga via NTP; health menampilkan `clock_skew_sec` (cek terhadap `Date` header respons KrisKris).

Operasi client (`infra/kriskris/client.go`):
- `CreateInvoice(ctx, amount, label) → Invoice`
- `GetInvoice(ctx, id) → Invoice`
- `BatchStatus(ctx, ids[≤50]) → []InvoiceStatus`
- `CancelInvoice(ctx, id)`

Poller (`usecase/payment.go` + worker di `main.go`):
- Ticker 5 detik. Ambil order `pending` yang belum expired (maks 50 per siklus, batch status 1 request).
- `paid` → tandai + picu `processPaidOrder` (sama dengan jalur webhook).
- `expired/cancelled` → update status, stop.
- Order yang `expires_at` lewat tanpa kabar → tandai `expired` lokal (tetap konfirmasi sekali via GET).

`processPaidOrder(orderID)` — idempotent, satu-satunya pintu voucher:
1. `BEGIN; SELECT status ... FOR UPDATE`-setara (SQLite: `BEGIN IMMEDIATE`); hanya lanjut bila `pending` → set `processing`; bila sudah `paid` → commit + return.
2. Generate kredensial (`crypto/rand`, alfabet `ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789`, panjang `voucher_length`).
3. Cek router by comment `mikbil:<order_id>`; belum ada → create user.
4. Insert voucher, set order `paid` + `paid_at`. `COMMIT`.
5. Gagal di langkah 3 → set `failed` + `failed_reason`, audit log; admin bisa retry manual (`POST /api/admin/orders/:id/retry` — tambahkan bila perlu).

## 7. Integrasi MikroTik (detail implementasi, v6 & v7)

Client: dial TCP `host:port` (API) atau TLS (API-SSL). Login user+password. Timeout 10 detik. Semua path di bawah identik di v6 & v7.

| Fungsi | Perintah API |
|---|---|
| Test | `/system/resource/print` (fallback `/ip/hotspot/print`) |
| List user profile | `/ip/hotspot/user/profile/print` |
| Buat user profile | `/ip/hotspot/user/profile/add` `=name=..` `=rate-limit=5M/10M` `=session-timeout=3600` `=shared-users=1` `=idle-timeout=5m` |
| Buat voucher | `/ip/hotspot/user/add` `=name=<u>` `=password=<p>` `=profile=<profile>` `=limit-uptime=<dur>` `=comment=mikbil:<order>` |
| Cari voucher order | `/ip/hotspot/user/print` `?comment=mikbil:<order>` |
| List server profile | `/ip/hotspot/profile/print` (baca `login-by`) |
| Aktifkan PAP | `/ip/hotspot/profile/set` `=.id=<id>` `=login-by=<existing>,http-pap` (skip bila sudah ada) |
| Walled garden list/add/remove | `/ip/hotspot/walled-garden/print|add|remove` (`=dst-host=..` / `=dst-address=..` `=action=allow` `=comment=mikbil`) |
| DNS statik list/add/remove | `/ip/dns/static/print|add|remove` (`=name=..` `=address=..` `=comment=mikbil`) |

Format `limit-uptime` dari `validity_seconds`: helper `1d 02:00:00` style RouterOS (hari + `HH:MM:SS`). Contoh: 43200 → `12:00:00`; 604800 → `7d 00:00:00`.

Semua entri yang dibuat Mikbil beri `comment=mikbil` agar verify/remove tepat sasaran.

## 8. Provisioning (sequence)

`POST /api/setup/provision` / `/api/admin/provisioning/apply` menjalankan berurutan, hasil per langkah dilaporkan `{step, ok, message}`:

1. **Koneksi**: dial + login. Gagal → stop.
2. **http-pap**: untuk tiap hotspot server profile yang dipakai server hotspot aktif (atau semua profile bila ragu): baca `login-by`, tambah `http-pap` bila belum ada.
3. **Walled garden**: hapus entri `comment=mikbil` lama → tambah baru:
   - VPS: `dst-host=<domain-publik>` (+ `dst-port=443`? — putuskan saat implementasi; default tanpa batas port).
   - Lokal: `dst-address=<ip-server>` + `dst-host=<domain-lokal>`.
4. **DNS statik** (mode lokal saja): `name=<domain-lokal>` → `address=<ip-server>`; cek `/ip/dns/print` `allow-remote-requests=yes`, bila tidak → beri peringatan (jangan ubah diam-diam? — putuskan: ubah + catat di hasil).
5. **Verify**: baca ulang semua; kembalikan status akhir.

`GET /api/admin/provisioning/status` hanya membaca (tanpa menulis) dan membandingkan dengan ekspektasi dari settings.

## 9. Konfigurasi & Build

Env:

| Var | Default | Ket |
|---|---|---|
| `MIKBIL_ADDR` | `:8080` | listen |
| `MIKBIL_DB` | `./mikbil.db` | path SQLite |
| `MIKBIL_ENC_KEY` | — (wajib) | 32 byte, hex 64 char; generate: `./mikbil genkey` |

Makefile (di root):
```make
frontend:  cd frontend && npm ci && npm run build && rm -rf ../server/web/dist && cp -r dist ../server/web/dist
build:     frontend && cd server && go build -o ../bin/mikbil ./cmd/mikbil
dev-be:    cd server && go run ./cmd/mikbil serve
dev-fe:    cd frontend && npm run dev   # proxy /api → :8080
```

Deploy VPS: binary + `.db` + env (systemd unit disediakan di `deploy/mikbil.service`), reverse proxy opsional (Caddy/nginx) untuk TLS bila tidak terminate di Go. Rekomendasi: TLS di reverse proxy, Go HTTP lokal.
Deploy lokal: binary di server LAN, autostart (systemd / Task Scheduler / pm2 — sesuai OS), pastikan NTP + internet egress.

Perintah binary: `serve`, `migrate`, `genkey`, `version`.

## 10. Keamanan

- Admin: bcrypt (cost 12), sesi acak 32 byte di cookie HttpOnly+Secure+SameSite=Lax, simpan hash sesi di DB (`sessions` — tambahkan tabel bila sesi persistent; alternatif JWT HMAC — putuskan: sesi DB agar bisa revoke).
- CSRF: token per sesi untuk semua `POST/PUT/DELETE /api/admin/*`.
- Rate-limit: login 5/menit/IP; create-order 10/menit/IP; get-order 30/menit/IP/order; webhook 120/menit/IP (longgar, retry KrisKris).
- Enkripsi: AES-256-GCM, nonce acak per nilai; format simpan `base64(nonce|ciphertext)`.
- Secret tidak pernah ke log; password router & kriskris tulis-saja via API (GET mengembalikan mask `••••` / `sudah diisi?`).
- Timeout HTTP client 10 detik; validasi `Content-Type`/ukuran body webhook (maks 64KB).

## 11. Testing

- Unit Go: signer KrisKris (vektor uji dari contoh panduan), verifier webhook, format limit-uptime, generator voucher, state machine order (repo mock).
- Integrasi: API + SQLite sementara; webhook dedup (kirim 2x delivery_id sama → 1 voucher); expired flow.
- Kontrak KrisKris: test manual/sandbox end-to-end (buat invoice kecil → bayar → paid).
- Matriks router: CHR/v6 + CHR/v7: test koneksi, list/buat profile, buat voucher, provisioning (walled garden, DNS statik, http-pap), hapus.
- UAT: 10 kriteria §14 PRD, termasuk mode lokal tanpa webhook dan uji HP belum-login.

## 12. Tahapan Implementasi (mapping PRD §16)

1. Skeleton Go + migrasi + embed web + auth admin + API setup (tanpa integrasi) + kerangka halaman setup/admin.
2. `infra/mikrotik` + koneksi/test + list/buat profile + provisioning + template/snippet + halaman `/admin/router`.
3. CRUD paket + katalog public.
4. `infra/kriskris` + order + halaman bayar + poller + webhook + retry.
5. Voucher + halaman hasil (PNG/JPG canvas + login URL) + riwayat admin.
6. Rate-limit, health, CSRF, systemd unit, panduan instalasi VPS & lokal, UAT penuh.
