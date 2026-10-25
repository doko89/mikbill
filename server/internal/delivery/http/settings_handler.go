package http

import (
	"net/http"

	"github.com/mikbil/mikbil/internal/usecase"
)

// SettingsHandler menangani GET/PUT /api/admin/settings (tech_spec 4.4).
type SettingsHandler struct {
	settings *usecase.SettingsUsecase
}

// NewSettingsHandler membuat SettingsHandler.
func NewSettingsHandler(settings *usecase.SettingsUsecase) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

type settingsDTO struct {
	DeployMode           string `json:"deploy_mode"`
	SiteURL              string `json:"site_url"`
	LocalServerIP        string `json:"local_server_ip"`
	HotspotLoginBase     string `json:"hotspot_login_base"`
	VoucherLength        int    `json:"voucher_length"`
	KrisKrisAPIKey       string `json:"kriskris_api_key"`
	KrisKrisAPISecretSet bool   `json:"kriskris_api_secret_set"`
	KrisKrisCallbackSet  bool   `json:"kriskris_callback_secret_set"`
	BrandName            string `json:"brand_name"`
	BrandTagline         string `json:"brand_tagline"`
	AccentColor          string `json:"accent_color"`
	ThemeMode            string `json:"theme_mode"`
}

func toSettingsDTO(v usecase.SettingsView) settingsDTO {
	return settingsDTO{
		DeployMode:           v.DeployMode,
		SiteURL:              v.SiteURL,
		LocalServerIP:        v.LocalServerIP,
		HotspotLoginBase:     v.HotspotLoginBase,
		VoucherLength:        v.VoucherLength,
		KrisKrisAPIKey:       v.KrisKrisAPIKey,
		KrisKrisAPISecretSet: v.KrisKrisSecretSet,
		KrisKrisCallbackSet:  v.KrisKrisCallbackSet,
		BrandName:            v.BrandName,
		BrandTagline:         v.BrandTagline,
		AccentColor:          v.AccentColor,
		ThemeMode:            v.ThemeMode,
	}
}

// Get menangani GET /api/admin/settings.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.settings.Get()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, toSettingsDTO(v))
}

type updateSettingsBody struct {
	DeployMode             *string `json:"deploy_mode"`
	SiteURL                *string `json:"site_url"`
	LocalServerIP          *string `json:"local_server_ip"`
	HotspotLoginBase       *string `json:"hotspot_login_base"`
	VoucherLength          *int    `json:"voucher_length"`
	KrisKrisAPIKey         *string `json:"kriskris_api_key"`
	KrisKrisAPISecret      *string `json:"kriskris_api_secret"`
	KrisKrisCallbackSecret *string `json:"kriskris_callback_secret"`
	BrandName              *string `json:"brand_name"`
	BrandTagline           *string `json:"brand_tagline"`
	AccentColor            *string `json:"accent_color"`
	ThemeMode              *string `json:"theme_mode"`
}

// Update menangani PUT /api/admin/settings (parsial; nil tidak berubah).
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b updateSettingsBody
	if !decodeJSON(w, r, &b) {
		return
	}
	v, err := h.settings.Update(usecase.SettingsUpdate{
		DeployMode:             b.DeployMode,
		SiteURL:                b.SiteURL,
		LocalServerIP:          b.LocalServerIP,
		HotspotLoginBase:       b.HotspotLoginBase,
		VoucherLength:          b.VoucherLength,
		KrisKrisAPIKey:         b.KrisKrisAPIKey,
		KrisKrisAPISecret:      b.KrisKrisAPISecret,
		KrisKrisCallbackSecret: b.KrisKrisCallbackSecret,
		BrandName:              b.BrandName,
		BrandTagline:           b.BrandTagline,
		AccentColor:            b.AccentColor,
		ThemeMode:              b.ThemeMode,
	})
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, toSettingsDTO(v))
}
