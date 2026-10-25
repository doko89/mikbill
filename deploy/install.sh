#!/usr/bin/env bash
#
# Mikbil — installer Linux + systemd.
#
# Pemakaian:
#   curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash
#   curl -fsSL https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.sh | sudo bash -s -- --version v1.2.0
#
# Idempotent: aman dijalankan ulang untuk upgrade.
# DB (/var/lib/mikbil) dan /etc/mikbil/mikbil.env tidak pernah ditimpa.
set -euo pipefail

REPO="doko89/mikbill"
VERSION="latest"
DRY_RUN=0

usage() {
	cat <<'EOF'
Pakai: install.sh [--version vX.Y.Z|latest] [--dry-run]

  --version vX.Y.Z  pasang rilis tertentu (awalan "v" opsional).
                    Default: latest (aset ditemukan dari checksums.txt rilis).
  --dry-run         hanya tampilkan arsitektur, nama aset, dan URL unduh.
  -h, --help        tampilkan bantuan ini.
EOF
}

log() { printf '[mikbil] %s\n' "$*"; }
die() { printf '[mikbil] GALAT: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || die "opsi --version butuh nilai, misal --version v1.2.0"
		VERSION="$2"
		shift 2
		;;
	--version=*)
		VERSION="${1#--version=}"
		[ -n "$VERSION" ] || die "opsi --version butuh nilai, misal --version=v1.2.0"
		shift
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "argumen tak dikenal: $1 (lihat --help)"
		;;
	esac
done

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) die "arsitektur tak didukung: $(uname -m) (hanya x86_64 dan aarch64)" ;;
	esac
}

GOARCH="$(detect_arch)" || exit 1

# Selesaikan nama tag/aset/URL. Aset rilis: mikbil_<ver>_linux_<arch>.tar.gz
# dengan <ver> tanpa awalan "v" (misal tag v1.2.0 -> mikbil_1.2.0_linux_amd64.tar.gz).
TAG=""
ASSET=""
BASE_URL=""
if [ "$VERSION" = "latest" ]; then
	BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
	TAG="$VERSION"
	case "$TAG" in v*) ;; *) TAG="v${TAG}" ;; esac
	ASSET="mikbil_${TAG#v}_linux_${GOARCH}.tar.gz"
	BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
fi

if [ "$DRY_RUN" = "1" ]; then
	echo "[simulasi] mesin: $(uname -m) -> linux_${GOARCH}"
	echo "[simulasi] versi diminta: ${VERSION}"
	if [ -n "$ASSET" ]; then
		echo "[simulasi] tag: ${TAG}"
		echo "[simulasi] aset: ${ASSET}"
		echo "[simulasi] unduh: ${BASE_URL}/${ASSET}"
		echo "[simulasi] unduh: ${BASE_URL}/checksums.txt"
	else
		echo "[simulasi] aset: mikbil_<ver>_linux_${GOARCH}.tar.gz (nama pasti ditemukan dari checksums.txt)"
		echo "[simulasi] unduh: ${BASE_URL}/checksums.txt lalu ${BASE_URL}/<aset>"
	fi
	echo "[simulasi] verifikasi: sha256 terhadap checksums.txt rilis"
	echo "[simulasi] pasang: /usr/local/bin/mikbil, user mikbil, /var/lib/mikbil, /etc/mikbil/mikbil.env (baru), systemd mikbil.service"
	exit 0
fi

[ "$(id -u)" = "0" ] || die "jalankan sebagai root (misal via sudo bash)"
for dep in curl tar systemctl sha256sum; do
	command -v "$dep" >/dev/null 2>&1 || die "butuh '$dep'; pasang dulu (misal apt install -y $dep)"
done

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"

log "mengunduh checksums.txt dari ${BASE_URL} ..."
curl -fsSL -o checksums.txt "${BASE_URL}/checksums.txt"

if [ -z "$ASSET" ]; then
	ASSET="$(grep -oE 'mikbil_[^[:space:]"'\'']*_linux_'"${GOARCH}"'\.tar\.gz' checksums.txt | head -n 1 || true)"
	[ -n "$ASSET" ] || die "aset linux_${GOARCH} tidak ditemukan di checksums.txt rilis latest"
	log "rilis latest memakai aset ${ASSET}"
