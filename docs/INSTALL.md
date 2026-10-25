# Panduan Instalasi Mikbil

Satu binary Go menyajikan backend + frontend (React hasil embed) + database
SQLite satu file. Cara termudah: skrip instalasi rilis (§2–§3). Isi rilis:
`mikbil_<ver>_linux_amd64.tar.gz`, `mikbil_<ver>_linux_arm64.tar.gz`
(isi: `mikbil` + `mikbil.service` + `mikbil.env.contoh` + `README`),
`mikbil_<ver>_windows_amd64.zip` (isi: `mikbil.exe` + `README`),
plus `checksums.txt` SHA256 semua aset.

Panduan ini mencakup instalasi di VPS (domain publik) dan server lokal
(mini PC di LAN), untuk Linux dan Windows. Bahasa: Indonesia.

## 1. Prasyarat

| Kebutuhan | Ket |
|---|---|
| OS | Linux 64-bit (amd64/arm64, systemd) atau Windows 10/11 / Server 2016+ (PowerShell 5.1+) |
| NTP sinkron | Wajib (signature KrisKris toleransi ±5 menit); cek `clock_skew_sec` via health-check (§8) |
| Port 8080 | Port listen default; buka di firewall bila diakses dari LAN |
| Egress internet | Ya, ke `kriskris.id` (API invoice + polling) dan server NTP |
| Domain | VPS: domain publik + DNS A ke IP VPS. Lokal: domain lokal, misal `mikbil.lan` |

### 1.1 Pastikan NTP aktif

Linux:

```sh
timedatectl status | grep -i ntp
# System clock synchronized: yes
```

Bila belum sinkron (Ubuntu/Debian):

```sh
sudo timedatectl set-ntp true
sudo systemctl enable --now systemd-timesyncd
```

Windows (jalankan sebagai Administrator bila perlu penyesuaian):

```powershell
w32tm /query /status
```

Cek selisih jam kapan saja via health-check (lihat §8): bila
`clock_skew_sec` di atas 60 detik, perbaiki NTP sebelum menerima pembayaran.

## 2. Instalasi Linux (satu baris)

```sh
curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash
```

Ini memasang rilis **terbaru**: mengunduh tarball + `checksums.txt`,
memverifikasi SHA256, memasang binary ke `/usr/local/bin/mikbil`,
service systemd `mikbil`, env di `/etc/mikbil/mikbil.env`, dan data di
`/var/lib/mikbil`. Idempotent: aman dijalankan ulang untuk upgrade
(DB + `MIKBIL_ENC_KEY` dipertahankan).

Versi tertentu:

```sh
curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash -s -- --version v1.0.0
```

Selesai → lanjut ke wizard (§5): `http://<ip-server>:8080/setup`.

## 3. Instalasi Windows (satu baris)

Jalankan di PowerShell sebagai **Administrator**:

```powershell
irm https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.ps1 | iex
```

Ini memasang rilis **terbaru**: mengunduh zip + `checksums.txt`,
memverifikasi SHA256, mengekstrak ke `C:\Program Files\Mikbil`,
data di `C:\ProgramData\Mikbil`, menyimpan `MIKBIL_DB` +
`MIKBIL_ENC_KEY` (dibangkitkan sekali via `mikbil.exe genkey`) sebagai
env level mesin, lalu mendaftarkan Scheduled Task `Mikbil` (jalan saat
startup sebagai SYSTEM). Idempotent: DB + kunci dipertahankan saat upgrade.

Versi tertentu:

```powershell
$env:MIKBIL_VERSION = 'v1.0.0'; irm https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.ps1 | iex
```

Selesai → buka `http://localhost:8080/setup` untuk wizard (§5). Bila
diakses dari LAN, izinkan port 8080 di Windows Firewall:

```powershell
New-NetFirewallRule -DisplayName "Mikbil" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8080
```

## 4. Instalasi manual (dari aset rilis)

1. Unduh aset sesuai OS dari halaman GitHub Releases (`doko89/mikbill`)
   beserta `checksums.txt`.
2. Verifikasi SHA256 (Linux: `sha256sum -c checksums.txt`; Windows:
   `Get-FileHash <file> -Algorithm SHA256` lalu cocokkan dengan entri
   di `checksums.txt`). Jangan lanjut bila tidak cocok.
