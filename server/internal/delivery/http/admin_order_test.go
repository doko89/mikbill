package http_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/infra/crypto"
	"github.com/mikbil/mikbil/internal/infra/kriskris"
	"github.com/mikbil/mikbil/internal/repository/sqlite"
	"github.com/mikbil/mikbil/internal/usecase"
	"net/http"
)

type adminOrderStack struct {
	handler http.Handler
	orders  *sqlite.OrderRepo
	pkgs    *sqlite.PackageRepo
}

func newAdminOrderStack(t *testing.T) adminOrderStack {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "admin-order.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(db); err != nil {
		t.Fatal(err)
	}
	box, err := crypto.NewProtector(testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	admins := sqlite.NewAdminRepo(db)
	conns := sqlite.NewConnectionRepo(db)
	set := sqlite.NewSettingsRepo(db)
	sessions := sqlite.NewSessionRepo(db)
	pkgRepo := sqlite.NewPackageRepo(db)
	orders := sqlite.NewOrderRepo(db)
	vouch := sqlite.NewVoucherRepo(db)
	hooks := sqlite.NewWebhookRepo(db)
	audits := sqlite.NewAuditRepo(db)

	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
	pkg, err := pkgRepo.Create(domain.Package{
		Name: "Harian", PriceIDR: 15000,
		ValidityLabel: "12 Jam", ProfileName: "default", IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	encPw, _ := box.Seal("router-pw")
	if _, err := conns.CreateActive(domain.Connection{
		Name: "default", Host: "10.0.0.1", Port: 8728,
		Username: "admin", EncPassword: encPw,
	}); err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(15 * time.Minute)
	mk := func(id, status string, total int) {
		t.Helper()
		if err := orders.Create(domain.Order{ID: id, PackageID: pkg.ID,
			BaseAmount: total, TotalAmount: total, KrisKrisInvoice: "inv-" + id,
			Status: status, ExpiresAt: &exp}); err != nil {
			t.Fatal(err)
		}
	}
	mk("ord_t1", domain.OrderPending, 15000)
	mk("ord_t2", domain.OrderPending, 25000)
	_ = orders.MarkFailed("ord_t2", "router mati")
	paidAt := time.Now()
	mk("ord_t3", domain.OrderPaid, 15000)
	_ = orders.MarkPaid("ord_t3", paidAt)

	gw := newFakeGateway()
	connUC := usecase.NewConnectionUsecase(conns, box, gw)
	profUC := usecase.NewProfileUsecase(connUC)
	provUC := usecase.NewProvisioningUsecase(set, connUC)
	hotUC := usecase.NewHotspotUsecase(set)
	stub := &stubInvoice{}
	factory := func() (usecase.InvoiceClient, error) { return stub, nil }
	setupUC := usecase.NewSetupUsecase(admins, conns, set, testHasher{}, box, connUC, provUC)
	orderUC := usecase.NewOrderUsecase(orders, vouch, pkgRepo, set, box, factory)
	payUC := usecase.NewPaymentUsecase(orders, vouch, hooks, audits,
		pkgRepo, set, box, connUC, factory, kriskris.VerifyWebhook)
	payUC.RandString = func(n int) (string, error) { return "RTRYC0DE"[:n], nil }
	adminUC := usecase.NewAdminOrderUsecase(orders, vouch, pkgRepo, set, box, payUC)

	h := deliveryhttp.NewRouter(deliveryhttp.Deps{
		Setup: setupUC,
		Auth:  usecase.NewAuthUsecase(admins, sessions, set, testHasher{}, crypto.RandomBytes),
		Conn:  connUC, Prof: profUC, Prov: provUC, Hot: hotUC,
		Pkgs: usecase.NewPackageUsecase(pkgRepo, profUC),
		Ord:  orderUC, Pay: payUC, Adm: adminUC,
	})
	return adminOrderStack{handler: h, orders: orders, pkgs: pkgRepo}
}

func TestAdminOrdersButuhLogin(t *testing.T) {
	st := newAdminOrderStack(t)
	for _, path := range []string{
		"/api/admin/orders", "/api/admin/orders/summary", "/api/admin/orders/ord_t1",
	} {
		if rec := doRequest(t, st.handler, "GET", path, nil, nil); rec.Code != 401 {
			t.Fatalf("GET %s tanpa sesi = %d, want 401", path, rec.Code)
		}
	}
}

func TestAdminOrdersDaftar(t *testing.T) {
	st := newAdminOrderStack(t)
	cookie, csrf := loginAdmin(t, st.handler)
	_ = csrf

	rec := doRequest(t, st.handler, "GET", "/api/admin/orders", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("list = %d (%s)", rec.Code, rec.Body.String())
	}
	var data struct {
		Items   []map[string]any `json:"items"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
		Total   int              `json:"total"`
	}
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data.Total != 3 || len(data.Items) != 3 || data.PerPage != 20 {
		t.Fatalf("list = %+v", data)
	}
	if data.Items[0]["package_name"] != "Harian" {
		t.Fatalf("nama paket = %v", data.Items[0]["package_name"])
	}
	// Terbaru dulu.
	if data.Items[0]["order_id"] != "ord_t3" {
		t.Fatalf("urutan = %v, want ord_t3 dulu", data.Items[0]["order_id"])
	}

	rec = doRequest(t, st.handler, "GET", "/api/admin/orders?status=paid", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data.Total != 1 {
		t.Fatalf("filter paid = %+v", data)
	}
	rec = doRequest(t, st.handler, "GET", "/api/admin/orders?q=ord_t1", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data.Total != 1 || data.Items[0]["order_id"] != "ord_t1" {
		t.Fatalf("cari = %+v", data)
	}
	rec = doRequest(t, st.handler, "GET", "/api/admin/orders?page=2", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data.Total != 3 || len(data.Items) != 0 {
		t.Fatalf("halaman 2 = %+v", data)
	}
	if rec := doRequest(t, st.handler, "GET", "/api/admin/orders?status=bodong", nil, nil, cookie); rec.Code != 400 {
		t.Fatalf("status asing = %d, want 400", rec.Code)
	}
	if rec := doRequest(t, st.handler, "GET", "/api/admin/orders?page=0", nil, nil, cookie); rec.Code != 400 {
		t.Fatalf("page 0 = %d, want 400", rec.Code)
	}
}

func TestAdminOrdersDetailDanRingkasan(t *testing.T) {
	st := newAdminOrderStack(t)
	cookie, _ := loginAdmin(t, st.handler)

	rec := doRequest(t, st.handler, "GET", "/api/admin/orders/ord_t1", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("detail = %d (%s)", rec.Code, rec.Body.String())
	}
	var detail map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &detail)
	if _, ada := detail["voucher"]; ada {
		t.Fatal("pending tidak boleh membawa voucher")
	}
	if rec := doRequest(t, st.handler, "GET", "/api/admin/orders/ord_tidakada", nil, nil, cookie); rec.Code != 400 {
		t.Fatalf("detail asing = %d, want 400", rec.Code)
	}

	rec = doRequest(t, st.handler, "GET", "/api/admin/orders/summary", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("summary = %d (%s)", rec.Code, rec.Body.String())
	}
	var sum map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &sum)
	if sum["revenue_today"] != float64(15000) {
		t.Fatalf("omzet = %v, want 15000", sum["revenue_today"])
	}
	if sum["pending_count"] != float64(1) || sum["failed_count"] != float64(1) {
		t.Fatalf("ringkasan = %v", sum)
	}
	if recent, _ := sum["recent"].([]any); len(recent) != 3 {
		t.Fatalf("terbaru = %d, want 3", len(recent))
	}
}

func TestAdminOrdersRetry(t *testing.T) {
	st := newAdminOrderStack(t)
	cookie, csrf := loginAdmin(t, st.handler)

	// Retry tanpa CSRF ditolak.
	if rec := doRequest(t, st.handler, "POST", "/api/admin/orders/ord_t2/retry", map[string]any{}, nil, cookie); rec.Code != 403 {
		t.Fatalf("tanpa CSRF = %d, want 403", rec.Code)
	}
	// Retry order pending ditolak.
	if rec := doRequest(t, st.handler, "POST", "/api/admin/orders/ord_t1/retry", map[string]any{}, csrf, cookie); rec.Code != 400 {
		t.Fatalf("retry pending = %d, want 400", rec.Code)
	}
	// Retry order failed menerbitkan voucher.
	rec := doRequest(t, st.handler, "POST", "/api/admin/orders/ord_t2/retry", map[string]any{}, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("retry = %d (%s)", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &out)
	if out["status"] != domain.OrderPaid {
		t.Fatalf("status = %v, want paid", out["status"])
	}
	v, _ := out["voucher"].(map[string]any)
	if v["username"] != "RTRYC0DE" || v["password"] != "RTRYC0DE" {
		t.Fatalf("voucher = %v", v)
	}
	if out["login_url"] == "" {
		t.Fatal("login_url kosong")
	}
	// Retry kedua ditolak karena sudah paid.
	if rec := doRequest(t, st.handler, "POST", "/api/admin/orders/ord_t2/retry", map[string]any{}, csrf, cookie); rec.Code != 400 {
		t.Fatalf("retry ganda = %d, want 400", rec.Code)
	}
	// Detail admin menampilkan password terdekripsi.
	rec = doRequest(t, st.handler, "GET", "/api/admin/orders/ord_t2", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &out)
	v, _ = out["voucher"].(map[string]any)
	if v["password"] != "RTRYC0DE" {
		t.Fatalf("detail password = %v", v)
	}
}
