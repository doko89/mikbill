package usecase

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// Batas panjang kode voucher (cermin payment.voucherLength).
const (
	minVoucherLength     = 6
	maxVoucherLength     = 32
	defaultVoucherLength = 8
)

// SettingsView adalah tampilan pengaturan; secret tulis-saja hanya
// dilaporkan sebagai penanda terisi, tidak pernah dibaca balik.
type SettingsView struct {
	DeployMode          string
	SiteURL             string
	LocalServerIP       string
	HotspotLoginBase    string
	VoucherLength       int
	KrisKrisAPIKey      string
	KrisKrisSecretSet   bool
	KrisKrisCallbackSet bool
}

// SettingsUpdate adalah pembaruan parsial; nil berarti tidak berubah.
type SettingsUpdate struct {
	DeployMode             *string
	SiteURL                *string
	LocalServerIP          *string
	HotspotLoginBase       *string
	VoucherLength          *int
	KrisKrisAPIKey         *string
	KrisKrisAPISecret      *string
	KrisKrisCallbackSecret *string
}

// SettingsUsecase menangani baca-tulis pengaturan admin (tech_spec 4.4).
type SettingsUsecase struct {
	set SettingsRepository
	box SecretBox
}

// NewSettingsUsecase membuat SettingsUsecase.
func NewSettingsUsecase(set SettingsRepository, box SecretBox) *SettingsUsecase {
	return &SettingsUsecase{set: set, box: box}
}

func getSetting(u *SettingsUsecase, key string) string {
	v, found, err := u.set.Get(key)
	if err != nil || !found {
		return ""
	}
	return v
}

// Get membaca seluruh pengaturan.
func (u *SettingsUsecase) Get() (SettingsView, error) {
	v := SettingsView{
		DeployMode:       getSetting(u, domain.SettingDeployMode),
		SiteURL:          getSetting(u, domain.SettingSiteURL),
		LocalServerIP:    getSetting(u, domain.SettingLocalServerIP),
		HotspotLoginBase: getSetting(u, domain.SettingHotspotLoginBase),
		VoucherLength:    defaultVoucherLength,
		KrisKrisAPIKey:   getSetting(u, domain.SettingKrisKrisAPIKey),
	}
	if n, err := strconv.Atoi(strings.TrimSpace(getSetting(u, domain.SettingVoucherLength))); err == nil && n >= minVoucherLength && n <= maxVoucherLength {
		v.VoucherLength = n
	}
	v.KrisKrisSecretSet = getSetting(u, domain.SettingKrisKrisAPISecret) != ""
	v.KrisKrisCallbackSet = getSetting(u, domain.SettingKrisKrisCallbackSec) != ""
	return v, nil
}

// Update menyimpan perubahan parsial; grup situs divalidasi gabungan
// (nilai baru di atas nilai tersimpan) hanya bila ada field situs diubah.
func (u *SettingsUsecase) Update(in SettingsUpdate) (SettingsView, error) {
	cur, err := u.Get()
	if err != nil {
		return SettingsView{}, err
	}
	merged := cur
	if in.DeployMode != nil {
		merged.DeployMode = strings.TrimSpace(*in.DeployMode)
	}
	if in.SiteURL != nil {
		merged.SiteURL = strings.TrimSpace(*in.SiteURL)
	}
	if in.LocalServerIP != nil {
		merged.LocalServerIP = strings.TrimSpace(*in.LocalServerIP)
	}
	if in.HotspotLoginBase != nil {
		merged.HotspotLoginBase = strings.TrimSpace(*in.HotspotLoginBase)
	}
	if in.DeployMode != nil || in.SiteURL != nil || in.LocalServerIP != nil || in.HotspotLoginBase != nil {
		if err := validateSite(merged.DeployMode, merged.SiteURL, merged.LocalServerIP, merged.HotspotLoginBase); err != nil {
			return SettingsView{}, err
		}
	}
	if in.VoucherLength != nil {
		if *in.VoucherLength < minVoucherLength || *in.VoucherLength > maxVoucherLength {
			return SettingsView{}, InputError{"panjang kode voucher 6 sampai 32 karakter"}
		}
		merged.VoucherLength = *in.VoucherLength
	}
	if in.KrisKrisAPIKey != nil && strings.TrimSpace(*in.KrisKrisAPIKey) == "" {
		return SettingsView{}, InputError{"API key wajib diisi"}
	}
	for _, s := range []struct {
		val *string
		msg string
	}{
		{in.KrisKrisAPISecret, "API secret wajib diisi"},
		{in.KrisKrisCallbackSecret, "callback secret wajib diisi"},
	} {
		if s.val != nil && *s.val == "" {
			return SettingsView{}, InputError{s.msg}
		}
	}

	writes := map[string]string{}
	if in.DeployMode != nil {
		writes[domain.SettingDeployMode] = merged.DeployMode
	}
	if in.SiteURL != nil {
		writes[domain.SettingSiteURL] = merged.SiteURL
	}
	if in.LocalServerIP != nil {
		writes[domain.SettingLocalServerIP] = merged.LocalServerIP
	}
	if in.HotspotLoginBase != nil {
		writes[domain.SettingHotspotLoginBase] = merged.HotspotLoginBase
	}
	if in.VoucherLength != nil {
		writes[domain.SettingVoucherLength] = strconv.Itoa(merged.VoucherLength)
	}
	if in.KrisKrisAPIKey != nil {
		writes[domain.SettingKrisKrisAPIKey] = strings.TrimSpace(*in.KrisKrisAPIKey)
	}
	for _, s := range []struct {
		val *string
		key string
	}{
		{in.KrisKrisAPISecret, domain.SettingKrisKrisAPISecret},
		{in.KrisKrisCallbackSecret, domain.SettingKrisKrisCallbackSec},
	} {
		if s.val == nil {
			continue
		}
		enc, err := u.box.Seal(*s.val)
		if err != nil {
			return SettingsView{}, err
		}
		writes[s.key] = enc
	}
	for k, v := range writes {
		if err := u.set.Set(k, v); err != nil {
			return SettingsView{}, err
		}
	}
	return u.Get()
}

// validateSite memvalidasi grup alamat situs (aturan sama dengan setup).
func validateSite(mode, siteURL, localIP, loginBase string) error {
	if mode != domain.DeployVPS && mode != domain.DeployLocal {
		return InputError{"mode deploy harus vps atau local"}
	}
	su, err := url.ParseRequestURI(siteURL)
	if err != nil || (su.Scheme != "https" && su.Scheme != "http") || su.Host == "" {
		return InputError{"alamat situs tidak valid"}
	}
	if mode == domain.DeployVPS && su.Scheme != "https" {
		return InputError{"mode VPS wajib memakai HTTPS"}
	}
	if mode == domain.DeployLocal && net.ParseIP(localIP) == nil {
		return InputError{"IP server lokal tidak valid"}
	}
	lu, err := url.ParseRequestURI(loginBase)
	if err != nil || (lu.Scheme != "https" && lu.Scheme != "http") || lu.Host == "" {
		return InputError{"alamat login hotspot tidak valid"}
	}
	return nil
}
