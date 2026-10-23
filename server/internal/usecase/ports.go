// Package usecase berisi business logic; hanya tergantung interface (ports).
package usecase

import (
	"context"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// AdminRepository port penyimpanan admin.
type AdminRepository interface {
	Create(a domain.Admin) (domain.Admin, error)
	GetByID(id int64) (domain.Admin, error)
	GetByUsername(username string) (domain.Admin, error)
	Count() (int, error)
}

// SettingsRepository port key-value settings.
type SettingsRepository interface {
	Get(key string) (val string, found bool, err error)
	Set(key, val string) error
}

// ConnectionRepository port koneksi MikroTik.
type ConnectionRepository interface {
	CreateActive(c domain.Connection) (domain.Connection, error)
	GetActive() (domain.Connection, error)
	List() ([]domain.Connection, error)
	GetByID(id int64) (domain.Connection, error)
	Create(c domain.Connection) (domain.Connection, error)
	Update(c domain.Connection) (domain.Connection, error)
	Delete(id int64) error
	SaveTestResult(id int64, ok bool, msg string) error
}

// SessionRepository port sesi login.
type SessionRepository interface {
	Create(s domain.Session) error
	Get(id string) (domain.Session, error)
	Delete(id string) error
	DeleteExpired() error
}

// PackageRepository port penyimpanan paket voucher.
type PackageRepository interface {
	List() ([]domain.Package, error)
	ListActive() ([]domain.Package, error)
	GetByID(id int64) (domain.Package, error)
	Create(p domain.Package) (domain.Package, error)
	Update(p domain.Package) (domain.Package, error)
	Delete(id int64) error
}

// PasswordHasher port hashing password.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Check(hash, password string) bool
}

// OrderRepository port penyimpanan pesanan.
type OrderRepository interface {
	Create(o domain.Order) error
	GetByID(id string) (domain.Order, error)
	GetByInvoice(invoiceID string) (domain.Order, error)
	PendingFresh(limit int, now time.Time) ([]domain.Order, error)
	PendingStale(limit int, now time.Time) ([]domain.Order, error)
	ClaimProcessing(id string) (bool, error)
	CompletePaid(orderID string, v domain.Voucher, paidAt time.Time) error
	MarkPaid(id string, paidAt time.Time) error
	MarkExpired(id string) error
	MarkCancelled(id string) error
	MarkFailed(id, reason string) error
	List(status, q string, limit, offset int) ([]domain.Order, error)
	Count(status, q string) (int, error)
	CountByStatus(status string) (int, error)
	RevenueSince(since time.Time) (int, error)
	Recent(limit int) ([]domain.Order, error)
	RequeueFailed(id string) (bool, error)
}

// VoucherRepository port penyimpanan voucher.
type VoucherRepository interface {
	GetByOrderID(orderID string) (domain.Voucher, error)
}

// WebhookRepository port dedup callback.
type WebhookRepository interface {
	Exists(deliveryID string) (bool, error)
	Record(d domain.WebhookDelivery) error
}

// AuditRepository port log audit.
type AuditRepository interface {
	Log(actor, action, entity, entityID, meta string) error
}

// InvoiceInfo adalah data invoice dari gateway pembayaran.
type InvoiceInfo struct {
	ID         string
	Status     string
	Amount     int
	BaseAmount int
	UniqueCode int
	Label      string
	QRPayload  string
	ExpiresAt  *time.Time
	PaidAt     *time.Time
}

// InvoiceStatusInfo adalah satu butir batch status.
type InvoiceStatusInfo struct {
	ID     string
	Found  bool
	Status string
	PaidAt *time.Time
}

// InvoiceError adalah kegagalan gateway pembayaran.
type InvoiceError struct {
	Msg        string
	Code       string
	Status     int
	RetryAfter int // detik
}

func (e InvoiceError) Error() string { return e.Msg }

// Temporary melaporkan apakah layak retry (429 atau duplicate_pending).
func (e InvoiceError) Temporary() bool {
	return e.Status == 429 || e.Code == "duplicate_pending"
}

// InvoiceClient port gateway invoice KrisKris (implementasi: infra/kriskris).
type InvoiceClient interface {
	CreateInvoice(ctx context.Context, amount int, label string) (InvoiceInfo, error)
	GetInvoice(ctx context.Context, id string) (InvoiceInfo, error)
	BatchStatus(ctx context.Context, ids []string) ([]InvoiceStatusInfo, error)
	CancelInvoice(ctx context.Context, id string) error
}

// InvoiceClientFactory membuat InvoiceClient dari settings saat dipakai.
type InvoiceClientFactory func() (InvoiceClient, error)

// InvoiceClientBuilder membuat InvoiceClient dari kredensial eksplisit
// (untuk tes koneksi tanpa menyimpan).
type InvoiceClientBuilder func(key, secret string) (InvoiceClient, error)

// SecretBox port enkripsi nilai rahasia.
type SecretBox interface {
	Seal(plaintext string) (string, error)
	Open(ciphertext string) (string, error)
}

// DBPinger port cek hidup database (implementasi: *sql.DB).
type DBPinger interface {
	Ping() error
}

// SkewReader port pembaca clock skew terakhir terhadap KrisKris (detik).
// ok=false bila belum ada sampel.
type SkewReader interface {
	ClockSkewSec() (skew float64, ok bool)
}

// RouterDialParams adalah parameter dial ke router.
type RouterDialParams struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
}

// RouterSession adalah sesi terautentikasi ke satu router.
type RouterSession interface {
	Close() error
	Test() (domain.RouterResource, error)
	ListUserProfiles() ([]domain.HotspotProfile, error)
	CreateUserProfile(p domain.HotspotProfile) error
	AddHotspotUser(u domain.HotspotUser) error
	FindHotspotUserByComment(comment string) (domain.HotspotUser, bool, error)
	ListWalledGarden() ([]domain.WalledGardenEntry, error)
	AddWalledGarden(e domain.WalledGardenEntry) error
	RemoveWalledGarden(id string) error
	ListServerProfiles() ([]domain.HotspotServerProfile, error)
	SetServerProfileLoginBy(id, loginBy string) error
	ListStaticDNS() ([]domain.StaticDNSEntry, error)
	AddStaticDNS(e domain.StaticDNSEntry) error
	RemoveStaticDNS(id string) error
	DNSAllowRemoteRequests() (bool, error)
	SetDNSAllowRemoteRequests(allow bool) error
}

// RouterGateway port operasi RouterOS API (implementasi: infra/mikrotik).
type RouterGateway interface {
	Dial(p RouterDialParams) (RouterSession, error)
}
