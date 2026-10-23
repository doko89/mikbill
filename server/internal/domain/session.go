package domain

import "time"

// Session adalah sesi login admin (disimpan di DB agar bisa revoke).
type Session struct {
	ID        string // hex sha256 dari token mentah
	AdminID   int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Expired melaporkan apakah sesi sudah kedaluwarsa.
func (s Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}
