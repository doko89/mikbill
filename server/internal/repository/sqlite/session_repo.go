package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// SessionRepo implementasi repository sesi di SQLite.
type SessionRepo struct{ db *sql.DB }

// NewSessionRepo membuat SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

// Create menyimpan sesi baru.
func (r *SessionRepo) Create(s domain.Session) error {
	_, err := r.db.Exec(
		`INSERT INTO sessions (id, admin_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`,
		s.ID, s.AdminID, s.CSRFToken, formatTime(s.ExpiresAt),
	)
	return err
}

// Get mengambil sesi berdasarkan id (hash token).
func (r *SessionRepo) Get(id string) (domain.Session, error) {
	var s domain.Session
	var expires, created string
	err := r.db.QueryRow(
		`SELECT id, admin_id, csrf_token, expires_at, created_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.AdminID, &s.CSRFToken, &expires, &created)
	if err == sql.ErrNoRows {
		return domain.Session{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	s.ExpiresAt = parseTime(expires)
	s.CreatedAt = parseTime(created)
	return s, nil
}

// Delete menghapus sesi.
func (r *SessionRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteExpired menghapus semua sesi kedaluwarsa.
func (r *SessionRepo) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%fZ','now')`)
	return err
}
