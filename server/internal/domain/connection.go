package domain

import "time"

// Connection adalah kredensial koneksi RouterOS API.
type Connection struct {
	ID          int64
	Name        string
	Host        string
	Port        int
	Username    string
	EncPassword string // AES-GCM, format base64(nonce|ciphertext)
	UseTLS      bool
	IsActive    bool
	LastTestAt  *time.Time
	LastTestOK  *bool
	LastTestMsg string
}
