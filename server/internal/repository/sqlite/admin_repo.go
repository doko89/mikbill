package sqlite

import (
	"database/sql"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// AdminRepo implementasi repository admin di SQLite.
type AdminRepo struct{ db *sql.DB }

// NewAdminRepo membuat AdminRepo.
func NewAdminRepo(db *sql.DB) *AdminRepo { return &AdminRepo{db: db} }

// Create menyimpan admin baru.
func (r *AdminRepo) Create(a domain.Admin) (domain.Admin, error) {
	res, err := r.db.Exec(
		`INSERT INTO admins (name, username, password_hash) VALUES (?, ?, ?)`,
		a.Name, a.Username, a.PasswordHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return domain.Admin{}, domain.ErrAlreadyExists
		}
		return domain.Admin{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Admin{}, err
	}
	a.ID = id
	return r.GetByID(id)
}

// GetByID mengambil admin berdasarkan id.
func (r *AdminRepo) GetByID(id int64) (domain.Admin, error) {
	var a domain.Admin
	var created string
	err := r.db.QueryRow(
		`SELECT id, name, username, password_hash, created_at FROM admins WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Username, &a.PasswordHash, &created)
	if err == sql.ErrNoRows {
		return domain.Admin{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Admin{}, err
	}
	a.CreatedAt = parseTime(created)
	return a, nil
}

// GetByUsername mengambil admin berdasarkan username.
func (r *AdminRepo) GetByUsername(username string) (domain.Admin, error) {
	var a domain.Admin
	var created string
	err := r.db.QueryRow(
		`SELECT id, name, username, password_hash, created_at FROM admins WHERE username = ?`, username,
	).Scan(&a.ID, &a.Name, &a.Username, &a.PasswordHash, &created)
	if err == sql.ErrNoRows {
		return domain.Admin{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Admin{}, err
	}
	a.CreatedAt = parseTime(created)
	return a, nil
}

// Count menghitung jumlah admin.
func (r *AdminRepo) Count() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
