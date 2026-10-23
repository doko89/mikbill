# PRD — Mikbil: Billing Hotspot Voucher Self-Service + QRIS

> Status: DRAFT v1.0 — siap didiskusikan sebelum implementasi.
> Bahasa UI: Indonesia. Mata uang: IDR.

## 1. Ringkasan

Mikbil adalah aplikasi billing self-service untuk penyedia layanan hotspot voucher MikroTik. Pembeli voucher cukup scan QRIS dari HP tanpa perlu penjaga konter; owner mengelola paket, koneksi router, dan pembayaran dari satu panel admin.

Satu instance Mikbil = satu owner = satu router (**single-tenant**, bukan multi-tenant). Aplikasi bisa dipasang di **VPS** (domain publik) atau di **server lokal** milik penyedia hotspot (mini PC/RPi di LAN, domain lokal diarahkan via DNS statik di MikroTik).

## 2. Tujuan & Tolok Ukur

- Pembeli bisa beli voucher dalam < 2 menit dari halaman hotspot login sampai voucher tampil.
- Pembayaran QRIS tercatat otomatis (lunas/kedaluwarsa) tanpa input manual.
- Voucher hanya dibuat tepat satu kali per pembayaran lunas (idempotent).
- Instalasi mudah: satu binary Go yang menyajikan backend + frontend React hasil embed.

## 3. Persona

| Persona | Kebutuhan |
|---|---|
| Owner hotspot (admin) | Setup awal, konek router, atur paket & harga, pantau transaksi, unduh template login hotspot |
| Pembeli voucher | Lihat harga, bayar QRIS dari HP (sering kali belum login internet / di balik walled garden), dapat kode voucher, login WiFi |
| (Opsional v2) Penjaga konter/reseller | Buat voucher manual, cetak banyak voucher |

## 4. Tech Stack (keputusan)

- **Backend: Go** (stdlib `net/http` + router ringan e.g. chi; tanpa framework berat).
- **Frontend: React** (repo ini) — React Router: area public `/` + area admin `/admin`.
- **Distribusi: satu binary.** Hasil build frontend (`web/dist`) di-embed ke binary Go via `go:embed` dan diserve oleh Go. Satu-satunya state eksternal: file SQLite + (opsional) file config.
- **Database: SQLite** (driver pure-Go agar tetap satu binary, e.g. `modernc.org/sqlite`). Backup = salin file `.db`.
- **MikroTik:** RouterOS API klasik (port 8728 / 8729 TLS). **Wajib mendukung v6 dan v7** — karena itu hanya pakai API klasik (REST API hanya ada di v7, tidak dipakai). Library Go: client RouterOS API (e.g. `routeros` / `gopik` — dipilih saat implementasi; diuji ke keduanya).
- **QRIS: KrisKris.id** sesuai kontrak di bawah (API invoice + callback webhook).

### 4.1 Struktur Go — Clean Architecture

```
server/                            # backend Go (modul Go terpisah)
  cmd/mikbil/main.go               # entrypoint: load config, migrate, serve
  internal/
    domain/                        # entity + error domain (tanpa dependensi luar)
      admin.go order.go package.go voucher.go connection.go webhook.go
    usecase/                       # business logic (interface repo & gateway di sini)
      setup.go auth.go package.go order.go payment.go voucher.go
      mikrotik.go provisioning.go
    repository/
      sqlite/                      # implementasi repo (migrasi, query)
    infra/
      kriskris/                    # client API + signer HMAC + verifier webhook
      mikrotik/                    # client RouterOS API + query builder
    delivery/
      http/                        # handler, route, middleware, DTO
      web/                         # embed FS frontend (web/dist)
  web/dist/                        # build output React (di-copy saat build, di-embed)
frontend/ (repo src/ saat ini)     # kode React
```

Aturan: `domain` tidak import layer lain; `usecase` hanya tergantung interface; `infra`/`repository` implementasi; `delivery` hanya mapping HTTP↔usecase.

## 5. Scope

### 5.1 In-scope (v1)

