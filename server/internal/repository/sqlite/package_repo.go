package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// PackageRepo implementasi repository paket di SQLite.
type PackageRepo struct{ db *sql.DB }

// NewPackageRepo membuat PackageRepo.
func NewPackageRepo(db *sql.DB) *PackageRepo { return &PackageRepo{db: db} }

const packageColumns = `id, name, description, price_idr, validity_label,
	profile_name, rate_limit, session_timeout, shared_users, is_active, sort_order,
	created_at, updated_at`

func scanPackage(row interface {
	Scan(dest ...any) error
}, p *domain.Package) error {
	var isActive int
	var created, updated string
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.PriceIDR,
		&p.ValidityLabel, &p.ProfileName, &p.RateLimit, &p.SessionTimeout,
		&p.SharedUsers, &isActive, &p.SortOrder, &created, &updated)
	if err != nil {
		return err
	}
	p.IsActive = isActive == 1
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return nil
}

// List mengembalikan semua paket urut sort_order lalu id.
func (r *PackageRepo) List() ([]domain.Package, error) {
	rows, err := r.db.Query(`SELECT ` + packageColumns + ` FROM packages ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Package{}
	for rows.Next() {
		var p domain.Package
		if err := scanPackage(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListActive mengembalikan paket aktif urut sort_order lalu id.
func (r *PackageRepo) ListActive() ([]domain.Package, error) {
	rows, err := r.db.Query(`SELECT ` + packageColumns +
		` FROM packages WHERE is_active = 1 ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Package{}
	for rows.Next() {
		var p domain.Package
		if err := scanPackage(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByID mengambil satu paket.
func (r *PackageRepo) GetByID(id int64) (domain.Package, error) {
	var p domain.Package
	err := scanPackage(r.db.QueryRow(
		`SELECT `+packageColumns+` FROM packages WHERE id = ?`, id), &p)
	if err == sql.ErrNoRows {
		return domain.Package{}, domain.ErrNotFound
	}
	return p, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Create menyimpan paket baru.
func (r *PackageRepo) Create(p domain.Package) (domain.Package, error) {
	res, err := r.db.Exec(
		`INSERT INTO packages (name, description, price_idr, validity_label,
		  profile_name, rate_limit, session_timeout, shared_users, is_active, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.PriceIDR, p.ValidityLabel,
		p.ProfileName, p.RateLimit, p.SessionTimeout, p.SharedUsers,
		boolInt(p.IsActive), p.SortOrder,
	)
	if err != nil {
		return domain.Package{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Package{}, err
	}
	return r.GetByID(id)
}

// Update mengubah paket.
func (r *PackageRepo) Update(p domain.Package) (domain.Package, error) {
	res, err := r.db.Exec(
		`UPDATE packages SET name = ?, description = ?, price_idr = ?,
		  validity_label = ?, profile_name = ?, rate_limit = ?,
		  session_timeout = ?, shared_users = ?, is_active = ?, sort_order = ?,
		  updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		 WHERE id = ?`,
		p.Name, p.Description, p.PriceIDR, p.ValidityLabel,
		p.ProfileName, p.RateLimit, p.SessionTimeout, p.SharedUsers,
		boolInt(p.IsActive), p.SortOrder, p.ID,
	)
	if err != nil {
		return domain.Package{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Package{}, err
	}
	if n == 0 {
		return domain.Package{}, domain.ErrNotFound
	}
	return r.GetByID(p.ID)
}

// Delete menghapus paket.
func (r *PackageRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM packages WHERE id = ?`, id)
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
