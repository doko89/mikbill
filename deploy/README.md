# Deploy Mikbil (Linux)

## Pasang cepat

```sh
curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash
```

Pasang versi tertentu:

```sh
curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash -s -- --version v1.2.0
```

Tanpa `--version`, skrip memakai rilis `latest` (nama aset ditemukan dari
`checksums.txt` rilis). Unduhan selalu diverifikasi sha256 terhadap
`checksums.txt` sebelum dipasang.

Simulasi tanpa memasang (cek arsitektur, nama aset, URL):

```sh
bash deploy/install.sh --dry-run
bash deploy/install.sh --version v1.2.0 --dry-run
```

## Yang dikerjakan install.sh

- Deteksi arsitektur (`x86_64` → `amd64`, `aarch64` → `arm64`).
- Unduh `mikbil_<ver>_linux_<arch>.tar.gz` + `checksums.txt` dari GitHub Release.
- Pasang binary ke `/usr/local/bin/mikbil`; buat user/group `mikbil`.
- Buat direktori `/var/lib/mikbil` dan `/etc/mikbil`.
- Tulis `/etc/mikbil/mikbil.env` (`MIKBIL_ADDR=:8080`,
  `MIKBIL_DB=/var/lib/mikbil/mikbil.db`, `MIKBIL_ENC_KEY` dari `mikbil genkey`)
  **hanya bila belum ada** — upgrade tidak menimpa DB maupun kunci.
- Pasang + enable + start unit systemd `mikbil`, lalu tunggu
  `GET /api/health` dan cetak langkah lanjut (wizard `/setup`, firewall).

Idempotent: jalankan ulang skrip yang sama untuk upgrade ke rilis baru.

## Berkas

- `install.sh`: installer di atas.
- `mikbil.service`: unit systemd (`User=mikbil`, `EnvironmentFile=/etc/mikbil/mikbil.env`).
- `mikbil.env.contoh`: contoh env untuk pasang manual.

Isi tarball rilis Linux: `mikbil` + `mikbil.service` + `mikbil.env.contoh` + `README`.

## Copot

```sh
sudo systemctl disable --now mikbil
sudo rm -f /usr/local/bin/mikbil /etc/systemd/system/mikbil.service
sudo systemctl daemon-reload
sudo userdel mikbil
# Data + config (hapus bila yakin; cadangkan dulu bila perlu):
# sudo rm -rf /var/lib/mikbil /etc/mikbil
```

## Pasang manual

Lihat komentar di `mikbil.service` dan panduan lengkap: `docs/INSTALL.md`.
