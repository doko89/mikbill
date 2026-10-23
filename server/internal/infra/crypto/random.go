package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"math/big"
)

// VoucherAlphabet: tanpa karakter ambigu 0/O/1/I/l.
const VoucherAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// RandomString membuat string acak sepanjang n dari alfabet voucher.
func RandomString(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(VoucherAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = VoucherAlphabet[v.Int64()]
	}
	return string(out), nil
}

// RandomBytes membuat n byte acak kriptografis.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// RandomTokenURL membuat token base64url dari n byte acak (untuk cookie sesi).
func RandomTokenURL(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RandomTokenHex membuat token hex dari n byte acak (untuk token CSRF).
func RandomTokenHex(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
