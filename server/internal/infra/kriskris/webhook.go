package kriskris

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// VerifyWebhook memverifikasi signature callback "sha256=<hex>"
// dengan HMAC-SHA256 body mentah memakai callback secret.
// Perbandingan constant-time; salah format berarti tidak valid.
func VerifyWebhook(callbackSecret string, body []byte, signature string) bool {
	hexPart, ok := strings.CutPrefix(signature, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexPart)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(callbackSecret))
	mac.Write(body)
	want := mac.Sum(nil)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
