package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// VoucherRepo implementasi repository voucher di SQLite.
type VoucherRepo struct{ db *sql.DB }

// NewVoucherRepo membuat VoucherRepo.
func NewVoucherRepo(db *sql.DB) *VoucherRepo { return &VoucherRepo{db: db} }

// GetByOrderID mengambil voucher berdasarkan order.
func (r *VoucherRepo) GetByOrderID(orderID string) (domain.Voucher, error) {
	var v domain.Voucher
	var created string
	err := r.db.QueryRow(
		`SELECT id, order_id, username, enc_password, profile_name, created_at
		 FROM vouchers WHERE order_id = ?`, orderID,
	).Scan(&v.ID, &v.OrderID, &v.Username, &v.EncPassword, &v.ProfileName, &created)
	if err == sql.ErrNoRows {
		return domain.Voucher{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Voucher{}, err
	}
	v.CreatedAt = parseTime(created)
	return v, nil
}

// CountByOrder menghitung voucher untuk satu order (untuk uji idempotency).
func (r *VoucherRepo) CountByOrder(orderID string) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM vouchers WHERE order_id = ?`, orderID).Scan(&n)
	return n, err
}