1. **Setup wizard (first-run):** buat akun admin, input domain Mikbil, input kredensial MikroTik, input kredensial KrisKris, provision walled garden.
2. **Auth admin:** login/logout sesi. Single role `owner` di v1.
3. **Manajemen koneksi MikroTik:** CRUD koneksi (host, port, user, password, TLS), tombol test koneksi.
4. **Manajemen profile hotspot:**
   - Ambil daftar user profile yang sudah ada di router.
   - **Mapping:** paket Mikbil ↔ profile yang sudah ada; atau **buat profile baru** dari Mikbil (nama, `rate-limit`, `session-timeout`/`uptime-limit`, `shared-users`).
5. **Manajemen paket voucher:** CRUD (nama, deskripsi, harga, profile, masa aktif voucher). Harga tampil di katalog public.
6. **Katalog public + checkout QRIS:** daftar paket → buat order → tampil QR + nominal total + countdown 15 menit → status otomatis (polling + webhook).
7. **Generate voucher otomatis** saat lunas: buat `/ip/hotspot/user` (username=password acak) dengan profile paket. Tampil halaman voucher: kode, tombol salin, unduh gambar (**PNG/JPG saja**), login otomatis.
8. **Riwayat transaksi + detail order.**
9. **Template hotspot `login.html` + snippet tombol:** admin bisa unduh `login.html` jadi dari Mikbil, atau tempel potongan kode tombol "Beli Voucher" ke `login.html` existing miliknya (snippet disediakan saat provisioning + di halaman pengaturan).
10. **Provisioning router:** tambah/hapus entri walled garden + aktifkan `http-pap` pada hotspot server profile (aditif, login-by existing dipertahankan) + tambah/hapus DNS statik bila mode server lokal (lihat §9.4).

### 5.2 Out-of-scope (v2+)

Multi-tenant \& multi-router/lokasi (diputuskan: single-tenant), multi-user admin & role, reseller & cetak massal, member/PPPoE, laporan keuangan lanjutan, multi-bahasa, notifikasi Telegram/WA, unduh voucher selain PNG/JPG (PDF/cetak konter).

## 6. Alur Utama

### 6.1 Onboarding / Setup wizard (`/setup`, hanya saat belum ada admin)

1. Langkah 1 — Akun admin: nama, username, password (konfirmasi). Validasi kekuatan minimal.
2. Langkah 2 — Alamat Mikbil: pilih mode **VPS** (`https://wifi.contoh.com`, wajib HTTPS publik) atau **server lokal** (`http://mikbil.lan`, IP LAN server + nama domain lokal). Alamat ini dipakai untuk link tombol "Beli Voucher", redirect login, dan callback URL default (`<alamat>/api/webhooks/kriskris`; catatan: webhook KrisKris hanya sampai pada mode VPS — mode lokal mengandalkan polling penuh, lihat §7).
3. Langkah 3 — MikroTik: host, port (default 8728 / 8729 bila TLS), user, password, toggle TLS, tombol **Test koneksi** (wajib sukses sebelum lanjut).
4. Langkah 4 — KrisKris: API key, API secret, callback secret. Tanpa tombol tes koneksi (lookup id dummy selalu 500 di server KrisKris); kredensial terbukti valid saat order pertama berhasil dibuat.
5. Langkah 5 — Provision: tombol **Aktifkan akses** → (a) tambah walled garden (mode VPS: `dst-host` domain publik; mode lokal: `dst-address` IP server + `dst-host` domain lokal), (b) mode lokal: tambah DNS statik (`/ip/dns/static`, domain lokal → IP server), (c) aktifkan `http-pap` pada hotspot server profile (aditif). Tampil ringkasan + tombol **Unduh login.html** + **potongan kode tombol "Beli Voucher"** siap tempel ke `login.html` existing (dengan tombol salin).
6. Selesai → redirect ke `/admin`.

### 6.2 Pembelian voucher (public)

