# Panduan Instalasi Mikbil

Satu binary Go menyajikan backend + frontend (React hasil embed) + database
SQLite satu file. Panduan ini mencakup instalasi di VPS (domain publik) dan
server lokal (mini PC/RPi di LAN). Bahasa: Indonesia.

## 1. Prasyarat

| Kebutuhan | VPS | Server lokal |
|---|---|---|
| OS | Linux 64-bit (systemd) | Linux 64-bit (systemd) |
| Akses internet egress | Ya (API KrisKris + NTP) | Ya (API KrisKris + NTP) |
| Domain | Domain publik + DNS A ke IP VPS | Domain lokal, misal `mikbil.lan` |
| TLS | Reverse proxy (Caddy/nginx), sertifikat valid | Tidak perlu (HTTP di LAN) |
| NTP sinkron | Wajib (signature KrisKris toleransi ±5 menit) | Wajib |

### 1.1 Pastikan NTP aktif

```sh
timedatectl status | grep -i ntp
# System clock synchronized: yes
```

Bila belum sinkron (Ubuntu/Debian):

```sh
sudo timedatectl set-ntp true
sudo systemctl enable --now systemd-timesyncd
```

Cek selisih jam kapan saja via health-check (lihat §6): bila
`clock_skew_sec` di atas 60 detik, perbaiki NTP sebelum menerima pembayaran.

### 1.2 Siapkan akun dan direktori

```sh
sudo useradd --system --home /var/lib/mikbil --create-home --shell /usr/sbin/nologin mikbil
sudo mkdir -p /etc/mikbil
```

## 2. Pasang Binary

Unduh/salin binary `mikbil` hasil `make build`, lalu:

```sh
sudo cp mikbil /usr/local/bin/mikbil
sudo chmod 755 /usr/local/bin/mikbil
mikbil version
```

Buat kunci enkripsi (sekali saja, simpan aman):

```sh
mikbil genkey
```

Isi env:

```sh
sudo cp deploy/mikbil.env.contoh /etc/mikbil/mikbil.env
sudo nano /etc/mikbil/mikbil.env   # isi MIKBIL_ENC_KEY dkk
sudo chown mikbil:mikbil /etc/mikbil/mikbil.env
sudo chmod 600 /etc/mikbil/mikbil.env
```

Variabel env:

| Var | Default | Ket |
|---|---|---|
| `MIKBIL_ADDR` | `:8080` | Alamat listen. Di balik reverse proxy: `127.0.0.1:8080` |
| `MIKBIL_DB` | `./mikbil.db` | Path file SQLite, misal `/var/lib/mikbil/mikbil.db` |
| `MIKBIL_ENC_KEY` | wajib | Hex 64 char dari `mikbil genkey` |
| `MIKBIL_DEBUG_KRISKRIS` | kosong | Isi `1` untuk log request/response kriskris.id ke stderr (secret disamarkan) |

Pasang dan jalankan service:

```sh
sudo cp deploy/mikbil.service /etc/systemd/system/mikbil.service
sudo systemctl daemon-reload
sudo systemctl enable --now mikbil
sudo systemctl status mikbil
sudo journalctl -u mikbil -f   # lihat log
```

Jalankan manual (tanpa systemd, misal untuk coba lokal):

```sh
export MIKBIL_ADDR=:8080 MIKBIL_DB=./mikbil.db MIKBIL_ENC_KEY=<hex>
./mikbil migrate   # opsional; serve juga auto-migrate
./mikbil serve
```

## 3. Setup Wizard (first-run)

1. Buka alamat Mikbil di browser (VPS: `https://wifi.contoh.com`,
   lokal: `http://mikbil.lan` setelah DNS statik §4.2, atau `http://<ip-server>:8080`
   untuk langkah awal).
2. Isi 5 langkah: akun admin → alamat Mikbil → MikroTik (tombol Test koneksi
   wajib sukses) → KrisKris → Provision (tombol Aktifkan akses).
3. Selesai → login di `/admin`.

## 4. Provisioning per Mode

Wizard menjalankan provisioning otomatis (langkah 5) atau ulangi kapan saja
dari `/admin/router` (tombol Terapkan) dan verifikasi dengan tombol
Cek & perbaiki:

### 4.1 Mode VPS

- Walled garden: `dst-host=<domain-publik>` (`comment=mikbil`).
- `http-pap` ditambahkan aditif ke `login-by` hotspot server profile
  (mode login lain tidak diubah).
- Isi callback URL di dashboard KrisKris:
  `https://<domain-publik>/api/webhooks/kriskris` + callback secret yang sama
  dengan yang disimpan di wizard langkah 4.

