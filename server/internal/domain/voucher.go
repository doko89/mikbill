package domain

import "time"

// Voucher adalah kredensial hotspot hasil pembayaran lunas.
type Voucher struct {
	ID          int64
	OrderID     string
	Username    string
	EncPassword string // AES-GCM
	ProfileName string
	CreatedAt   time.Time
}