1. User connect WiFi → halaman hotspot MikroTik → klik **Beli Voucher** → buka `https://<mikbil>/` (lolos walled garden).
2. Pilih paket → POST create order → backend buat invoice KrisKris → tampil QR (`qr_payload` dirender jadi gambar di frontend), nominal **total** (base + kode unik), label, countdown kedaluwarsa.
3. Frontend poll `GET /api/orders/:id` tiap ~5 detik. Backend juga menjalankan poller ke KrisKris tiap 5 detik per order pending + menerima webhook sebagai jalur utama.
4. Lunas → backend (tepat sekali, terkunci per order): generate kredensial acak → buat user di MikroTik → simpan voucher → tandai order `paid`.
5. Frontend tampil halaman voucher: kode besar, tombol **Salin**, **Unduh PNG/JPG**, **Login WiFi** (via http-pap, lihat §9.3).
6. Kedaluwarsa/dibatalkan → tampil status + tombol kembali ke katalog / buat order baru.

### 6.3 Admin kelola paket & profile

- Buka `/admin/paket` → daftar paket (nama, harga, profile, aktif).
- Tambah paket: isi nama/harga/masa aktif → pilih **profile yang sudah ada** (dropdown dari router, live) atau **buat baru** (form: nama profile, rate-limit up/down, batas durasi, shared-users) → simpan. Saat simpan "buat baru", backend create profile di router dulu, gagal = paket tidak tersimpan.
- Edit/hapus paket. Hapus paket tidak menghapus user/voucher yang sudah terjual.

## 7. Kontrak KrisKris.id (dari kriskris.id/panduan)

> Uang pembeli masuk langsung ke rekening outlet GoBiz owner; KrisKris hanya mencatat & mencocokkan mutasi.

- Base URL: `https://kriskris.id`. Rate limit: 60 req/menit per API key.
- Auth tiap request: header `X-API-Key`, `X-Kriskris-Timestamp` (RFC3339, toleransi ±5 menit), `X-Kriskris-Signature: v1=<hex>`.
- String signature = 5 baris join `\n`: `timestamp`, `METHOD`, `path`, `raw query` (baris kosong bila tanpa query), `hex(sha256(body))`. HMAC-SHA256 dengan API secret.
- `POST /api/v1/external/invoices` body `{amount, label?}` → `201 {ok:true, data:{id, status, amount, base_amount, unique_code, label, qr_payload, expires_at}}`.
  - Aturan: amount min Rp1.000; label opsional maks 64 char; kode unik acak 10–99; total QR = base + kode unik; expired 15 menit.
  - Dedup: satu total pending per outlet; bila 90 kode habis → `422 duplicate_pending` → Mikbil harus retry dengan backoff + pesan "coba lagi".
- `GET /api/v1/external/invoices/{id}` → detail + `paid_at`.
- `GET /api/v1/external/invoices/status?ids=a,b` (maks 50 id; query ikut signature) → array seurutan; id asing → `{id, found:false}`.
- `POST /api/v1/external/invoices/{id}/cancel` → `{data.status: "cancelled"}`.
- Status: `pending | paid | expired | cancelled`. Amplop sukses `{ok:true, data}`; gagal `{ok:false, error}` + kode: 401 (key/signature salah), 429 (`retry_after` detik), 400, 404, 422 (`invalid_amount | duplicate_pending | quota_exceeded | suspended | gobiz_offline`).
- Strategi Mikbil: **webhook sebagai jalur utama, polling tiap 5 detik sebagai fallback** (khusus server tanpa URL publik, polling jadi satu-satunya jalan).
- Callback: owner isi callback URL (`/api/webhooks/kriskris`) + secret di dashboard KrisKris. Event: `invoice.paid`, `invoice.expired`, `test.ping`. Header `X-Kriskris-Event`, `X-Kriskris-Delivery`, `X-Kriskris-Signature: sha256=<hex>` (HMAC-SHA256 body mentah dengan callback secret; wajib diverifikasi termasuk `test.ping`). Balas 2xx < 10 detik. Retry maks 5x (1m, 5m, 30m, 2h, 12h) dengan `delivery_id` sama → Mikbil simpan `delivery_id` dan abaikan duplikat.

## 8. Kontrak MikroTik (RouterOS API)

Operasi yang dipakai Mikbil (path RouterOS):