### 4.2 Mode server lokal

- Walled garden: `dst-address=<ip-server>` + `dst-host=<domain-lokal>`.
- DNS statik di MikroTik: `name=<domain-lokal>` → `address=<ip-server>`;
  Mikbil mengecek `allow-remote-requests=yes` (diubah + dicatat bila belum).
- Webhook KrisKris tidak akan sampai (tanpa URL publik): Mikbil otomatis
  mengandalkan polling tiap 5 detik. Tidak ada konfigurasi tambahan.
  Callback secret tidak perlu diisi (field-nya disembunyikan di wizard).
- Pastikan HP pembeli memakai router sebagai DNS (default klien hotspot).

### 4.3 Template login hotspot

Pilih salah satu (keduanya tersedia di `/admin/router`):

- (a) Unduh `login.html` jadi dari Mikbil, upload ke router
  (`Files > hotspot > login.html`), atau
- (b) Tempel snippet tombol Beli Voucher ke `login.html` existing.

## 5. TLS via Reverse Proxy (mode VPS)

Go berjalan HTTP di `127.0.0.1:8080`; TLS di-terminate di reverse proxy.
Contoh Caddy (sertifikat otomatis):

```caddy
wifi.contoh.com {
    reverse_proxy 127.0.0.1:8080
}
```

Contoh nginx (sertifikat via certbot):

```nginx
server {
    listen 443 ssl;
    server_name wifi.contoh.com;
    ssl_certificate /etc/letsencrypt/live/wifi.contoh.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wifi.contoh.com/privkey.pem;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Mikbil membaca IP klien dari `X-Forwarded-For` untuk rate-limit; pastikan
header tersebut diteruskan. Mode VPS wajib HTTPS valid, kalau tidak wizard
menolak `site_url` non-HTTPS.

## 6. Health-Check dan Operasional

```sh
curl -s http://127.0.0.1:8080/api/health | jq .
```

Respons:

```json
{"ok":true,"data":{
  "time":"2026-10-23T10:00:00Z",
  "db":"ok",
  "clock_skew_sec":1.2,
  "router_last_test":{"at":"2026-10-23T09:00:00Z","ok":true,"message":"..."}
}}
```

- `clock_skew_sec` null = belum ada panggilan KrisKris sejak start;
  angka = selisih detik jam server KrisKris terhadap jam lokal.
- `router_last_test` null = belum ada koneksi router / belum pernah dites.
- `db:"gagal"` + HTTP 503 = database bermasalah, cek log dan path `MIKBIL_DB`.

Batas laju (per menit per IP): login admin 5, buat order 10, lihat order 30
per order, webhook 120. Respons berlebih: HTTP 429 + header `Retry-After`.

## 7. Backup dan Restore

Satu-satunya state adalah file `.db` (path `MIKBIL_DB`).

Backup (hentikan tulis sesaat agar konsisten):

```sh
sudo systemctl stop mikbil
sudo cp /var/lib/mikbil/mikbil.db /var/backups/mikbil-$(date +%F).db
sudo systemctl start mikbil
```

Atau tanpa henti via snapshot SQLite (butuh `sqlite3`):

```sh
sqlite3 /var/lib/mikbil/mikbil.db ".backup '/var/backups/mikbil-$(date +%F).db'"
```

Restore: hentikan service, salin file kembali, pastikan owner `mikbil`,
lalu start. **Jangan lupa backup `/etc/mikbil/mikbil.env`**: tanpa
`MIKBIL_ENC_KEY` yang sama, password router/voucher tak bisa dibuka.

## 8. Upgrade

1. Backup `.db` + env (§7).
2. Hentikan service, ganti `/usr/local/bin/mikbil` dengan binary baru.
3. Start service (migrasi DB berjalan otomatis).
4. Cek `mikbil version` dan `/api/health`.

## 9. Troubleshoot

| Gejala | Cek |
|---|---|
| Semua request KrisKris 401 | NTP (§1.1) dan API key/secret di wizard |
| Webhook tak sampai (VPS) | Callback URL + secret di dashboard KrisKris; log `journalctl -u mikbil` |
| Order pending terus (lokal) | Normal memakai polling 5 detik; cek egress internet server |
| HP belum-login tak bisa buka Mikbil | Tombol Cek & perbaiki di `/admin/router`; walled garden + DNS statik |
| Tombol Login WiFi gagal | `http-pap` aktif? (status provisioning); fallback: salin kode manual |
| Login admin 429 | Tunggu 1 menit (rate-limit 5/menit) |
