package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// ConnectionRepo implementasi repository koneksi MikroTik di SQLite.
type ConnectionRepo struct{ db *sql.DB }

// NewConnectionRepo membuat ConnectionRepo.
func NewConnectionRepo(db *sql.DB) *ConnectionRepo { return &ConnectionRepo{db: db} }

// CreateActive menyimpan koneksi baru sebagai satu-satunya koneksi aktif.
func (r *ConnectionRepo) CreateActive(c domain.Connection) (domain.Connection, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Connection{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE mikrotik_connections SET is_active = 0`); err != nil {
		return domain.Connection{}, err
	}
	useTLS := 0
	if c.UseTLS {
		useTLS = 1
	}
	res, err := tx.Exec(
		`INSERT INTO mikrotik_connections (name, host, port, username, enc_password, use_tls, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, 1)`,
		c.Name, c.Host, c.Port, c.Username, c.EncPassword, useTLS,
	)
	if err != nil {
		return domain.Connection{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Connection{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Connection{}, err
	}
	c.ID = id
	c.IsActive = true
	return c, nil
}

// GetActive mengambil koneksi aktif.
func (r *ConnectionRepo) GetActive() (domain.Connection, error) {
	var c domain.Connection
	var useTLS, isActive int
	var lastTestAt sql.NullString
	var lastTestOK sql.NullBool
	var lastTestMsg sql.NullString
	err := r.db.QueryRow(
		`SELECT id, name, host, port, username, enc_password, use_tls, is_active,
		        last_test_at, last_test_ok, last_test_msg
		 FROM mikrotik_connections WHERE is_active = 1 ORDER BY id DESC LIMIT 1`,
	).Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.EncPassword,
		&useTLS, &isActive, &lastTestAt, &lastTestOK, &lastTestMsg)
	if err == sql.ErrNoRows {
		return domain.Connection{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Connection{}, err
	}
	applyTestState(&c, useTLS, isActive, lastTestAt, lastTestOK, lastTestMsg)
	return c, nil
}

func applyTestState(c *domain.Connection, useTLS, isActive int, lastTestAt sql.NullString, lastTestOK sql.NullBool, lastTestMsg sql.NullString) {
	c.UseTLS = useTLS == 1
	c.IsActive = isActive == 1
	if lastTestAt.Valid {
		t := parseTime(lastTestAt.String)
		c.LastTestAt = &t
	}
	if lastTestOK.Valid {
		c.LastTestOK = &lastTestOK.Bool
	}
	if lastTestMsg.Valid {
		c.LastTestMsg = lastTestMsg.String
	}
}

func scanConnection(row interface {
	Scan(dest ...any) error
}, c *domain.Connection) error {
	var useTLS, isActive int
	var lastTestAt sql.NullString
	var lastTestOK sql.NullBool
	var lastTestMsg sql.NullString
	err := row.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.EncPassword,
		&useTLS, &isActive, &lastTestAt, &lastTestOK, &lastTestMsg)
	if err != nil {
		return err
	}
	applyTestState(c, useTLS, isActive, lastTestAt, lastTestOK, lastTestMsg)
	return nil
}

const connectionColumns = `id, name, host, port, username, enc_password, use_tls, is_active,
	last_test_at, last_test_ok, last_test_msg`

// List mengembalikan semua koneksi urut id.
func (r *ConnectionRepo) List() ([]domain.Connection, error) {
	rows, err := r.db.Query(`SELECT ` + connectionColumns + ` FROM mikrotik_connections ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Connection{}
	for rows.Next() {
		var c domain.Connection
		if err := scanConnection(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetByID mengambil satu koneksi.
func (r *ConnectionRepo) GetByID(id int64) (domain.Connection, error) {
	var c domain.Connection
	err := scanConnection(r.db.QueryRow(
		`SELECT `+connectionColumns+` FROM mikrotik_connections WHERE id = ?`, id), &c)
	if err == sql.ErrNoRows {
		return domain.Connection{}, domain.ErrNotFound
	}
	return c, err
}

// Create menyimpan koneksi baru; bila IsActive, koneksi lain dinonaktifkan.
// Koneksi pertama selalu menjadi aktif.
func (r *ConnectionRepo) Create(c domain.Connection) (domain.Connection, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Connection{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM mikrotik_connections`).Scan(&n); err != nil {
		return domain.Connection{}, err
	}
	active := 0
	if c.IsActive || n == 0 {
		active = 1
	}
	if active == 1 {
		if _, err := tx.Exec(`UPDATE mikrotik_connections SET is_active = 0`); err != nil {
			return domain.Connection{}, err
		}
	}
	useTLS := 0
	if c.UseTLS {
		useTLS = 1
	}
	res, err := tx.Exec(
		`INSERT INTO mikrotik_connections (name, host, port, username, enc_password, use_tls, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Host, c.Port, c.Username, c.EncPassword, useTLS, active,
	)
	if err != nil {
		return domain.Connection{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Connection{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Connection{}, err
	}
	c.ID = id
	c.IsActive = active == 1
	return c, nil
}

// Update mengubah koneksi; bila IsActive, koneksi lain dinonaktifkan.
func (r *ConnectionRepo) Update(c domain.Connection) (domain.Connection, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Connection{}, err
	}
	defer tx.Rollback()
	if c.IsActive {
		if _, err := tx.Exec(`UPDATE mikrotik_connections SET is_active = 0 WHERE id <> ?`, c.ID); err != nil {
			return domain.Connection{}, err
		}
	}
	active := 0
	if c.IsActive {
		active = 1
	}
	useTLS := 0
	if c.UseTLS {
		useTLS = 1
	}
	res, err := tx.Exec(
		`UPDATE mikrotik_connections
		 SET name = ?, host = ?, port = ?, username = ?, enc_password = ?, use_tls = ?, is_active = ?
		 WHERE id = ?`,
		c.Name, c.Host, c.Port, c.Username, c.EncPassword, useTLS, active, c.ID,
	)
	if err != nil {
		return domain.Connection{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Connection{}, err
	}
	if n == 0 {
		return domain.Connection{}, domain.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return domain.Connection{}, err
	}
	// Pertahankan status tes terakhir dari baris yang ada.
	kept, err := r.GetByID(c.ID)
	if err != nil {
		return domain.Connection{}, err
	}
	return kept, nil
}

// Delete menghapus koneksi; bila yang aktif dihapus, sisa terbaru diaktifkan.
func (r *ConnectionRepo) Delete(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var wasActive int
	err = tx.QueryRow(`SELECT is_active FROM mikrotik_connections WHERE id = ?`, id).Scan(&wasActive)
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mikrotik_connections WHERE id = ?`, id); err != nil {
		return err
	}
	if wasActive == 1 {
		_, _ = tx.Exec(`UPDATE mikrotik_connections SET is_active = 1 WHERE id = (
			SELECT id FROM mikrotik_connections ORDER BY id DESC LIMIT 1)`)
	}
	return tx.Commit()
}

// SaveTestResult mencatat hasil uji koneksi.
func (r *ConnectionRepo) SaveTestResult(id int64, ok bool, msg string) error {
	v := 0
	if ok {
		v = 1
	}
	_, err := r.db.Exec(
		`UPDATE mikrotik_connections SET last_test_at = ?, last_test_ok = ?, last_test_msg = ? WHERE id = ?`,
		formatTimeNow(), v, msg, id,
	)
	return err
}
