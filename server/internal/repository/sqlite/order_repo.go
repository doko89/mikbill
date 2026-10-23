package sqlite

import (
	"database/sql"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// OrderRepo implementasi repository order di SQLite.
type OrderRepo struct{ db *sql.DB }

// NewOrderRepo membuat OrderRepo.
func NewOrderRepo(db *sql.DB) *OrderRepo { return &OrderRepo{db: db} }

const orderColumns = `id, package_id, base_amount, unique_code, total_amount,
	kriskris_invoice_id, status, qr_payload, expires_at, paid_at, failed_reason,
	created_at, updated_at`

func scanOrder(row interface{ Scan(dest ...any) error }, o *domain.Order) error {
	var inv, qr, exp, paid, failed, created, updated sql.NullString
	var base, unique, total int
	var pkg int64
	var status string
	var id string
	err := row.Scan(&id, &pkg, &base, &unique, &total,
		&inv, &status, &qr, &exp, &paid, &failed, &created, &updated)
	if err != nil {
		return err
	}
	expires := parseTimeNull(exp)
	paidAt := parseTimeNull(paid)
	*o = domain.Order{
		ID: id, PackageID: pkg, BaseAmount: base, UniqueCode: unique,
		TotalAmount: total, KrisKrisInvoice: inv.String, Status: status,
		QRPayload: qr.String, ExpiresAt: expires, PaidAt: paidAt,
		FailedReason: failed.String,
		CreatedAt:    parseTime(created.String), UpdatedAt: parseTime(updated.String),
	}
	return nil
}

func parseTimeNull(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

// Create menyimpan order baru (id dibuat pemanggil via domain.NewOrderID).
func (r *OrderRepo) Create(o domain.Order) error {
	_, err := r.db.Exec(
		`INSERT INTO orders (id, package_id, base_amount, unique_code, total_amount,
		  kriskris_invoice_id, status, qr_payload, expires_at, paid_at, failed_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.PackageID, o.BaseAmount, o.UniqueCode, o.TotalAmount,
		nullStr(o.KrisKrisInvoice), o.Status, o.QRPayload,
		nullTime(o.ExpiresAt), nullTime(o.PaidAt), o.FailedReason,
	)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetByID mengambil satu order.
func (r *OrderRepo) GetByID(id string) (domain.Order, error) {
	var o domain.Order
	err := scanOrder(r.db.QueryRow(
		`SELECT `+orderColumns+` FROM orders WHERE id = ?`, id), &o)
	if err == sql.ErrNoRows {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, err
}

// GetByInvoice mengambil order berdasarkan id invoice KrisKris.
func (r *OrderRepo) GetByInvoice(invoiceID string) (domain.Order, error) {
	var o domain.Order
	err := scanOrder(r.db.QueryRow(
		`SELECT `+orderColumns+` FROM orders WHERE kriskris_invoice_id = ?`, invoiceID), &o)
	if err == sql.ErrNoRows {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, err
}

// pendingByExpiry mengambil order pending untuk satu siklus poller.
func (r *OrderRepo) pendingByExpiry(limit int, now time.Time, stale bool) ([]domain.Order, error) {
	cmp := ">"
	if stale {
		cmp = "<="
	}
	rows, err := r.db.Query(
		`SELECT `+orderColumns+` FROM orders
		 WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at `+cmp+` ?
		 ORDER BY created_at LIMIT ?`, formatTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Order{}
	for rows.Next() {
		var o domain.Order
		if err := scanOrder(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// PendingFresh mengambil order pending yang belum kedaluwarsa (untuk batch status).
func (r *OrderRepo) PendingFresh(limit int, now time.Time) ([]domain.Order, error) {
	return r.pendingByExpiry(limit, now, false)
}

// PendingStale mengambil order pending yang expires_at-nya sudah lewat
// (untuk konfirmasi sekali via GET).
func (r *OrderRepo) PendingStale(limit int, now time.Time) ([]domain.Order, error) {
	return r.pendingByExpiry(limit, now, true)
}

func (r *OrderRepo) setStatus(id, status, extra string, args ...any) error {
	query := `UPDATE orders SET status = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now'))` +
		extra + ` WHERE id = ?`
	_, err := r.db.Exec(query, append([]any{status}, append(args, id)...)...)
	return err
}

// ClaimProcessing mengubah pending menjadi processing secara atomik lewat
// satu UPDATE bersyarat (setara SELECT FOR UPDATE). True hanya untuk satu
// pemenang; false bila status sudah bukan pending.
func (r *OrderRepo) ClaimProcessing(id string) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE orders SET status = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		 WHERE id = ? AND status = ?`, domain.OrderProcess, id, domain.OrderPending)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// CompletePaid menyimpan voucher dan menandai order paid dalam satu transaksi.
func (r *OrderRepo) CompletePaid(orderID string, v domain.Voucher, paidAt time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO vouchers (order_id, username, enc_password, profile_name)
		 VALUES (?, ?, ?, ?)`,
		orderID, v.Username, v.EncPassword, v.ProfileName); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE orders SET status = ?, paid_at = ?,
		  updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now')) WHERE id = ?`,
		domain.OrderPaid, formatTime(paidAt), orderID); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkPaid menandai order lunas (untuk jalur yang vouchernya sudah ada).
func (r *OrderRepo) MarkPaid(id string, paidAt time.Time) error {
	return r.setStatus(id, domain.OrderPaid, `, paid_at = ?`, formatTime(paidAt))
}

// MarkExpired menandai order kedaluwarsa.
func (r *OrderRepo) MarkExpired(id string) error {
	return r.setStatus(id, domain.OrderExpired, "")
}

// MarkCancelled menandai order dibatalkan.
func (r *OrderRepo) MarkCancelled(id string) error {
	return r.setStatus(id, domain.OrderCancelled, "")
}

// MarkFailed menandai order gagal beserta alasan.
func (r *OrderRepo) MarkFailed(id, reason string) error {
	return r.setStatus(id, domain.OrderFailed, `, failed_reason = ?`, reason)
}

// orderFilter menyusun klausa WHERE untuk daftar admin.
func orderFilter(status, q string) (string, []any) {
	where := " WHERE 1 = 1"
	var args []any
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if q != "" {
		where += " AND (id LIKE ? ESCAPE '\\' OR kriskris_invoice_id LIKE ? ESCAPE '\\')"
		like := "%" + escapeLike(q) + "%"
		args = append(args, like, like)
	}
	return where, args
}

func escapeLike(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '%' || c == '_' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// List mengambil order untuk halaman admin, terbaru dulu.
func (r *OrderRepo) List(status, q string, limit, offset int) ([]domain.Order, error) {
	where, args := orderFilter(status, q)
	args = append(args, limit, offset)
	rows, err := r.db.Query(
		`SELECT `+orderColumns+` FROM orders`+where+
			` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Order{}
	for rows.Next() {
		var o domain.Order
		if err := scanOrder(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Count menghitung order untuk paginasi daftar admin.
func (r *OrderRepo) Count(status, q string) (int, error) {
	where, args := orderFilter(status, q)
	var n int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM orders`+where, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountByStatus menghitung order berstatus tertentu.
func (r *OrderRepo) CountByStatus(status string) (int, error) {
	var n int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM orders WHERE status = ?`, status).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// RevenueSince menjumlah total_amount order paid sejak waktu tertentu.
func (r *OrderRepo) RevenueSince(since time.Time) (int, error) {
	var total int
	if err := r.db.QueryRow(
		`SELECT COALESCE(SUM(total_amount), 0) FROM orders
		 WHERE status = 'paid' AND paid_at IS NOT NULL AND paid_at >= ?`,
		formatTime(since)).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// Recent mengambil order terbaru untuk ringkasan admin.
func (r *OrderRepo) Recent(limit int) ([]domain.Order, error) {
	return r.List("", "", limit, 0)
}

// RequeueFailed mengembalikan order failed menjadi pending agar bisa retry.
// True hanya bila status sebelumnya failed.
func (r *OrderRepo) RequeueFailed(id string) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE orders SET status = ?, failed_reason = '',
		  updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		 WHERE id = ? AND status = ?`, domain.OrderPending, id, domain.OrderFailed)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
