package sqlite

import "database/sql"

// AuditRepo implementasi log audit di SQLite.
type AuditRepo struct{ db *sql.DB }

// NewAuditRepo membuat AuditRepo.
func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }

// Log mencatat satu baris audit.
func (r *AuditRepo) Log(actor, action, entity, entityID, meta string) error {
	_, err := r.db.Exec(
		`INSERT INTO audit_logs (actor, action, entity, entity_id, meta_json)
		 VALUES (?, ?, ?, ?, ?)`,
		actor, action, entity, entityID, meta,
	)
	return err
}
