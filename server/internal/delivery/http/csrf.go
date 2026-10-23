package http

import (
	"crypto/subtle"
	"net/http"
)

// isUnsafeMethod melaporkan apakah metode HTTP mengubah state.
func isUnsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// validCSRF membandingkan token CSRF secara constant-time.
func validCSRF(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