| Kebutuhan | Operasi |
|---|---|
| Test koneksi | login + `/system/resource/print` (atau `/ip/hotspot/print`) |
| Daftar profile | `/ip/hotspot/user/profile/print` |
| Buat profile | `/ip/hotspot/user/profile/add` (`name`, `rate-limit`, `session-timeout`, `shared-users`, `idle-timeout`) |
| Buat voucher | `/ip/hotspot/user/add` (`name`, `password`, `profile`, `comment=order:<id>`; tanpa `limit-uptime`, masa aktif ikut profile) |
| Cek/hapus voucher | `/ip/hotspot/user/print`, `/remove` |
| Walled garden | `/ip/hotspot/walled-garden/print|add|remove` (`dst-host`, opsional `dst-port`, `action=allow`) |
| Aktifkan PAP | `/ip/hotspot/profile/print` (baca `login-by`) lalu `/ip/hotspot/profile/set` tambah `http-pap` aditif |
| DNS statik (mode lokal) | `/ip/dns/static/print|add|remove` (`name` domain lokal, `address` IP server) |

Semua path di atas tersedia identik di v6 & v7. Uji matriks wajib: provisioning + voucher ke lab/CHR v6 dan v7.

- Kredensial router disimpan terenkripsi (AES-GCM, kunci dari env/file, bukan di DB plaintext).
- Semua operasi tulis ke router harus idempotent di sisi Mikbil (cek `comment=order:<id>` sebelum create ulang).

## 9. Keputusan Desain Penting

### 9.1 Format voucher

Default: `username = password` = string acak aman (alfabet tanpa karakter ambigu `0/O/1/I/l`, panjang 8). Bisa diubah panjangnya di settings. Alasan: mudah diketik pembeli dari HP.

### 9.2 Idempotency pembayaran→voucher

- Kolom `orders.status` + `orders.voucher_id` dengan transaksi DB; worker/webhook mengakuisisi lock per order (status `processing`) sebelum create user di router.
- Cek ganda: jika `voucher_id` sudah ada → skip. Jika user sudah ada di router (by comment) → pakai yang ada.

### 9.3 Login otomatis via PAP (diaktifkan saat provisioning)

Link login langsung `GET /login?username=<u>&password=<p>` hanya bekerja bila hotspot server profile mengizinkan `http-pap`.

Keputusan:
- Saat provisioning, Mikbil membaca `login-by` aktif lalu **menambahkan `http-pap` secara aditif** (`/ip/hotspot/profile/set login-by=<existing>,http-pap`) — mode lain (chap, http-chap, cookie, dll) tidak diubah.
- Halaman voucher Mikbil redirect ke `$(link-login)?username=<u>&password=<p>` (URL dasar hotspot dikonfigurasi di settings, default dari router).
- Opsi halaman login ada dua, admin pilih salah satu:
  - (a) **Unduh `login.html` jadi** dari Mikbil (sudah ada tombol "Beli Voucher" + kompatibel PAP/CHAP), upload ke router; atau
  - (b) **Tempel snippet tombol** ke `login.html` existing milik admin. Snippet (inline style, tanpa dependensi eksternal):
    ```html
    <!-- Mikbil: tombol beli voucher (tempel di dalam <body> login.html) -->
    <a href="https://<domain-mikbil>/?mac=$(mac)&ip=$(ip)"
       style="display:block;text-align:center;padding:14px;border-radius:10px;
              background:#16a34a;color:#fff;font-weight:700;text-decoration:none;">
      Beli Voucher WiFi
    </a>
    ```
- Voucher tetap bisa dilihat ulang dan diinput manual (lihat §9.6) sebagai fallback bila auto-login gagal.

### 9.4 Mode deploy: VPS vs server lokal

| Aspek | Mode VPS | Mode server lokal |
|---|---|---|
| Alamat Mikbil | Domain publik, **wajib HTTPS** valid | Domain lokal (e.g. `mikbil.lan`), **HTTP** |
| Walled garden | `dst-host` domain publik | `dst-address` IP server + `dst-host` domain lokal |
| DNS | Publik seperti biasa | DNS statik di MikroTik (`/ip/dns/static`) → IP server; wajib aktifkan `allow-remote-requests` agar klien hotspot memakai router sebagai DNS |
| Webhook KrisKris | Ya (URL publik) | **Tidak** — andalkan polling 5 detik penuh |
| Syarat | VPS + domain + sertifikat | Server di LAN + akses internet egress (untuk API KrisKris) + NTP |

