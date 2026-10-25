package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// ThemeRepo implementasi aset biner tema di SQLite.
type ThemeRepo struct{ db *sql.DB }

// NewThemeRepo membuat ThemeRepo.
func NewThemeRepo(db *sql.DB) *ThemeRepo { return &ThemeRepo{db: db} }

// Get mengambil aset; found=false bila kunci belum ada.
func (r *ThemeRepo) Get(key string) (domain.ThemeAsset, bool, error) {
	var a domain.ThemeAsset
	a.Key = key
	err := r.db.QueryRow(`SELECT mime, data FROM theme_assets WHERE key = ?`, key).Scan(&a.MIME, &a.Data)
	if err == sql.ErrNoRows {
		return domain.ThemeAsset{}, false, nil
	}
	if err != nil {
		return domain.ThemeAsset{}, false, err
	}
	return a, true, nil
}

// Set menyimpan aset (upsert).
func (r *ThemeRepo) Set(key, mime string, data []byte) error {
	_, err := r.db.Exec(
		`INSERT INTO theme_assets (key, mime, data, updated_at)
		 VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		 ON CONFLICT(key) DO UPDATE SET mime = excluded.mime,
		 data = excluded.data, updated_at = excluded.updated_at`,
		key, mime, data,
	)
	return err
}

// Delete menghapus aset; kunci yang hilang bukan error.
func (r *ThemeRepo) Delete(key string) error {
	_, err := r.db.Exec(`DELETE FROM theme_assets WHERE key = ?`, key)
	return err
}
