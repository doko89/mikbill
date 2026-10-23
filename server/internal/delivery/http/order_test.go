package http_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/infra/crypto"
	"github.com/mikbil/mikbil/internal/infra/kriskris"
	"github.com/mikbil/mikbil/internal/repository/sqlite"
	"github.com/mikbil/mikbil/internal/usecase"
)

// stubInvoice meniru gateway invoice untuk test HTTP.
type stubInvoice struct {
	mu        sync.Mutex
	createOut usecase.InvoiceInfo
	batchOut  []usecase.InvoiceStatusInfo
	batchErr  error
	cancelled []string
}

func (s *stubInvoice) CreateInvoice(_ context.Context, _ int, _ string) (usecase.InvoiceInfo, error) {
	return s.createOut, nil
}

func (s *stubInvoice) GetInvoice(_ context.Context, id string) (usecase.InvoiceInfo, error) {
	return usecase.InvoiceInfo{ID: id, Status: "pending"}, nil
}

func (s *stubInvoice) BatchStatus(_ context.Context, _ []string) ([]usecase.InvoiceStatusInfo, error) {
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	return s.batchOut, nil
}

func (s *stubInvoice) CancelInvoice(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelled = append(s.cancelled, id)
	return nil
}

type orderStack struct {
	handler http.Handler
	vouch   *sqlite.VoucherRepo
	stub    *stubInvoice
}

func newOrderStack(t *testing.T) orderStack {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "order.db"))
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

	encSecret, _ := box.Seal("api-secret-uji")
	encCallback, _ := box.Seal("callback-uji")
	_ = set.Set(domain.SettingKrisKrisAPIKey, "kunci-uji")
	_ = set.Set(domain.SettingKrisKrisAPISecret, encSecret)
	_ = set.Set(domain.SettingKrisKrisCallbackSec, encCallback)
	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
	if _, err := pkgRepo.Create(domain.Package{
		Name: "Harian", PriceIDR: 15000,
		ValidityLabel: "12 Jam", ProfileName: "default", IsActive: true,
	}); err != nil {
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
	stub := &stubInvoice{createOut: usecase.InvoiceInfo{
		ID: "inv_http1", Status: "pending", Amount: 15042,
		BaseAmount: 15000, UniqueCode: 42, QRPayload: "QRISI-UJI",
		ExpiresAt: &exp,
	}}
	factory := func() (usecase.InvoiceClient, error) { return stub, nil }
	gw := newFakeGateway()
	connUC := usecase.NewConnectionUsecase(conns, box, gw)
	profUC := usecase.NewProfileUsecase(connUC)
	provUC := usecase.NewProvisioningUsecase(set, connUC)
	hotUC := usecase.NewHotspotUsecase(set)
	setupUC := usecase.NewSetupUsecase(admins, conns, set, testHasher{}, box, connUC, provUC)
	orderUC := usecase.NewOrderUsecase(orders, vouch, pkgRepo, set, box, factory)
	payUC := usecase.NewPaymentUsecase(orders, vouch, hooks, audits,
		pkgRepo, set, box, connUC, factory, kriskris.VerifyWebhook)
	payUC.RandString = func(n int) (string, error) { return "KODEHTTP"[:n], nil }

	h := deliveryhttp.NewRouter(deliveryhttp.Deps{
		Setup: setupUC,
		Auth:  usecase.NewAuthUsecase(admins, sessions, set, testHasher{}, crypto.RandomBytes),
		Conn:  connUC, Prof: profUC, Prov: provUC, Hot: hotUC,
		Pkgs: usecase.NewPackageUsecase(pkgRepo, profUC),
		Ord:  orderUC, Pay: payUC,
	})
	return orderStack{handler: h, vouch: vouch, stub: stub}
}

