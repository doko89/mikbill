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
)

// DeployMode valid.
const (
	DeployVPS   = "vps"
	DeployLocal = "local"
)

// Setting adalah pasangan key-value konfigurasi.
type Setting struct {
	Key   string
	Value string
}