- Desain default: semua panggilan KrisKris lewat backend Mikbil, dan QR dirender dari payload di frontend (tanpa request ke `kriskris.id` dari HP pembeli) → `kriskris.id` tidak perlu masuk walled garden.
- Sediakan tombol "Cek & perbaiki" yang memverifikasi entri walled garden + DNS statik masih ada di router.

### 9.6 Voucher bisa dilihat ulang (keputusan)

Password voucher boleh diambil ulang via `GET /api/orders/:id` (status paid) dengan rate-limit, agar pembeli bisa input manual bila auto-login gagal atau halaman ter-refresh. Halaman voucher selalu menampilkan kode + tombol salin + unduh gambar (**PNG/JPG saja**, render via canvas di frontend) + login otomatis.

### 9.7 Jam server

Signature KrisKris toleransi ±5 menit → installer wajib pastikan NTP aktif. Health-check menampilkan selisih jam bila > 60 detik.

## 10. Model Data (SQLite, awal)

- `admins(id, name, username UNIQUE, password_hash, created_at)`
- `settings(key PK, value)` — `site_domain`, `setup_completed`, `voucher_length`, `hotspot_login_url`, ...
- `mikrotik_connections(id, name, host, port, username, enc_password, use_tls, is_active, last_test_at, last_test_ok)`
- `packages(id, name, description, price_idr, validity_label, profile_name, rate_limit_down, rate_limit_up, session_timeout, shared_users, is_active, sort_order, created_at, updated_at)`
- `orders(id PK text `ord_...`, package_id FK, base_amount, unique_code, total_amount, kriskris_invoice_id UNIQUE, status `pending|processing|paid|expired|cancelled|failed`, qr_payload, expires_at, paid_at, failed_reason, created_at, updated_at)`
- `vouchers(id, order_id UNIQUE FK, username, enc_password, profile_name, created_at)`
- `webhook_deliveries(delivery_id PK, event, invoice_id, received_at)` — dedup callback
- `audit_logs(id, actor, action, entity, entity_id, meta_json, created_at)`

## 11. API Internal (draf)

Public (tanpa auth):

- `GET /api/packages` — paket aktif + harga
- `POST /api/orders {package_id}` → `{order_id, qr_payload(text), total_amount, expires_at}`
- `GET /api/orders/:id` → status + (bila paid) voucher `{username, password}` — boleh diambil ulang dengan rate-limit (keputusan §9.6: mendukung input manual).
- `POST /api/orders/:id/cancel`

Webhook:

- `POST /api/webhooks/kriskris` — verifikasi HMAC, dedup `delivery_id`, update order, picu generate voucher async. Selalu 2xx cepat bila signature valid.

Admin (sesi cookie HttpOnly + CSRF):

- `POST /api/admin/login|logout`, `GET /api/admin/me`
- `POST /api/admin/setup/*` (wizard, hanya saat `setup_completed=false`)
- CRUD `/api/admin/connections` + `POST /:id/test`
- `GET /api/admin/router/profiles` (live dari router), `POST /api/admin/router/profiles` (buat baru)
- CRUD `/api/admin/packages`
- `GET /api/admin/orders`, `GET /api/admin/orders/:id`
- `POST /api/admin/provisioning/apply` (walled garden + http-pap), `GET /api/admin/provisioning/status` (verify)
- `GET /api/admin/hotspot/login-template` (unduh `login.html`), `GET /api/admin/hotspot/button-snippet` (potongan kode tombol)

## 12. Halaman Frontend (draf route)

Public: `/` (katalog), `/bayar/:orderId` (QR + countdown + status), `/voucher/:orderId` (hasil + unduh + login), `/bantuan`.
Admin: `/admin/login`, `/admin` (ringkasan+transaksi terbaru), `/admin/transaksi`, `/admin/paket`, `/admin/router` (koneksi + profile + walled garden), `/admin/pengaturan` (domain, kriskris, template hotspot).
Setup: `/setup` (wizard 5 langkah, guard first-run).

## 13. Non-Fungsional