func signCallback(secret string, body []byte) string {
	// doRequest memakai json.Encoder yang menambah newline di akhir body.
	raw := append(append([]byte{}, body...), '\n')
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func createTestOrder(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(t, h, "POST", "/api/orders", map[string]any{"package_id": 1}, nil)
	if rec.Code != 201 {
		t.Fatalf("buat order = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	return data
}

func TestOrderAPIPublic(t *testing.T) {
	st := newOrderStack(t)

	data := createTestOrder(t, st.handler)
	for _, k := range []string{"order_id", "qr_payload", "base_amount", "unique_code", "total_amount", "expires_at"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("respons create tanpa %s: %v", k, data)
		}
	}
	if data["total_amount"].(float64) != 15042 || data["qr_payload"].(string) != "QRISI-UJI" {
		t.Fatalf("create = %v", data)
	}
	orderID := data["order_id"].(string)

	rec := doRequest(t, st.handler, "GET", "/api/orders/"+orderID, nil, nil)
	if rec.Code != 200 {
		t.Fatalf("get = %d, want 200", rec.Code)
	}
	var detail map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &detail)
	if detail["status"].(string) != "pending" {
		t.Fatalf("status = %v", detail["status"])
	}
	if _, ok := detail["voucher"]; ok {
		t.Fatal("pending memuat voucher")
	}
	if _, ok := detail["login_url"]; ok {
		t.Fatal("pending memuat login_url")
	}

	rec = doRequest(t, st.handler, "POST", "/api/orders/"+orderID+"/cancel", map[string]any{}, nil)
	if rec.Code != 200 {
		t.Fatalf("cancel = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var cancelled map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &cancelled)
	if cancelled["status"].(string) != "cancelled" {
		t.Fatalf("cancel = %v", cancelled)
	}
	if len(st.stub.cancelled) != 1 {
		t.Fatalf("cancel invoice = %v", st.stub.cancelled)
	}
	rec = doRequest(t, st.handler, "POST", "/api/orders/"+orderID+"/cancel", map[string]any{}, nil)
	if rec.Code != 400 {
		t.Fatalf("cancel ganda = %d, want 400", rec.Code)
	}
	rec = doRequest(t, st.handler, "POST", "/api/orders", map[string]any{"package_id": 999}, nil)
	if rec.Code != 400 {
		t.Fatalf("paket asing = %d, want 400", rec.Code)
	}
}

func TestWebhookDedupSatuVoucher(t *testing.T) {
	st := newOrderStack(t)
	data := createTestOrder(t, st.handler)
	orderID := data["order_id"].(string)

	body := []byte(`{"event":"invoice.paid","delivery_id":"del_http1","invoice_id":"inv_http1"}`)
	headers := map[string]string{
		"X-Kriskris-Event":     "invoice.paid",
		"X-Kriskris-Delivery":  "del_http1",
		"X-Kriskris-Signature": signCallback("callback-uji", body),
	}
	for i := 0; i < 2; i++ {
		rec := doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(body), headers)
		if rec.Code != 200 {
			t.Fatalf("webhook %d = %d (%s), want 200", i, rec.Code, rec.Body.String())
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	var detail map[string]any
	for {
		rec := doRequest(t, st.handler, "GET", "/api/orders/"+orderID, nil, nil)
		detail = map[string]any{}
		_ = json.Unmarshal(mustEnvelope(t, rec).Data, &detail)
		if detail["status"].(string) == "paid" || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if detail["status"].(string) != "paid" {
		t.Fatalf("status = %v, want paid", detail["status"])
	}
	v, ok := detail["voucher"].(map[string]any)
	if !ok || v["username"].(string) != "KODEHTTP" || v["password"].(string) != "KODEHTTP" {
		t.Fatalf("voucher = %v", detail["voucher"])
	}
	if detail["login_url"].(string) != "http://192.168.88.1/login?username=KODEHTTP&password=KODEHTTP" {
		t.Fatalf("login_url = %v", detail["login_url"])
	}
	n, err := st.vouch.CountByOrder(orderID)
	if err != nil || n != 1 {
		t.Fatalf("voucher di DB = %d, err = %v (2 delivery sama harus 1 voucher)", n, err)
	}

	bad := map[string]string{
		"X-Kriskris-Event": "invoice.paid", "X-Kriskris-Delivery": "del_bad",
		"X-Kriskris-Signature": "sha256=" + hex.EncodeToString(make([]byte, 32)),
	}
	rec := doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(body), bad)
	if rec.Code != 401 {
		t.Fatalf("signature salah = %d, want 401", rec.Code)
	}

	ping := []byte(`{"event":"test.ping","delivery_id":"del_ping1"}`)
	rec = doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(ping), map[string]string{
		"X-Kriskris-Event": "test.ping", "X-Kriskris-Delivery": "del_ping1",
		"X-Kriskris-Signature": signCallback("callback-uji", ping),
	})
	if rec.Code != 200 {
		t.Fatalf("ping = %d, want 200", rec.Code)
	}
}

func TestWebhookExpiredTanpaVoucher(t *testing.T) {
	st := newOrderStack(t)
	data := createTestOrder(t, st.handler)
	orderID := data["order_id"].(string)

	body := []byte(`{"event":"invoice.expired","delivery_id":"del_exp1","invoice_id":"inv_http1"}`)
	rec := doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(body), map[string]string{
		"X-Kriskris-Event": "invoice.expired", "X-Kriskris-Delivery": "del_exp1",
		"X-Kriskris-Signature": signCallback("callback-uji", body),
	})
	if rec.Code != 200 {
		t.Fatalf("expired = %d, want 200", rec.Code)
	}
	rec = doRequest(t, st.handler, "GET", "/api/orders/"+orderID, nil, nil)
	var detail map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &detail)
	if detail["status"].(string) != "expired" {
		t.Fatalf("status = %v, want expired", detail["status"])
	}
	n, _ := st.vouch.CountByOrder(orderID)
	if n != 0 {
		t.Fatal("order expired punya voucher")
	}
}