3. Linux: ekstrak tarball, lalu:
   ```sh
   sudo cp mikbil /usr/local/bin/mikbil
   sudo useradd --system --home /var/lib/mikbil --create-home --shell /usr/sbin/nologin mikbil
   sudo mkdir -p /etc/mikbil
   sudo cp mikbil.env.contoh /etc/mikbil/mikbil.env   # lalu isi MIKBIL_ENC_KEY dkk
   sudo chown mikbil:mikbil /etc/mikbil/mikbil.env && sudo chmod 600 /etc/mikbil/mikbil.env
   sudo cp mikbil.service /etc/systemd/system/mikbil.service
   sudo systemctl daemon-reload && sudo systemctl enable --now mikbil
   ```
4. Windows: ekstrak zip ke `C:\Program Files\Mikbil`, buat
   `C:\ProgramData\Mikbil`, set env mesin `MIKBIL_DB` +
   `MIKBIL_ENC_KEY` (dari `mikbil.exe genkey`), lalu daftarkan
   Scheduled Task setara §3 atau jalankan manual:
   ```powershell
   [Environment]::SetEnvironmentVariable('MIKBIL_DB', 'C:\ProgramData\Mikbil\mikbil.db', 'Machine')
   [Environment]::SetEnvironmentVariable('MIKBIL_ENC_KEY', (& 'C:\Program Files\Mikbil\mikbil.exe' genkey), 'Machine')
   ```

Buat kunci enkripsi (sekali saja, simpan aman):

```sh
mikbil genkey
```

Variabel env:

| Var | Default | Ket |
|---|---|---|
| `MIKBIL_ADDR` | `:8080` | Alamat listen. Di balik reverse proxy: `127.0.0.1:8080` |
| `MIKBIL_DB` | `./mikbil.db` | Path file SQLite, misal `/var/lib/mikbil/mikbil.db` |
| `MIKBIL_ENC_KEY` | wajib | Hex 64 char dari `mikbil genkey` |
| `MIKBIL_DEBUG_KRISKRIS` | kosong | Isi `1` untuk log request/response kriskris.id ke stderr (secret disamarkan) |

Jalankan manual (tanpa service, misal untuk coba lokal):

```sh
export MIKBIL_ADDR=:8080 MIKBIL_DB=./mikbil.db MIKBIL_ENC_KEY=<hex>
./mikbil migrate   # opsional; serve juga auto-migrate
./mikbil serve
```

## 5. Setup Wizard (first-run)

1. Buka alamat Mikbil di browser (VPS: `https://wifi.contoh.com`,
   lokal: `http://mikbil.lan` setelah DNS statik §6.2, atau `http://<ip-server>:8080`
   untuk langkah awal; Windows lokal: `http://localhost:8080/setup`).
2. Isi 5 langkah: akun admin → alamat Mikbil → MikroTik (tombol Test koneksi
   wajib sukses) → KrisKris → Provision (tombol Aktifkan akses).
3. Selesai → login di `/admin`.

## 6. Provisioning per Mode

Wizard menjalankan provisioning otomatis (langkah 5) atau ulangi kapan saja
dari `/admin/router` (tombol Terapkan) dan verifikasi dengan tombol
Cek & perbaiki:

### 6.1 Mode VPS

- Walled garden: `dst-host=<domain-publik>` (`comment=mikbil`).
- `http-pap` ditambahkan aditif ke `login-by` hotspot server profile
  (mode login lain tidak diubah).
- Isi callback URL di dashboard KrisKris:
  `https://<domain-publik>/api/webhooks/kriskris` + callback secret yang sama
  dengan yang disimpan di wizard langkah 4.

### 6.2 Mode server lokal

- Walled garden: `dst-address=<ip-server>` + `dst-host=<domain-lokal>`.
- DNS statik di MikroTik: `name=<domain-lokal>` → `address=<ip-server>`;
  Mikbil mengecek `allow-remote-requests=yes` (diubah + dicatat bila belum).
- Webhook KrisKris tidak akan sampai (tanpa URL publik): Mikbil otomatis
  mengandalkan polling tiap 5 detik. Tidak ada konfigurasi tambahan.
  Callback secret tidak perlu diisi (field-nya disembunyikan di wizard).
- Pastikan HP pembeli memakai router sebagai DNS (default klien hotspot).

### 6.3 Template login hotspot

Pilih salah satu (keduanya tersedia di `/admin/router`):

- (a) Unduh `login.html` jadi dari Mikbil, upload ke router
  (`Files > hotspot > login.html`), atau
- (b) Tempel snippet tombol Beli Voucher ke `login.html` existing.

## 7. TLS via Reverse Proxy (mode VPS)

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

