package usecase

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// Batas retry pembuatan invoice (tech_spec 4.1).
const (
	orderMaxRetry  = 3
	orderRetryCap  = 60 * time.Second
	orderLabelCap  = 64
	defaultExpires = 15 * time.Minute
)

// OrderUsecase menangani pembuatan dan pembacaan pesanan (tech_spec 4.1).
type OrderUsecase struct {
	orders   OrderRepository
	vouchers VoucherRepository
	pkgs     PackageRepository
	set      SettingsRepository
	box      SecretBox
	invoices InvoiceClientFactory

	// Sleep dijeda antar retry; bisa diganti di test.
	Sleep func(time.Duration)
	// Now jam; bisa diganti di test.
	Now func() time.Time
}

// NewOrderUsecase membuat OrderUsecase.
func NewOrderUsecase(orders OrderRepository, vouchers VoucherRepository,
	pkgs PackageRepository, set SettingsRepository, box SecretBox,
	invoices InvoiceClientFactory) *OrderUsecase {
	return &OrderUsecase{
		orders: orders, vouchers: vouchers, pkgs: pkgs,
		set: set, box: box, invoices: invoices,
		Sleep: time.Sleep, Now: time.Now,
	}
}

// OrderCreated adalah hasil pembuatan pesanan.
type OrderCreated struct {
	Order     domain.Order
	ExpiresAt time.Time
}

// Create membuat order: paket harus aktif, invoice dibuat di KrisKris
// dengan retry otomatis untuk duplicate_pending/429 (maks 3x).
func (u *OrderUsecase) Create(ctx context.Context, packageID int64) (OrderCreated, error) {
	p, err := u.pkgs.GetByID(packageID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return OrderCreated{}, InputError{"paket tidak ditemukan"}
		}
		return OrderCreated{}, err
	}
	if !p.IsActive {
		return OrderCreated{}, InputError{"paket sedang tidak tersedia"}
	}
	client, err := u.invoices()
	if err != nil {
		return OrderCreated{}, err
	}
	orderID, err := domain.NewOrderID()
	if err != nil {
		return OrderCreated{}, err
	}
	label := "Mikbil " + p.Name + " " + orderID
	if len(label) > orderLabelCap {
		label = label[:orderLabelCap]
	}
	inv, err := createWithRetry(ctx, client, p.PriceIDR, label, u.Sleep)
	if err != nil {
		var invErr InvoiceError
		if errors.As(err, &invErr) {
			return OrderCreated{}, InputError{"pembayaran sibuk, coba lagi sebentar: " + shortErr(err)}
		}
		return OrderCreated{}, err
	}
	expires := u.Now().Add(defaultExpires)
	if inv.ExpiresAt != nil {
		expires = *inv.ExpiresAt
	}
	o := domain.Order{
		ID: orderID, PackageID: p.ID,
		BaseAmount: inv.BaseAmount, UniqueCode: inv.UniqueCode,
		TotalAmount: inv.Amount, KrisKrisInvoice: inv.ID,
		Status: domain.OrderPending, QRPayload: inv.QRPayload,
		ExpiresAt: &expires,
	}
	if o.BaseAmount == 0 {
		o.BaseAmount = p.PriceIDR
	}
	if o.TotalAmount == 0 {
		o.TotalAmount = o.BaseAmount + o.UniqueCode
	}
	if err := u.orders.Create(o); err != nil {
		return OrderCreated{}, err
	}
	o.ExpiresAt = &expires
	return OrderCreated{Order: o, ExpiresAt: expires}, nil
}

func createWithRetry(ctx context.Context, c InvoiceClient, amount int, label string, sleep func(time.Duration)) (InvoiceInfo, error) {
	var err error
	var inv InvoiceInfo
	for attempt := 0; attempt <= orderMaxRetry; attempt++ {
		if attempt > 0 {
			if ctx.Err() != nil {
				return InvoiceInfo{}, ctx.Err()
			}
		}
		inv, err = c.CreateInvoice(ctx, amount, label)
		if err == nil {
			return inv, nil
		}
		var invErr InvoiceError
		if !errors.As(err, &invErr) || !invErr.Temporary() || attempt == orderMaxRetry {
			return InvoiceInfo{}, err
		}
		wait := time.Duration(invErr.RetryAfter) * time.Second
		if wait <= 0 {
			wait = time.Duration(attempt+1) * time.Second
		}
		if wait > orderRetryCap {
			wait = orderRetryCap
		}
		sleep(wait)
	}
	return InvoiceInfo{}, err
}

// VoucherCreds adalah kredensial voucher terdekripsi untuk pemilik order.
type VoucherCreds struct {
	Username    string
	Password    string
	ProfileName string
}

// OrderDetail adalah data GET /api/orders/:id.
type OrderDetail struct {
	Order    domain.Order
	Package  domain.Package
	Voucher  *VoucherCreds // hanya bila paid
	LoginURL string        // hanya bila paid
}

// Detail mengambil order + paket + (bila paid) voucher dan login_url.
func (u *OrderUsecase) Detail(orderID string) (OrderDetail, error) {
	o, err := u.orders.GetByID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return OrderDetail{}, InputError{"pesanan tidak ditemukan"}
		}
		return OrderDetail{}, err
	}
	p, err := u.pkgs.GetByID(o.PackageID)
	if err != nil {
		return OrderDetail{}, err
	}
	d := OrderDetail{Order: o, Package: p}
	if o.Status != domain.OrderPaid {
		return d, nil
	}
	v, err := u.vouchers.GetByOrderID(orderID)
	if err != nil {
		return OrderDetail{}, err
	}
	pw, err := u.box.Open(v.EncPassword)
	if err != nil {
		return OrderDetail{}, err
	}
	d.Voucher = &VoucherCreds{Username: v.Username, Password: pw, ProfileName: v.ProfileName}
	if base, found, _ := u.set.Get(domain.SettingHotspotLoginBase); found && base != "" {
		d.LoginURL = loginURL(base, v.Username, pw)
	}
	return d, nil
}

func loginURL(base, username, password string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "username=" + url.QueryEscape(username) +
		"&password=" + url.QueryEscape(password)
}

// Cancel membatalkan order pending; invoice ikut dibatalkan best-effort.
func (u *OrderUsecase) Cancel(ctx context.Context, orderID string) (domain.Order, error) {
	o, err := u.orders.GetByID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Order{}, InputError{"pesanan tidak ditemukan"}
		}
		return domain.Order{}, err
	}
	if o.Status != domain.OrderPending {
		return domain.Order{}, InputError{"pesanan tidak bisa dibatalkan"}
	}
	if o.KrisKrisInvoice != "" {
		if client, err := u.invoices(); err == nil {
			_ = client.CancelInvoice(ctx, o.KrisKrisInvoice)
		}
	}
	if err := u.orders.MarkCancelled(orderID); err != nil {
		return domain.Order{}, err
	}
	o.Status = domain.OrderCancelled
	return o, nil
}
