package http

import (
	"net/http"
	"time"

	"github.com/mikbil/mikbil/internal/usecase"
)

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
		Secure:   h.auth.SecureCookie(),
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
		Secure:   h.auth.SecureCookie(),
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
