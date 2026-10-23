package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

func openTestOrders(t *testing.T) (*OrderRepo, *VoucherRepo, *WebhookRepo) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO packages (name, price_idr, validity_label, profile_name)
		 VALUES ('Harian', 15000, '12 Jam', 'default')`); err != nil {
		t.Fatal(err)
	}
	return NewOrderRepo(db), NewVoucherRepo(db), NewWebhookRepo(db)
}

func TestOrderRepoSiklus(t *testing.T) {
	orders, vouchers, _ := openTestOrders(t)
	exp := time.Now().Add(15 * time.Minute).UTC().Truncate(time.Second)
	o := domain.Order{ID: "ord_repo1", PackageID: 1, BaseAmount: 15000,
		UniqueCode: 42, TotalAmount: 15042, KrisKrisInvoice: "inv_r1",
		Status: domain.OrderPending, QRPayload: "QR", ExpiresAt: &exp}
	if err := orders.Create(o); err != nil {
		t.Fatal(err)
	}
	got, err := orders.GetByID("ord_repo1")
	if err != nil || got.TotalAmount != 15042 || got.QRPayload != "QR" {
		t.Fatalf("get = %+v, err = %v", got, err)
	}
	byInv, err := orders.GetByInvoice("inv_r1")
	if err != nil || byInv.ID != "ord_repo1" {
		t.Fatalf("byInvoice = %+v, err = %v", byInv, err)
	}
	if _, err := orders.GetByID("ord_tidakada"); err != domain.ErrNotFound {
		t.Fatalf("asing err = %v, want ErrNotFound", err)
	}

	claimed, err := orders.ClaimProcessing("ord_repo1")
	if err != nil || !claimed {
		t.Fatalf("claim = %v, err = %v", claimed, err)
	}
	claimed, err = orders.ClaimProcessing("ord_repo1")
	if err != nil || claimed {
		t.Fatalf("claim ganda = %v, want false", claimed)
	}

	paidAt := time.Now().UTC().Truncate(time.Second)
	if err := orders.CompletePaid("ord_repo1", domain.Voucher{
		OrderID: "ord_repo1", Username: "KODE1",
		EncPassword: "enc", ProfileName: "default",
	}, paidAt); err != nil {
		t.Fatal(err)
	}
	got, _ = orders.GetByID("ord_repo1")
	if got.Status != domain.OrderPaid || got.PaidAt == nil {
		t.Fatalf("paid = %+v", got)
	}
	v, err := vouchers.GetByOrderID("ord_repo1")
	if err != nil || v.Username != "KODE1" {
		t.Fatalf("voucher = %+v, err = %v", v, err)
	}
}

func TestOrderRepoPendingFreshStale(t *testing.T) {
	orders, _, _ := openTestOrders(t)
	now := time.Now()
	fresh := now.Add(5 * time.Minute)
	stale := now.Add(-time.Minute)
	mk := func(id string, exp *time.Time) {
		t.Helper()
		if err := orders.Create(domain.Order{ID: id, PackageID: 1,
			TotalAmount: 1000, KrisKrisInvoice: "inv-" + id,
			Status: domain.OrderPending, ExpiresAt: exp}); err != nil {
			t.Fatal(err)
		}
	}
	mk("ord_f1", &fresh)
	mk("ord_s1", &stale)
	stale2 := stale
	mk("ord_s2", &stale2)
	if err := orders.MarkPaid("ord_s2", now); err != nil {
		t.Fatal(err)
	}

	got, err := orders.PendingFresh(50, now)
	if err != nil || len(got) != 1 || got[0].ID != "ord_f1" {
		t.Fatalf("fresh = %+v, err = %v", got, err)
	}
	got, err = orders.PendingStale(50, now)
	if err != nil || len(got) != 1 || got[0].ID != "ord_s1" {
		t.Fatalf("stale = %+v, err = %v", got, err)
	}
	if err := orders.MarkExpired("ord_s1"); err != nil {
		t.Fatal(err)
	}
	if err := orders.MarkCancelled("ord_f1"); err != nil {
		t.Fatal(err)
	}
	if err := orders.MarkFailed("ord_s2", "uji"); err != nil {
		t.Fatal(err)
	}
	o, _ := orders.GetByID("ord_s2")
	if o.Status != domain.OrderFailed || o.FailedReason != "uji" {
		t.Fatalf("failed = %+v", o)
	}
}

func TestOrderRepoListCountAdmin(t *testing.T) {
	orders, _, _ := openTestOrders(t)
	now := time.Now()
	mk := func(id, status string, total int, paidAt *time.Time) {
		t.Helper()
		exp := now.Add(15 * time.Minute)
		o := domain.Order{ID: id, PackageID: 1, BaseAmount: total,
			TotalAmount: total, KrisKrisInvoice: "inv-" + id,
			Status: status, ExpiresAt: &exp}
		if err := orders.Create(o); err != nil {
			t.Fatal(err)
		}
		if status == domain.OrderPaid && paidAt != nil {
			if err := orders.MarkPaid(id, *paidAt); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("ord_l1", domain.OrderPending, 15000, nil)
	mk("ord_l2", domain.OrderPaid, 15000, &now)
	old := now.Add(-48 * time.Hour)
	mk("ord_l3", domain.OrderPaid, 25000, &old)

	got, err := orders.List("", "", 20, 0)
	if err != nil || len(got) != 3 {
		t.Fatalf("list = %d, err = %v", len(got), err)
	}
	n, err := orders.Count("", "")
	if err != nil || n != 3 {
		t.Fatalf("count = %d, err = %v", n, err)
	}
	got, _ = orders.List(domain.OrderPaid, "", 20, 0)
	if len(got) != 2 {
		t.Fatalf("list paid = %d, want 2", len(got))
	}
	got, _ = orders.List("", "ord_l1", 20, 0)
	if len(got) != 1 || got[0].ID != "ord_l1" {
		t.Fatalf("cari id = %+v", got)
	}
	got, _ = orders.List("", "inv-ord_l2", 20, 0)
	if len(got) != 1 || got[0].ID != "ord_l2" {
		t.Fatalf("cari invoice = %+v", got)
	}
	// Karakter LIKE diperlakukan harfiah.
	if _, err := orders.List("", "ord_%", 20, 0); err != nil {
		t.Fatal(err)
	}
	n, _ = orders.Count("", "ord_%")
	if n != 0 {
		t.Fatalf("cari ord_%% = %d, want 0 (harfiah)", n)
	}
	n, _ = orders.CountByStatus(domain.OrderPending)
	if n != 1 {
		t.Fatalf("pending = %d, want 1", n)
	}
	got, _ = orders.List("", "", 2, 2)
	if len(got) != 1 {
		t.Fatalf("halaman 2 = %d, want 1", len(got))
	}
	recent, err := orders.Recent(2)
	if err != nil || len(recent) != 2 {
		t.Fatalf("recent = %d, err = %v", len(recent), err)
	}

	mid := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	rev, err := orders.RevenueSince(mid)
	if err != nil || rev != 15000 {
		t.Fatalf("omzet = %d, err = %v, want 15000", rev, err)
	}
}

func TestOrderRepoRequeueFailed(t *testing.T) {
	orders, _, _ := openTestOrders(t)
	exp := time.Now().Add(15 * time.Minute)
	if err := orders.Create(domain.Order{ID: "ord_q1", PackageID: 1,
		TotalAmount: 1000, KrisKrisInvoice: "inv-q1",
		Status: domain.OrderFailed, FailedReason: "router mati",
		ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
	ok, err := orders.RequeueFailed("ord_q1")
	if err != nil || !ok {
		t.Fatalf("requeue = %v, err = %v", ok, err)
	}
	o, _ := orders.GetByID("ord_q1")
	if o.Status != domain.OrderPending || o.FailedReason != "" {
		t.Fatalf("setelah requeue = %+v", o)
	}
	ok, _ = orders.RequeueFailed("ord_q1")
	if ok {
		t.Fatal("requeue ganda lolos")
	}
	if err := orders.Create(domain.Order{ID: "ord_q2", PackageID: 1,
		TotalAmount: 1000, Status: domain.OrderPending,
		ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
	ok, _ = orders.RequeueFailed("ord_q2")
	if ok {
		t.Fatal("requeue pending lolos")
	}
}

func TestWebhookRepoDedup(t *testing.T) {
	_, _, hooks := openTestOrders(t)
	dup, err := hooks.Exists("del_1")
	if err != nil || dup {
		t.Fatalf("awal dup = %v", dup)
	}
	if err := hooks.Record(domain.WebhookDelivery{
		DeliveryID: "del_1", Event: "invoice.paid", InvoiceID: "inv_1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := hooks.Record(domain.WebhookDelivery{
		DeliveryID: "del_1", Event: "invoice.paid", InvoiceID: "inv_1",
	}); err != nil {
		t.Fatalf("record ganda err = %v", err)
	}
	dup, err = hooks.Exists("del_1")
	if err != nil || !dup {
		t.Fatalf("akhir dup = %v, err = %v", dup, err)
	}
}
