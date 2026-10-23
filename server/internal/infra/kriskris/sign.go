// Package kriskris adalah client API invoice KrisKris.id (PRD bagian 7).
package kriskris

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SignRequest menandatangani request keluar: lima baris join newline,
// HMAC-SHA256 dengan API secret, hasil "v1=<hex>".
func SignRequest(apiSecret, timestamp, method, path, rawQuery string, body []byte) string {
	sum := sha256.Sum256(body)
	toSign := strings.Join([]string{
		timestamp,
		strings.ToUpper(method),
		path,
		rawQuery,
		hex.EncodeToString(sum[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(toSign))
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}
