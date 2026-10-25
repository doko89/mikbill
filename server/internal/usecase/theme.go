package usecase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// Batas logo sesuai kontrak tema v1.
const (
	maxLogoBytes = 512 * 1024
	maxLogoDim   = 1024
)

// orDefault mengembalikan nilai bawaan bila kosong.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// validAccent melaporkan format #rrggbb valid.
func validAccent(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		hexDigit := c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
		if !hexDigit {
			return false
		}
	}
	return true
}

// validateThemeUpdate memvalidasi field tema dan menulis hasil trim ke merged.
func validateThemeUpdate(in SettingsUpdate, merged *SettingsView) error {
	if in.BrandName != nil {
		name := strings.TrimSpace(*in.BrandName)
		if len([]rune(name)) < 1 || len([]rune(name)) > 40 {
			return InputError{"nama brand 1 sampai 40 karakter"}
		}
		merged.BrandName = name
	}
	if in.BrandTagline != nil {
		tag := strings.TrimSpace(*in.BrandTagline)
		if len([]rune(tag)) > 80 {
			return InputError{"tagline maksimal 80 karakter"}
		}
		merged.BrandTagline = tag
	}
	if in.AccentColor != nil {
		accent := strings.TrimSpace(*in.AccentColor)
		if !validAccent(accent) {
			return InputError{"warna aksen harus heksadesimal #rrggbb"}
		}
		merged.AccentColor = accent
	}
	if in.ThemeMode != nil {
		mode := strings.TrimSpace(*in.ThemeMode)
		if mode != domain.ThemeSystem && mode != domain.ThemeLight && mode != domain.ThemeDark {
			return InputError{"mode tema harus system, light, atau dark"}
		}
		merged.ThemeMode = mode
	}
	return nil
}

// ThemeView adalah tampilan tema publik; LogoVersion kosong bila belum ada logo.
type ThemeView struct {
	BrandName    string
	BrandTagline string
	AccentColor  string
	ThemeMode    string
	LogoVersion  string // 8 heks sha256, "" bila tanpa logo
	LogoMIME     string
	LogoData     []byte // hanya diisi GetLogo
}

// ThemeUsecase menangani tema publik dan aset logo (kontrak tema v1).
type ThemeUsecase struct {
	set    SettingsRepository
	assets ThemeAssetRepository
}

// NewThemeUsecase membuat ThemeUsecase.
func NewThemeUsecase(set SettingsRepository, assets ThemeAssetRepository) *ThemeUsecase {
	return &ThemeUsecase{set: set, assets: assets}
}

// LogoVersionOf menghitung versi cache 8 heks dari bytes logo.
func LogoVersionOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:4])
}

// Get membaca tema publik dengan nilai bawaan kontrak.
func (u *ThemeUsecase) Get() (ThemeView, error) {
	v := ThemeView{
		BrandName:    orDefault(getSettingRaw(u.set, domain.SettingBrandName), domain.DefaultBrandName),
		BrandTagline: orDefault(getSettingRaw(u.set, domain.SettingBrandTagline), domain.DefaultBrandTagline),
		AccentColor:  orDefault(getSettingRaw(u.set, domain.SettingAccentColor), domain.DefaultAccentColor),
		ThemeMode:    orDefault(getSettingRaw(u.set, domain.SettingThemeMode), domain.DefaultThemeMode),
	}
	if u.assets != nil {
		if a, found, err := u.assets.Get(domain.ThemeLogoKey); err != nil {
			return ThemeView{}, err
		} else if found {
			v.LogoVersion = LogoVersionOf(a.Data)
		}
	}
	return v, nil
}

func getSettingRaw(set SettingsRepository, key string) string {
	v, found, err := set.Get(key)
	if err != nil || !found {
		return ""
	}
	return v
}

// ValidateLogo memeriksa tipe, ukuran, dan dimensi bytes logo.
func ValidateLogo(mime string, data []byte) error {
	if mime != "image/png" && mime != "image/jpeg" {
		return InputError{"logo harus berkas PNG atau JPEG"}
	}
	if len(data) > maxLogoBytes {
		return InputError{"ukuran logo maksimal 512KB"}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return InputError{"berkas logo tidak valid"}
	}
	if cfg.Width > maxLogoDim || cfg.Height > maxLogoDim {
		return InputError{"dimensi logo maksimal 1024px"}
	}
	return nil
}

// SaveLogo memvalidasi lalu menyimpan logo sebagai satu-satunya aset.
func (u *ThemeUsecase) SaveLogo(mime string, data []byte) (ThemeView, error) {
	if err := ValidateLogo(mime, data); err != nil {
		return ThemeView{}, err
	}
	if err := u.assets.Set(domain.ThemeLogoKey, mime, data); err != nil {
		return ThemeView{}, err
	}
	return u.Get()
}

// DeleteLogo menghapus logo; tanpa logo bukan error.
func (u *ThemeUsecase) DeleteLogo() error {
	return u.assets.Delete(domain.ThemeLogoKey)
}

// GetLogo mengambil bytes logo untuk disajikan publik.
func (u *ThemeUsecase) GetLogo() (ThemeView, bool, error) {
	a, found, err := u.assets.Get(domain.ThemeLogoKey)
	if err != nil || !found {
		return ThemeView{}, found, err
	}
	return ThemeView{
		LogoVersion: LogoVersionOf(a.Data),
		LogoMIME:    a.MIME,
		LogoData:    a.Data,
	}, true, nil
}
