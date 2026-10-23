package usecase

import (
	"testing"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

func newAdminUC() (*fakeOrders, *fakePackages, *fakeSettings, *AdminOrderUsecase, *PaymentUsecase) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})
	admin := NewAdminOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, pay)
	return orders, pkgs, set, admin, pay
}

func seedOrder(t *testing.T, orders *fakeOrders, id string, pkgID int64, status string, total int) {
	t.Helper()
	exp := time.Now().Add(15 * time.Minute)
	if err := orders.Create(domain.Order{ID: id, PackageID: pkgID,
		BaseAmount: total, TotalAmount: total, KrisKrisInvoice: "inv-" + id,
		Status: status, ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
}

func TestAdminListFilterDanPaginasi(t *testing.T) {
	orders, pkgs, _, admin, _ := newAdminUC()
	p := seedPackage(t, pkgs, true)
	seedOrder(t, orders, "ord_a1", p.ID, domain.OrderPending, 15000)
	seedOrder(t, orders, "ord_a2", p.ID, domain.OrderPaid, 15000)
	seedOrder(t, orders, "ord_b1", p.ID, domain.OrderPaid, 25000)

	res, err := admin.List("", "", 1)
	if err != nil || res.Total != 3 || len(res.Items) != 3 {
		t.Fatalf("semua = %+v, err = %v", res, err)
	}
	if res.PerPage != 20 {
		t.Fatalf("perPage = %d, want 20", res.PerPage)
	}
	if res.Items[0].PackageName != "Harian" {
		t.Fatalf("nama paket = %q", res.Items[0].PackageName)
	}
	res, err = admin.List(domain.OrderPaid, "", 1)
	if err != nil || res.Total != 2 {
		t.Fatalf("paid = %+v, err = %v", res, err)
	}
	res, err = admin.List("", "ord_b", 1)
	if err != nil || res.Total != 1 || res.Items[0].Order.ID != "ord_b1" {
		t.Fatalf("cari = %+v, err = %v", res, err)
	}
	res, err = admin.List("", "inv-ord_a1", 1)
	if err != nil || res.Total != 1 {
		t.Fatalf("cari invoice = %+v, err = %v", res, err)
	}
	res, err = admin.List("", "", 2)
	if err != nil || res.Total != 3 || len(res.Items) != 0 {
		t.Fatalf("halaman 2 = %+v, err = %v", res, err)
	}
	if _, err := admin.List("bodong", "", 1); err == nil {
		t.Fatal("status asing lolos")
	}
}

func TestAdminDetailVoucherTerdekripsi(t *testing.T) {
	orders, pkgs, set, admin, pay := newAdminUC()
	p := seedPackage(t, pkgs, true)
	seedOrder(t, orders, "ord_d1", p.ID, domain.OrderPending, 15000)
	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
	if err := pay.ProcessPaidOrder("ord_d1"); err != nil {
		t.Fatal(err)
	}
	d, err := admin.Detail("ord_d1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Voucher == nil || d.Voucher.Password != "VCHR1234" {
		t.Fatalf("voucher = %+v", d.Voucher)
	}
	if d.LoginURL == "" {
		t.Fatal("login_url kosong")
	}
	d2, err := admin.Detail("ord_d1")
	if err != nil || d2.Package.Name != "Harian" {
		t.Fatalf("paket = %+v, err = %v", d2.Package, err)
	}
	if _, err := admin.Detail("ord_tidakada"); err == nil {
		t.Fatal("order asing lolos")
	}
	seedOrder(t, orders, "ord_d2", p.ID, domain.OrderPending, 15000)
	d3, err := admin.Detail("ord_d2")
	if err != nil || d3.Voucher != nil || d3.LoginURL != "" {
		t.Fatalf("pending harus tanpa voucher: %+v, err = %v", d3, err)
	}
}

func TestAdminSummary(t *testing.T) {
	orders, pkgs, _, admin, _ := newAdminUC()
	p := seedPackage(t, pkgs, true)
	now := time.Now()
	admin.Now = func() time.Time { return now }
	seedOrder(t, orders, "ord_s1", p.ID, domain.OrderPending, 15000)
	seedOrder(t, orders, "ord_s2", p.ID, domain.OrderPending, 25000)
	_ = orders.MarkFailed("ord_s2", "uji")
	seedOrder(t, orders, "ord_s3", p.ID, domain.OrderPaid, 15000)
	_ = orders.MarkPaid("ord_s3", now.Add(-time.Hour))
	seedOrder(t, orders, "ord_s4", p.ID, domain.OrderPaid, 99999)
	_ = orders.MarkPaid("ord_s4", now.Add(-48*time.Hour))

	s, err := admin.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if s.RevenueToday != 15000 {
		t.Fatalf("omzet = %d, want 15000", s.RevenueToday)
	}
	if s.PendingCount != 1 || s.FailedCount != 1 {
		t.Fatalf("pending = %d failed = %d", s.PendingCount, s.FailedCount)
	}
	if len(s.Recent) != 4 {
		t.Fatalf("terbaru = %d, want 4", len(s.Recent))
	}
	// Batas hari: paid tepat tengah malam ikut, sedetik sebelumnya tidak.
	mid := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	seedOrder(t, orders, "ord_s5", p.ID, domain.OrderPaid, 1000)
	_ = orders.MarkPaid("ord_s5", mid)
	seedOrder(t, orders, "ord_s6", p.ID, domain.OrderPaid, 2000)
	_ = orders.MarkPaid("ord_s6", mid.Add(-time.Second))
	s, _ = admin.Summary()
	if s.RevenueToday != 16000 {
		t.Fatalf("omzet batas hari = %d, want 16000", s.RevenueToday)
	}
}

func TestAdminRetryHanyaFailed(t *testing.T) {
	orders, pkgs, set, admin, _ := newAdminUC()
	p := seedPackage(t, pkgs, true)
	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
	seedOrder(t, orders, "ord_r1", p.ID, domain.OrderFailed, 15000)
	_ = orders.MarkFailed("ord_r1", "router mati")

	d, err := admin.Retry("ord_r1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Order.Status != domain.OrderPaid || d.Voucher == nil {
		t.Fatalf("setelah retry = %+v", d.Order)
	}
	if o, _ := orders.GetByID("ord_r1"); o.FailedReason != "" {
		t.Fatalf("alasan gagal tersisa: %q", o.FailedReason)
	}
	seedOrder(t, orders, "ord_r2", p.ID, domain.OrderPending, 15000)
	if _, err := admin.Retry("ord_r2"); err == nil {
		t.Fatal("retry pending lolos")
	}
	if _, err := admin.Retry("ord_r1"); err == nil {
		t.Fatal("retry paid lolos")
	}
	if _, err := admin.Retry("ord_tidakada"); err == nil {
		t.Fatal("retry asing lolos")
	}
}

func TestAdminRetryRouterTetapGagal(t *testing.T) {
	orders, pkgs, set, hooks, audits, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	conns := newFakeConns()
	connUC := NewConnectionUsecase(conns, fakeBox{}, &fakeGateway{sess: &fakeSession{}})
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})
	admin := NewAdminOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, pay)
	seedOrder(t, orders, "ord_r9", p.ID, domain.OrderFailed, 15000)

	d, err := admin.Retry("ord_r9")
	if err != nil {
		t.Fatalf("retry harus kembali 200 dengan status akhir: %v", err)
	}
	if d.Order.Status != domain.OrderFailed || d.Order.FailedReason == "" {
		t.Fatalf("status akhir = %+v", d.Order)
	}
	if d.Voucher != nil {
		t.Fatal("voucher tercipta walau router gagal")
	}
}