- **Distribusi:** satu binary + satu file `.db`; config via env (`MIKBIL_DB`, `MIKBIL_ADDR`, `MIKBIL_ENC_KEY`). Perintah: `./mikbil serve`, `./mikbil migrate`, `./mikbil version`. Binary yang sama untuk mode VPS maupun server lokal (beda config alamat + provisioning).
- **Keamanan:** password admin bcrypt; sesi HttpOnly+SameSite; CSRF; rate-limit login & create-order; secret router & kriskris terenkripsi AES-GCM; webhook diverifikasi HMAC + dedup; tidak ada secret di log.
- **Reliabilitas:** poller background (5 detik, hanya order pending & belum expired); retry create-invoice saat `duplicate_pending`/`429` (hormati `retry_after`); timeout HTTP 10 detik ke KrisKris & router.
- **Observabilitas:** log terstruktur, halaman health `/api/health` (db, jam/NTP skew, koneksi router terakhir, saldo/quota kriskris bila tersedia).

## 14. Kriteria Penerimaan v1

1. Fresh install → wizard 5 langkah sukses → login admin bisa.
2. Test koneksi router sukses & gagal (pesan error jelas) keduanya tertangani.
3. Buat paket dengan profile existing (muncul di dropdown live) dan dengan profile baru (terbuat di router, terlihat di WinBox).
4. End-to-end dengan invoice KrisKris real/sandbox: bayar → webhook/polling → voucher terbuat di router → halaman voucher tampil → kredensial bisa login WiFi.
5. Bayar 2x callback dengan `delivery_id` sama → hanya 1 voucher.
6. Invoice expired (15 menit) → status expired, tidak ada voucher.
7. Walled garden: HP belum login bisa buka domain Mikbil; tombol Beli di template login mengarah benar; tombol Login WiFi berhasil login.
8. Build menghasilkan satu binary; restart binary tidak menghilangkan data (SQLite persist).
9. Matriks uji: provisioning + beli voucher lolos di RouterOS v6 dan v7.
10. Mode server lokal: HP belum login bisa buka domain lokal (DNS statik + walled garden IP), beli sampai voucher terbit hanya via polling (tanpa webhook).

## 15. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Hotspot belum aktif http-pap | Login otomatis via link gagal | Provisioning aktifkan http-pap aditif + tombol verify; fallback kode + salin + input manual |
| Server tanpa URL publik | Webhook tak sampai | Polling 5 detik sebagai fallback penuh; tetap fungsional |
| `duplicate_pending` saat ramai | Order gagal dibuat | Retry otomatis + pesan ramah; batch status untuk efisiensi |
| HTTPS/walled garden/DNS salah config | Pembeli tak bisa buka Mikbil | Tombol verify + panduan per mode (VPS/lokal) + health-check |
| Jam server meleset > 5 mnt | Semua request KrisKris 401 | Cek NTP di health + peringatan setup |

## 16. Tahapan Kerja

1. **Fondasi:** skeleton Go clean-arch + SQLite + embed React + auth admin + wizard setup (tanpa integrasi).
2. **MikroTik:** koneksi + test + list/buat profile + walled garden + unduh template login.
3. **Paket & katalog:** CRUD paket + halaman katalog public.
4. **Pembayaran:** client KrisKris (signer, invoice, status, cancel) + order + QR + poller + webhook + retry.
5. **Voucher:** generate + halaman hasil + unduh PNG + login otomatis + riwayat admin.
6. **Hardening:** rate-limit, health, backup docs, build satu binary, panduan instalasi, UAT end-to-end.

## 17. Pertanyaan Terbuka untuk Owner

1. ~~RouterOS target: v6, v7, atau keduanya?~~ → **Diputuskan: keduanya** (API klasik saja, uji matriks v6+v7).
2. ~~Password voucher boleh ditampilkan ulang atau sekali-lihat?~~ → **Diputuskan: boleh dilihat ulang** (mendukung input manual).
3. ~~Halaman struk/cetak untuk konter?~~ → **Diputuskan: PNG/JPG saja**, tanpa PDF/cetak.
4. ~~Satu atau dua mode deploy?~~ → **Diputuskan: dua mode** (VPS domain publik + server lokal dengan DNS statik di MikroTik); single-tenant.
