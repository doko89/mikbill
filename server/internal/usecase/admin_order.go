package usecase

import (
	"errors"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// Paginasi daftar transaksi admin (tech_spec 4.4).
const (
	adminOrdersPerPage = 20
	adminRecentLimit   = 8
)

// AdminOrderUsecase menangani daftar, detail, ringkasan, dan retry order (admin).
type AdminOrderUsecase struct {
	orders   OrderRepository
	vouchers VoucherRepository
	pkgs     PackageRepository
	set      SettingsRepository
	box      SecretBox
	pay      *PaymentUsecase

	// Now jam; bisa diganti di test.
	Now func() time.Time
}

// NewAdminOrderUsecase membuat AdminOrderUsecase.
func NewAdminOrderUsecase(orders OrderRepository, vouchers VoucherRepository,
	pkgs PackageRepository, set SettingsRepository, box SecretBox,
	pay *PaymentUsecase) *AdminOrderUsecase {
	return &AdminOrderUsecase{
		orders: orders, vouchers: vouchers, pkgs: pkgs,
		set: set, box: box, pay: pay, Now: time.Now,
	}
}

// AdminOrderItem adalah satu baris daftar transaksi.
type AdminOrderItem struct {
	Order       domain.Order
	PackageName string
}

// AdminOrderPage adalah hasil daftar transaksi berpaginasi.
type AdminOrderPage struct {
	Items   []AdminOrderItem
	Page    int
	PerPage int
	Total   int
}

// validStatus memeriksa filter status daftar admin.
func validStatus(s string) bool {
	switch s {
	case "", domain.OrderPending, domain.OrderProcess, domain.OrderPaid,
		domain.OrderExpired, domain.OrderCancelled, domain.OrderFailed:
		return true
	}
	return false
}

// List mengambil halaman transaksi dengan filter status dan pencarian.
func (u *AdminOrderUsecase) List(status, q string, page int) (AdminOrderPage, error) {
	status = strings.TrimSpace(status)
	if !validStatus(status) {
		return AdminOrderPage{}, InputError{"status filter tidak valid"}
	}
	if page < 1 {
		page = 1
	}
	q = strings.TrimSpace(q)
	total, err := u.orders.Count(status, q)
	if err != nil {
		return AdminOrderPage{}, err
	}
	found, err := u.orders.List(status, q, adminOrdersPerPage, (page-1)*adminOrdersPerPage)
	if err != nil {
		return AdminOrderPage{}, err
	}
	items := make([]AdminOrderItem, 0, len(found))
	for _, o := range found {
		name := ""
		if p, err := u.pkgs.GetByID(o.PackageID); err == nil {
			name = p.Name
		}
		items = append(items, AdminOrderItem{Order: o, PackageName: name})
	}
	return AdminOrderPage{Items: items, Page: page, PerPage: adminOrdersPerPage, Total: total}, nil
}

// AdminOrderDetail adalah detail order versi admin.
type AdminOrderDetail struct {
	Order    domain.Order
	Package  domain.Package
	Voucher  *VoucherCreds // terdekripsi; nil bila belum ada voucher
	LoginURL string        // terisi bila voucher ada
}

// Detail mengambil order + paket + voucher terdekripsi (bila sudah terbit).
func (u *AdminOrderUsecase) Detail(orderID string) (AdminOrderDetail, error) {
	o, err := u.orders.GetByID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AdminOrderDetail{}, InputError{"pesanan tidak ditemukan"}
		}
		return AdminOrderDetail{}, err
	}
	p, err := u.pkgs.GetByID(o.PackageID)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	d := AdminOrderDetail{Order: o, Package: p}
	v, err := u.vouchers.GetByOrderID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return d, nil
		}
		return AdminOrderDetail{}, err
	}
	pw, err := u.box.Open(v.EncPassword)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	d.Voucher = &VoucherCreds{Username: v.Username, Password: pw, ProfileName: v.ProfileName}
	if base, found, _ := u.set.Get(domain.SettingHotspotLoginBase); found && base != "" {
		d.LoginURL = loginURL(base, v.Username, pw)
	}
	return d, nil
}

// AdminSummary adalah angka ringkasan dasbor admin.
type AdminSummary struct {
	RevenueToday int
	PendingCount int
	FailedCount  int
	Recent       []AdminOrderItem
}

// Summary menghitung omzet hari ini, jumlah pending/gagal, dan transaksi terbaru.
func (u *AdminOrderUsecase) Summary() (AdminSummary, error) {
	now := u.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	revenue, err := u.orders.RevenueSince(start)
	if err != nil {
		return AdminSummary{}, err
	}
	pending, err := u.orders.CountByStatus(domain.OrderPending)
	if err != nil {
		return AdminSummary{}, err
	}
	failed, err := u.orders.CountByStatus(domain.OrderFailed)
	if err != nil {
		return AdminSummary{}, err
	}
	recent, err := u.orders.Recent(adminRecentLimit)
	if err != nil {
		return AdminSummary{}, err
	}
	items := make([]AdminOrderItem, 0, len(recent))
	for _, o := range recent {
		name := ""
		if p, err := u.pkgs.GetByID(o.PackageID); err == nil {
			name = p.Name
		}
		items = append(items, AdminOrderItem{Order: o, PackageName: name})
	}
	return AdminSummary{
		RevenueToday: revenue, PendingCount: pending,
		FailedCount: failed, Recent: items,
	}, nil
}

// Retry menerbitkan ulang voucher untuk order failed lewat satu-satunya
// pintu voucher (processPaidOrder). Mengembalikan detail status akhir.
func (u *AdminOrderUsecase) Retry(orderID string) (AdminOrderDetail, error) {
	o, err := u.orders.GetByID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AdminOrderDetail{}, InputError{"pesanan tidak ditemukan"}
		}
		return AdminOrderDetail{}, err
	}
	if o.Status != domain.OrderFailed {
		return AdminOrderDetail{}, InputError{"hanya pesanan gagal yang bisa diulang"}
	}
	if u.pay == nil {
		return AdminOrderDetail{}, errors.New("pemroses pembayaran belum dikonfigurasi")
	}
	ok, err := u.orders.RequeueFailed(orderID)
	if err != nil {
		return AdminOrderDetail{}, err
	}
	if !ok {
		return AdminOrderDetail{}, InputError{"hanya pesanan gagal yang bisa diulang"}
	}
	_ = u.pay.ProcessPaidOrder(orderID)
	return u.Detail(orderID)
}
