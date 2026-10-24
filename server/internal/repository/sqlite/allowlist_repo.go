package sqlite

import (
	"database/sql"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// AllowlistRepo implementasi repository allowlist walled garden di SQLite.
type AllowlistRepo struct{ db *sql.DB }

// NewAllowlistRepo membuat AllowlistRepo.
func NewAllowlistRepo(db *sql.DB) *AllowlistRepo { return &AllowlistRepo{db: db} }

func scanAllowlist(row interface {
	Scan(dest ...any) error
}, a *domain.WalledGardenAllow) error {
	var enabled int
	var created string
	if err := row.Scan(&a.ID, &a.DstHost, &enabled, &created); err != nil {
		return err
	}
	a.Enabled = enabled == 1
	return nil
}

func allowlistUnique(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return domain.ErrAlreadyExists
	}
	return err
}

// List mengembalikan semua host allowlist urut nama domain.
func (r *AllowlistRepo) List() ([]domain.WalledGardenAllow, error) {
	rows, err := r.db.Query(`SELECT id, dst_host, enabled, created_at
		FROM walled_garden_allowlist ORDER BY dst_host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.WalledGardenAllow{}
	for rows.Next() {
		var a domain.WalledGardenAllow
		if err := scanAllowlist(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetByID mengambil satu host allowlist.
func (r *AllowlistRepo) GetByID(id int64) (domain.WalledGardenAllow, error) {
	var a domain.WalledGardenAllow
	err := scanAllowlist(r.db.QueryRow(
		`SELECT id, dst_host, enabled, created_at FROM walled_garden_allowlist WHERE id = ?`, id), &a)
	if err == sql.ErrNoRows {
		return domain.WalledGardenAllow{}, domain.ErrNotFound
	}
	return a, err
}

// Create menyimpan host allowlist baru.
func (r *AllowlistRepo) Create(dstHost string, enabled bool) (domain.WalledGardenAllow, error) {
	res, err := r.db.Exec(
		`INSERT INTO walled_garden_allowlist (dst_host, enabled) VALUES (?, ?)`,
		dstHost, boolInt(enabled),
	)
	if err != nil {
		return domain.WalledGardenAllow{}, allowlistUnique(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.WalledGardenAllow{}, err
	}
	return r.GetByID(id)
}

// Update mengubah host allowlist.
func (r *AllowlistRepo) Update(a domain.WalledGardenAllow) (domain.WalledGardenAllow, error) {
	res, err := r.db.Exec(
		`UPDATE walled_garden_allowlist SET dst_host = ?, enabled = ? WHERE id = ?`,
		a.DstHost, boolInt(a.Enabled), a.ID,
	)
	if err != nil {
		return domain.WalledGardenAllow{}, allowlistUnique(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.WalledGardenAllow{}, err
	}
	if n == 0 {
		return domain.WalledGardenAllow{}, domain.ErrNotFound
	}
	return r.GetByID(a.ID)
}

// Delete menghapus host allowlist.
func (r *AllowlistRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM walled_garden_allowlist WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