Mikbil membaca IP klien dari `X-Forwarded-For` untuk rate-limit dan skema
dari `X-Forwarded-Proto` untuk flag Secure cookie sesi; pastikan kedua header
tersebut diteruskan. Mode VPS wajib HTTPS valid, kalau tidak wizard menolak
`site_url` non-HTTPS.

## 8. Health-Check dan Operasional

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

## 9. Kelola layanan

Linux (systemd):

```sh
sudo systemctl status mikbil
sudo systemctl restart mikbil
sudo systemctl stop mikbil
sudo journalctl -u mikbil -f   # lihat log
```

Windows (Scheduled Task `Mikbil`, sebagai Administrator):

```powershell
Get-ScheduledTask -TaskName Mikbil
Start-ScheduledTask -TaskName Mikbil
Stop-ScheduledTask -TaskName Mikbil
# atau via GUI: Task Scheduler > Task Scheduler Library > Mikbil
```

## 10. Upgrade

1. Backup `.db` + env/kunci (§11).
2. Linux: jalankan ulang perintah satu baris §2 (tambah `--version`
   bila ingin versi tertentu). Windows: jalankan ulang perintah §3.
   Keduanya idempotent: DB + kunci dipertahankan, migrasi DB otomatis.
3. Cek `mikbil version` dan `/api/health`.

## 11. Backup dan Restore

Satu-satunya state adalah file `.db` (path `MIKBIL_DB`).

Backup Linux (hentikan tulis sesaat agar konsisten):

```sh
sudo systemctl stop mikbil
sudo cp /var/lib/mikbil/mikbil.db /var/backups/mikbil-$(date +%F).db
sudo systemctl start mikbil
```

Atau tanpa henti via snapshot SQLite (butuh `sqlite3`):

```sh
sqlite3 /var/lib/mikbil/mikbil.db ".backup '/var/backups/mikbil-$(date +%F).db'"
```

Backup Windows (sebagai Administrator; hentikan task dulu):

```powershell
Stop-ScheduledTask -TaskName Mikbil
Copy-Item 'C:\ProgramData\Mikbil\mikbil.db' "C:\ProgramData\Mikbil\mikbil-$(Get-Date -Format yyyy-MM-dd).db"
Start-ScheduledTask -TaskName Mikbil
```

Restore: hentikan service/task, salin file kembali, pastikan kepemilikan
benar (Linux: owner `mikbil`), lalu start. **Jangan lupa backup kunci**:
Linux `/etc/mikbil/mikbil.env`, Windows env mesin `MIKBIL_ENC_KEY` —
tanpa kunci yang sama, password router/voucher tak bisa dibuka.

## 12. Uninstall

Linux:

```sh
sudo systemctl disable --now mikbil
sudo rm /etc/systemd/system/mikbil.service /usr/local/bin/mikbil
sudo systemctl daemon-reload
# Opsional, hapus data + env (tak bisa dibatalkan):
# sudo rm -rf /var/lib/mikbil /etc/mikbil
# sudo userdel mikbil
```

Windows (sebagai Administrator):

```powershell
Stop-ScheduledTask -TaskName Mikbil -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName Mikbil -Confirm:$false
Remove-Item 'C:\Program Files\Mikbil' -Recurse -Force
[Environment]::SetEnvironmentVariable('MIKBIL_DB', $null, 'Machine')
[Environment]::SetEnvironmentVariable('MIKBIL_ENC_KEY', $null, 'Machine')
Remove-NetFirewallRule -DisplayName 'Mikbil' -ErrorAction SilentlyContinue
# Opsional, hapus data (tak bisa dibatalkan):
# Remove-Item 'C:\ProgramData\Mikbil' -Recurse -Force
```

## 13. Troubleshoot

| Gejala | Cek |
|---|---|
| Semua request KrisKris 401 | NTP (§1.1) dan API key/secret di wizard |
| Webhook tak sampai (VPS) | Callback URL + secret di dashboard KrisKris; log (`journalctl -u mikbil` / Task Scheduler) |
| Order pending terus (lokal) | Normal memakai polling 5 detik; cek egress internet server |
| HP belum-login tak bisa buka Mikbil | Tombol Cek & perbaiki di `/admin/router`; walled garden + DNS statik |
| Tombol Login WiFi gagal | `http-pap` aktif? (status provisioning); fallback: salin kode manual |
| Login admin 429 | Tunggu 1 menit (rate-limit 5/menit) |
| SHA256 skrip instalasi tidak cocok | Unduh ulang; jangan paksa pasang bila tetap gagal |
| `MIKBIL_ENC_KEY tidak valid` saat start | Env salah/hilang; kembalikan kunci lama dari backup, jangan buat baru |
