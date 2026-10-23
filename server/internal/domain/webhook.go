package domain

import "time"

// WebhookDelivery mencatat delivery_id callback KrisKris untuk dedup.
type WebhookDelivery struct {
	DeliveryID string
	Event      string
	InvoiceID  string
	ReceivedAt time.Time
}

// Event webhook KrisKris.
const (
	EventInvoicePaid    = "invoice.paid"
	EventInvoiceExpired = "invoice.expired"
	EventTestPing       = "test.ping"
)
