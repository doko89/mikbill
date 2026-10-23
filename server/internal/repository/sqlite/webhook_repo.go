package sqlite

import (
	"database/sql"

	"github.com/mikbil/mikbil/internal/domain"
)

// WebhookRepo implementasi dedup callback KrisKris di SQLite.
type WebhookRepo struct{ db *sql.DB }

// NewWebhookRepo membuat WebhookRepo.
func NewWebhookRepo(db *sql.DB) *WebhookRepo { return &WebhookRepo{db: db} }

// Exists melaporkan apakah delivery_id sudah pernah dicatat.
func (r *WebhookRepo) Exists(deliveryID string) (bool, error) {
	var one int
	err := r.db.QueryRow(
		`SELECT 1 FROM webhook_deliveries WHERE delivery_id = ?`, deliveryID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Record mencatat delivery baru; duplikat diabaikan (idempotent).
func (r *WebhookRepo) Record(d domain.WebhookDelivery) error {
	_, err := r.db.Exec(
		`INSERT INTO webhook_deliveries (delivery_id, event, invoice_id)
		 VALUES (?, ?, ?) ON CONFLICT(delivery_id) DO NOTHING`,
		d.DeliveryID, d.Event, d.InvoiceID,
	)
	return err
}
