package http

import (
	"net/http"

	"github.com/mikbil/mikbil/internal/usecase"
)

// SetupHandler menangani API wizard penyiapan (tech_spec 4.3).
type SetupHandler struct {
	setup *usecase.SetupUsecase
}

// NewSetupHandler membuat SetupHandler.
func NewSetupHandler(setup *usecase.SetupUsecase) *SetupHandler {
	return &SetupHandler{setup: setup}
}

// Guard menolak semua endpoint setup dengan 404 bila penyiapan selesai.
func (h *SetupHandler) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done, err := h.setup.Completed()
		if err != nil {
			WriteErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		if done {
			WriteErr(w, http.StatusNotFound, "tidak ditemukan")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Status melaporkan status penyiapan (tetap aktif setelah finish, untuk guard first-run).
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	done, hasAdmin, err := h.setup.Status()
	if err != nil {
		WriteErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	WriteOK(w, http.StatusOK, map[string]any{
		"setup_completed": done,
		"has_admin":       hasAdmin,
	})
}

type createAdminBody struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateAdmin menangani POST /api/setup/admin.
func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	var b createAdminBody
	if !decodeJSON(w, r, &b) {
		return
	}
	a, err := h.setup.CreateAdmin(usecase.CreateAdminInput{
		Name: b.Name, Username: b.Username, Password: b.Password,
	})
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, map[string]any{
		"id": a.ID, "name": a.Name, "username": a.Username,
	})
}

type saveSiteBody struct {
	DeployMode       string `json:"deploy_mode"`
	SiteURL          string `json:"site_url"`
	LocalServerIP    string `json:"local_server_ip"`
	HotspotLoginBase string `json:"hotspot_login_base"`
}

// SaveSite menangani POST /api/setup/site.
func (h *SetupHandler) SaveSite(w http.ResponseWriter, r *http.Request) {
	var b saveSiteBody
	if !decodeJSON(w, r, &b) {
		return
	}
	if err := h.setup.SaveSite(usecase.SaveSiteInput{
		DeployMode: b.DeployMode, SiteURL: b.SiteURL,
		LocalServerIP: b.LocalServerIP, HotspotLoginURL: b.HotspotLoginBase,
	}); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"saved": true})
}

type saveConnectionBody struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"use_tls"`
}

// SaveConnection menangani POST /api/setup/connection.
func (h *SetupHandler) SaveConnection(w http.ResponseWriter, r *http.Request) {
	var b saveConnectionBody
	if !decodeJSON(w, r, &b) {
		return
	}
	c, err := h.setup.SaveConnection(usecase.SaveConnectionInput{
		Host: b.Host, Port: b.Port, Username: b.Username,
		Password: b.Password, UseTLS: b.UseTLS,
	})
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, map[string]any{
		"id": c.ID, "host": c.Host, "port": c.Port,
		"username": c.Username, "use_tls": c.UseTLS,
	})
}

// TestConnection menangani POST /api/setup/connection/test.
func (h *SetupHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var b saveConnectionBody
	if !decodeJSON(w, r, &b) {
		return
	}
	res, err := h.setup.TestConnection(usecase.SaveConnectionInput{
		Host: b.Host, Port: b.Port, Username: b.Username,
		Password: b.Password, UseTLS: b.UseTLS,
	})
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, res)
}

type saveKrisKrisBody struct {
	APIKey         string `json:"api_key"`
	APISecret      string `json:"api_secret"`
	CallbackSecret string `json:"callback_secret"`
}

// SaveKrisKris menangani POST /api/setup/kriskris.
func (h *SetupHandler) SaveKrisKris(w http.ResponseWriter, r *http.Request) {
	var b saveKrisKrisBody
	if !decodeJSON(w, r, &b) {
		return
	}
	if err := h.setup.SaveKrisKris(usecase.SaveKrisKrisInput{
		APIKey: b.APIKey, APISecret: b.APISecret, CallbackSecret: b.CallbackSecret,
	}); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"saved": true})
}

// Provision menangani POST /api/setup/provision.
func (h *SetupHandler) Provision(w http.ResponseWriter, r *http.Request) {
	steps, err := h.setup.Provision()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]any{"steps": steps})
}

// Finish menangani POST /api/setup/finish.
func (h *SetupHandler) Finish(w http.ResponseWriter, r *http.Request) {
	if err := h.setup.Finish(); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"setup_completed": true})
}
