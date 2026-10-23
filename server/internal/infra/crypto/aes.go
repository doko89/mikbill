// Package crypto berisi primitif kriptografi: AES-GCM, bcrypt, acak.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// Protector mengenkripsi nilai rahasia dengan AES-256-GCM.
type Protector struct {
	aead cipher.AEAD
}

// NewProtector membuat Protector dari kunci hex 64 char (32 byte).
func NewProtector(keyHex string) (*Protector, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return nil, errors.New("kunci enkripsi harus 32 byte (hex 64 char)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Protector{aead: aead}, nil
}

// Seal mengenkripsi plaintext, hasil base64(nonce|ciphertext).
func (p *Protector) Seal(plaintext string) (string, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := p.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Open mendekripsi hasil Seal.
func (p *Protector) Open(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.New("nilai terenkripsi rusak")
	}
	if len(raw) < p.aead.NonceSize() {
		return "", errors.New("nilai terenkripsi rusak")
	}
	nonce, data := raw[:p.aead.NonceSize()], raw[p.aead.NonceSize():]
	plain, err := p.aead.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("gagal mendekripsi: %w", err)
	}
	return string(plain), nil
}
