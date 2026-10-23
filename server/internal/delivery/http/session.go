package http

import (
	"context"
	"net/http"

	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/usecase"
)

// SessionCookie adalah nama cookie sesi admin.
const SessionCookie = "mikbil_session"

// CSRFHeader adalah header token CSRF untuk request mutasi admin.
const CSRFHeader = "X-CSRF-Token"

type ctxKey string

const (
	ctxAdmin   ctxKey = "admin"
	ctxSession ctxKey = "session"
)

// AdminFromContext mengambil admin terautentikasi dari context.
func AdminFromContext(ctx context.Context) domain.Admin {
	a, _ := ctx.Value(ctxAdmin).(domain.Admin)
	return a
}

// SessionFromContext mengambil sesi dari context.
func SessionFromContext(ctx context.Context) domain.Session {
	s, _ := ctx.Value(ctxSession).(domain.Session)
	return s
}

// RequireAuth memastikan request membawa sesi valid; untuk metode mutasi
// juga memastikan header CSRF cocok dengan token sesi.
func RequireAuth(auth *usecase.AuthUsecase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookie)
			if err != nil || cookie.Value == "" {
				WriteErr(w, http.StatusUnauthorized, "belum login")
				return
			}
			admin, sess, err := auth.Authenticate(cookie.Value)
			if err != nil {
				writeUsecaseErr(w, err)
				return
			}
			if isUnsafeMethod(r.Method) && !validCSRF(r.Header.Get(CSRFHeader), sess.CSRFToken) {
				WriteErr(w, http.StatusForbidden, "token CSRF tidak valid")
				return
			}
			ctx := context.WithValue(r.Context(), ctxAdmin, admin)
			ctx = context.WithValue(ctx, ctxSession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
