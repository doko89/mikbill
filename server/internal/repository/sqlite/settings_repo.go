package sqlite

import "database/sql"

// SettingsRepo implementasi key-value settings di SQLite.
type SettingsRepo struct{ db *sql.DB }

// NewSettingsRepo membuat SettingsRepo.
func NewSettingsRepo(db *sql.DB) *SettingsRepo { return &SettingsRepo{db: db} }

// Get mengambil nilai; found=false bila kunci belum ada.
func (r *SettingsRepo) Get(key string) (val string, found bool, err error) {
	err = r.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Set menyimpan nilai (upsert).
func (r *SettingsRepo) Set(key, val string) error {
	_, err := r.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, val,
	)
	return err
}