fi

log "mengunduh ${ASSET} ..."
curl -fsSL -o "$ASSET" "${BASE_URL}/${ASSET}"

log "memverifikasi sha256 ..."
awk -v f="$ASSET" '{h=$1; n=$2; sub(/^\*/, "", n); if (n==f) print h"  "f}' checksums.txt >verify.sum
[ -s verify.sum ] || die "hash ${ASSET} tidak ada di checksums.txt"
sha256sum -c verify.sum

log "mengekstrak ..."
tar -xzf "$ASSET"
BIN="$(find "$TMP" -maxdepth 2 -name mikbil -type f | head -n 1)"
[ -n "$BIN" ] || die "binary mikbil tidak ada di ${ASSET}"
SVC="$(find "$TMP" -maxdepth 2 -name mikbil.service -type f | head -n 1)"
[ -n "$SVC" ] || die "mikbil.service tidak ada di ${ASSET}"

if ! getent group mikbil >/dev/null; then
	groupadd -r mikbil
fi
if ! id -u mikbil >/dev/null 2>&1; then
	NOLOGIN=/usr/sbin/nologin
	[ -x "$NOLOGIN" ] || NOLOGIN=/bin/false
	useradd -r -g mikbil -s "$NOLOGIN" -d /var/lib/mikbil -M mikbil
fi
mkdir -p /var/lib/mikbil /etc/mikbil

install -m 0755 "$BIN" /usr/local/bin/mikbil
log "binary terpasang: $(/usr/local/bin/mikbil version)"

if [ -f /etc/mikbil/mikbil.env ]; then
	log "/etc/mikbil/mikbil.env sudah ada — dipertahankan (upgrade aman)"
else
	ENC_KEY="$(/usr/local/bin/mikbil genkey)"
	{
		echo "# Mikbil — dibuat oleh install.sh. Jangan ubah MIKBIL_ENC_KEY setelah data tersimpan."
		echo "MIKBIL_ADDR=:8080"
		echo "MIKBIL_DB=/var/lib/mikbil/mikbil.db"
		echo "MIKBIL_ENC_KEY=${ENC_KEY}"
	} >/etc/mikbil/mikbil.env
	chown root:mikbil /etc/mikbil/mikbil.env
	chmod 0640 /etc/mikbil/mikbil.env
	log "konfigurasi baru ditulis ke /etc/mikbil/mikbil.env"
fi
chown -R mikbil:mikbil /var/lib/mikbil

install -m 0644 "$SVC" /etc/systemd/system/mikbil.service
systemctl daemon-reload
systemctl enable mikbil >/dev/null
systemctl restart mikbil

PORT="$(sed -n 's/^MIKBIL_ADDR=.*:\([0-9][0-9]*\).*/\1/p' /etc/mikbil/mikbil.env | tail -n 1)"
[ -n "$PORT" ] || PORT=8080

log "menunggu sehat di http://127.0.0.1:${PORT}/api/health ..."
OK=0
for _ in $(seq 1 30); do
	if curl -fs -m 2 "http://127.0.0.1:${PORT}/api/health" >/dev/null 2>&1; then
		OK=1
		break
	fi
	sleep 2
done
[ "$OK" = "1" ] || die "service tidak sehat setelah 60 detik; periksa 'systemctl status mikbil' dan 'journalctl -u mikbil -n 50'"

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$IP" ] || IP="<ip-server>"
cat <<EOF
[berhasil] Mikbil berjalan dan sehat.

Langkah lanjut:
  1. Buka wizard penyiapan awal: http://${IP}:${PORT}/setup
  2. Bila port belum bisa diakses dari luar, buka firewall, misal:
EOF
if command -v ufw >/dev/null 2>&1; then
	echo "       sudo ufw allow ${PORT}/tcp"
elif command -v firewall-cmd >/dev/null 2>&1; then
	echo "       sudo firewall-cmd --permanent --add-port=${PORT}/tcp && sudo firewall-cmd --reload"
else
	echo "       (sesuaikan dengan firewall yang dipakai)"
fi
cat <<EOF
  3. Perintah berguna:
       systemctl status mikbil
       journalctl -u mikbil -f
       mikbil version
EOF
