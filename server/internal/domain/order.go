package domain

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
	"time"
)

// Status order sesuai tech_spec bagian 3.
const (
	OrderPending   = "pending"
	OrderProcess   = "processing"
	OrderPaid      = "paid"
	OrderExpired   = "expired"
	OrderCancelled = "cancelled"
	OrderFailed    = "failed"
)

// NewOrderID membuat id order "ord_<12 char base32>" dari crypto/rand.
func NewOrderID() (string, error) {
	b := make([]byte, 7) // 56 bit -> 12 char base32
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return "ord_" + strings.ToLower(enc.EncodeToString(b)), nil
}

// Order adalah pesanan pembelian voucher.
type Order struct {
	ID              string // ord_<12 char base32>
	PackageID       int64
	BaseAmount      int
	UniqueCode      int
	TotalAmount     int
	KrisKrisInvoice string
	Status          string
	QRPayload       string
	ExpiresAt       *time.Time
	PaidAt          *time.Time
	FailedReason    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
