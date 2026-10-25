package domain

// Kunci settings sesuai tech_spec bagian 3.
const (
	SettingSetupCompleted      = "setup_completed" // "0"/"1"
	SettingDeployMode          = "deploy_mode"     // "vps"|"local"
	SettingSiteURL             = "site_url"
	SettingLocalServerIP       = "local_server_ip"
	SettingHotspotLoginBase    = "hotspot_login_base"
	SettingVoucherLength       = "voucher_length"
	SettingKrisKrisAPIKey      = "kriskris_api_key"
	SettingKrisKrisAPISecret   = "kriskris_api_secret" // terenkripsi
	SettingKrisKrisCallbackSec = "kriskris_callback_secret"
	SettingBrandName           = "brand_name"
	SettingBrandTagline        = "brand_tagline"
	SettingAccentColor         = "accent_color"
	SettingThemeMode           = "theme_mode"
)

// DeployMode valid.
const (
	DeployVPS   = "vps"
	DeployLocal = "local"
)

// Mode tema valid (kontrak tema v1).
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Nilai bawaan tema saat settings kosong (kontrak tema v1).
const (
	DefaultBrandName    = "Mikbil"
	DefaultBrandTagline = "Beli voucher WiFi mandiri"
	DefaultAccentColor  = "#059669"
	DefaultThemeMode    = ThemeSystem
)

// ThemeLogoKey adalah kunci tunggal aset logo di theme_assets.
const ThemeLogoKey = "logo"

// ThemeAsset adalah satu berkas biner tema (logo).
type ThemeAsset struct {
	Key  string
	MIME string
	Data []byte
}

// Setting adalah pasangan key-value konfigurasi.
type Setting struct {
	Key   string
	Value string
}
