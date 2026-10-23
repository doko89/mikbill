package domain

import "time"

// Package adalah paket voucher yang dijual di katalog.
type Package struct {
	ID             int64
	Name           string
	Description    string
	PriceIDR       int
	ValiditySecs   int
	ValidityLabel  string
	ProfileName    string
	RateLimit      string
	SessionTimeout int
	SharedUsers    int
	IsActive       bool
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
