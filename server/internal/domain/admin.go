// Package domain berisi entity dan error domain Mikbil.
// Tanpa dependensi ke layer lain.
package domain

import "time"

// Admin adalah akun pemilik (single role owner di v1).
type Admin struct {
	ID           int64
	Name         string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}
