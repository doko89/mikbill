package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/usecase"
)

// secureCookieFor menentukan flag Secure dari skema request aktual: TLS
// langsung atau X-Forwarded-Proto=https di belakang reverse proxy. Skema
// konfigurasi (site_url) tidak dipakai: cookie Secure yang diterima lewat
// HTTP dibuang browser sehingga semua halaman admin 401.
func secureCookieFor(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

// AdminHandler menangani auth admin (tech_spec 4.4).
type AdminHandler struct {
	auth *usecase.AuthUsecase
}

// NewAdminHandler membuat AdminHandler.
func NewAdminHandler(auth *usecase.AuthUsecase) *AdminHandler {
	return &AdminHandler{auth: auth}
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login menangani POST /api/admin/login.
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var b loginBody
	if !decodeJSON(w, r, &b) {
		return
	}
	token, admin, sess, err := h.auth.Login(b.Username, b.Password)
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
		MaxAge:   int(time.Until(sess.ExpiresAt).Seconds()),
	})
	WriteOK(w, http.StatusOK, map[string]any{
		"admin": map[string]any{
			"id": admin.ID, "name": admin.Name, "username": admin.Username,
		},
		"csrf_token": sess.CSRFToken,
	})
}

// Logout menangani POST /api/admin/logout.
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		_ = h.auth.Logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	WriteOK(w, http.StatusOK, map[string]bool{"logout": true})
}

// Me menangani GET /api/admin/me.
func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	admin := AdminFromContext(r.Context())
	sess := SessionFromContext(r.Context())
	WriteOK(w, http.StatusOK, map[string]any{
		"admin": map[string]any{
			"id": admin.ID, "name": admin.Name, "username": admin.Username,
		},
		"csrf_token": sess.CSRFToken,
	})
}
