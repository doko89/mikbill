package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/usecase"
)

// RouterHandler menangani API koneksi, profile, provisioning, template (tech_spec 4.4).
type RouterHandler struct {
	conns    *usecase.ConnectionUsecase
	profiles *usecase.ProfileUsecase
	prov     *usecase.ProvisioningUsecase
	hotspot  *usecase.HotspotUsecase
}

// NewRouterHandler membuat RouterHandler.
func NewRouterHandler(conns *usecase.ConnectionUsecase, profiles *usecase.ProfileUsecase, prov *usecase.ProvisioningUsecase, hotspot *usecase.HotspotUsecase) *RouterHandler {
	return &RouterHandler{conns: conns, profiles: profiles, prov: prov, hotspot: hotspot}
}

func connectionDTO(c domain.Connection) map[string]any {
	var lastTestAt any
	if c.LastTestAt != nil {
		lastTestAt = c.LastTestAt.Format("2006-01-02T15:04:05.000Z")
	}
	var lastTestOK any
	if c.LastTestOK != nil {
		lastTestOK = *c.LastTestOK
	}
	return map[string]any{
		"id": c.ID, "name": c.Name, "host": c.Host, "port": c.Port,
		"username": c.Username, "use_tls": c.UseTLS, "is_active": c.IsActive,
		"last_test_at": lastTestAt, "last_test_ok": lastTestOK, "last_test_msg": c.LastTestMsg,
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		WriteErr(w, http.StatusBadRequest, "id tidak valid")
		return 0, false
	}
	return id, true
}

// ListConnections menangani GET /api/admin/connections.
func (h *RouterHandler) ListConnections(w http.ResponseWriter, _ *http.Request) {
	cs, err := h.conns.List()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, connectionDTO(c))
	}
	WriteOK(w, http.StatusOK, out)
}

type connectionBody struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"use_tls"`
	IsActive *bool  `json:"is_active"`
}

func (b connectionBody) input() usecase.ConnectionInput {
	active := true
	if b.IsActive != nil {
		active = *b.IsActive
	}
	return usecase.ConnectionInput{
		Name: b.Name, Host: b.Host, Port: b.Port, Username: b.Username,
		Password: b.Password, UseTLS: b.UseTLS, IsActive: active,
	}
}

// CreateConnection menangani POST /api/admin/connections.
func (h *RouterHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var b connectionBody
	if !decodeJSON(w, r, &b) {
		return
	}
	c, err := h.conns.Create(b.input())
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, connectionDTO(c))
}

// UpdateConnection menangani PUT /api/admin/connections/:id.
func (h *RouterHandler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b connectionBody
	if !decodeJSON(w, r, &b) {
		return
	}
	c, err := h.conns.Update(id, b.input())
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, connectionDTO(c))
}

// DeleteConnection menangani DELETE /api/admin/connections/:id.
func (h *RouterHandler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.conns.Delete(id); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"deleted": true})
}

// TestConnection menangani POST /api/admin/connections/:id/test.
func (h *RouterHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	res, err := h.conns.Test(id)
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, res)
}

// ListProfiles menangani GET /api/admin/router/profiles (live dari router).
func (h *RouterHandler) ListProfiles(w http.ResponseWriter, _ *http.Request) {
	ps, err := h.profiles.List()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, map[string]any{
			"name": p.Name, "rate_limit": p.RateLimit,
			"session_timeout": p.SessionTimeout, "shared_users": p.SharedUsers,
		})
	}
	WriteOK(w, http.StatusOK, out)
}

type createProfileBody struct {
	Name           string `json:"name"`
	RateLimit      string `json:"rate_limit"`
	SessionTimeout int    `json:"session_timeout"`
	SharedUsers    int    `json:"shared_users"`
}

// CreateProfile menangani POST /api/admin/router/profiles.
func (h *RouterHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var b createProfileBody
	if !decodeJSON(w, r, &b) {
		return
	}
	p, err := h.profiles.Create(usecase.CreateProfileInput{
		Name: b.Name, RateLimit: b.RateLimit,
		SessionTimeout: b.SessionTimeout, SharedUsers: b.SharedUsers,
	})
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, map[string]any{
		"name": p.Name, "rate_limit": p.RateLimit,
		"session_timeout": p.SessionTimeout, "shared_users": p.SharedUsers,
	})
}

// ProvisioningStatus menangani GET /api/admin/provisioning/status.
func (h *RouterHandler) ProvisioningStatus(w http.ResponseWriter, _ *http.Request) {
	st, err := h.prov.Status()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, st)
}

// ProvisioningApply menangani POST /api/admin/provisioning/apply.
func (h *RouterHandler) ProvisioningApply(w http.ResponseWriter, _ *http.Request) {
	steps, err := h.prov.Apply()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]any{"steps": steps})
}

// LoginTemplate menangani GET /api/admin/hotspot/login-template (unduh file).
func (h *RouterHandler) LoginTemplate(w http.ResponseWriter, _ *http.Request) {
	tpl, err := h.hotspot.LoginTemplate()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="login.html"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(tpl))
}

// ButtonSnippet menangani GET /api/admin/hotspot/button-snippet.
func (h *RouterHandler) ButtonSnippet(w http.ResponseWriter, _ *http.Request) {
	snippet, err := h.hotspot.ButtonSnippet()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]string{"snippet": snippet})
}
