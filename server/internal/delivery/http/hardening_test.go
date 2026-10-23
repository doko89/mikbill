package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/repository/sqlite"
	"github.com/mikbil/mikbil/internal/usecase"
)

// doRequestFrom sama dengan doRequest tetapi dari alamat IP tertentu.
func doRequestFrom(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string, remoteAddr string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustRateLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	e := mustEnvelope(t, rec)
	if e.OK || e.Code != "rate_limited" {
		t.Fatalf("envelope = %+v, want gagal + code rate_limited", e)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("header Retry-After kosong")
	}
}

func TestRateLimitLogin(t *testing.T) {
	h := newTestRouter(t)
	doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "O", "username": "owner", "password": "katasandi123",
	}, nil)
	bad := map[string]string{"username": "owner", "password": "salah"}
	for i := 0; i < 5; i++ {
		if rec := doRequest(t, h, "POST", "/api/admin/login", bad, nil); rec.Code != 401 {
			t.Fatalf("login %d = %d, want 401", i, rec.Code)
		}
	}
	mustRateLimited(t, doRequest(t, h, "POST", "/api/admin/login", bad, nil))
	// IP lain tidak ikut terblokir.
	rec := doRequestFrom(t, h, "POST", "/api/admin/login", bad, nil, "10.9.9.9:1234")
	if rec.Code != 401 {
		t.Fatalf("login IP lain = %d, want 401", rec.Code)
	}
}

func TestRateLimitCreateOrder(t *testing.T) {
	st := newOrderStack(t)
	for i := 0; i < 10; i++ {
		exp := time.Now().Add(15 * time.Minute)
		st.stub.createOut = usecase.InvoiceInfo{
			ID: fmt.Sprintf("inv_rl%d", i), Status: "pending",
			Amount: 15000 + i, BaseAmount: 15000, UniqueCode: 10 + i,
			QRPayload: "QRISI-RL", ExpiresAt: &exp,
		}
		rec := doRequest(t, st.handler, "POST", "/api/orders", map[string]any{"package_id": 1}, nil)
		if rec.Code != 201 {
			t.Fatalf("create %d = %d (%s), want 201", i, rec.Code, rec.Body.String())
		}
	}
	mustRateLimited(t, doRequest(t, st.handler, "POST", "/api/orders", map[string]any{"package_id": 1}, nil))
}

func TestRateLimitGetOrder(t *testing.T) {
	st := newOrderStack(t)
	first := createTestOrder(t, st.handler)
	id1 := first["order_id"].(string)
	for i := 0; i < 30; i++ {
		if rec := doRequest(t, st.handler, "GET", "/api/orders/"+id1, nil, nil); rec.Code != 200 {
			t.Fatalf("get %d = %d, want 200", i, rec.Code)
		}
	}
	mustRateLimited(t, doRequest(t, st.handler, "GET", "/api/orders/"+id1, nil, nil))
	// Order lain dari IP sama tetap boleh (batas per IP per order).
	exp := time.Now().Add(15 * time.Minute)
	st.stub.createOut = usecase.InvoiceInfo{
		ID: "inv_rl_beda", Status: "pending", Amount: 15099,
		BaseAmount: 15000, UniqueCode: 99, QRPayload: "QRISI-RL",
		ExpiresAt: &exp,
	}
	second := createTestOrder(t, st.handler)
	rec := doRequest(t, st.handler, "GET", "/api/orders/"+second["order_id"].(string), nil, nil)
	if rec.Code != 200 {
		t.Fatalf("get order lain = %d, want 200", rec.Code)
	}
}

func TestRateLimitWebhook(t *testing.T) {
	st := newOrderStack(t)
	for i := 0; i < 120; i++ {
		body := []byte(fmt.Sprintf(`{"event":"test.ping","delivery_id":"del_rl%d"}`, i))
		rec := doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(body), map[string]string{
			"X-Kriskris-Event": "test.ping", "X-Kriskris-Delivery": fmt.Sprintf("del_rl%d", i),
			"X-Kriskris-Signature": signCallback("callback-uji", body),
		})
		if rec.Code != 200 {
			t.Fatalf("ping %d = %d (%s), want 200", i, rec.Code, rec.Body.String())
		}
	}
	body := []byte(`{"event":"test.ping","delivery_id":"del_rl_lebih"}`)
	mustRateLimited(t, doRequest(t, st.handler, "POST", "/api/webhooks/kriskris", json.RawMessage(body), map[string]string{
		"X-Kriskris-Event": "test.ping", "X-Kriskris-Delivery": "del_rl_lebih",
		"X-Kriskris-Signature": signCallback("callback-uji", body),
	}))
}

type stubSkew struct {
	sec float64
	ok  bool
}

func (s stubSkew) ClockSkewSec() (float64, bool) { return s.sec, s.ok }

func newHealthRouter(t *testing.T, skew usecase.SkewReader, seedConn bool) http.Handler {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(db); err != nil {
		t.Fatal(err)
	}
	conns := sqlite.NewConnectionRepo(db)
	if seedConn {
		c, err := conns.CreateActive(domain.Connection{
			Name: "default", Host: "10.0.0.1", Port: 8728,
			Username: "admin", EncPassword: "x",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := conns.SaveTestResult(c.ID, true, "koneksi router berhasil"); err != nil {
			t.Fatal(err)
		}
	}
	return deliveryhttp.NewRouter(deliveryhttp.Deps{
		Health: usecase.NewHealthUsecase(db, conns, skew),
	})
}

func TestHealthKontrak(t *testing.T) {
	h := newHealthRouter(t, stubSkew{}, false)
	rec := doRequest(t, h, "GET", "/api/health", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("health = %d, want 200", rec.Code)
	}
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	for _, k := range []string{"time", "db", "clock_skew_sec", "router_last_test"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("health tanpa kunci %s: %v", k, data)
		}
	}
	if data["db"] != "ok" {
		t.Fatalf("db = %v, want ok", data["db"])
	}
	if _, err := time.Parse(time.RFC3339, data["time"].(string)); err != nil {
		t.Fatalf("time bukan RFC3339: %v", data["time"])
	}
	if data["clock_skew_sec"] != nil || data["router_last_test"] != nil {
		t.Fatalf("skew/router = %v/%v, want null", data["clock_skew_sec"], data["router_last_test"])
	}
}

func TestHealthSkewDanRouter(t *testing.T) {
	h := newHealthRouter(t, stubSkew{sec: 75, ok: true}, true)
	rec := doRequest(t, h, "GET", "/api/health", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("health = %d, want 200", rec.Code)
	}
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data["clock_skew_sec"].(float64) != 75 {
		t.Fatalf("skew = %v, want 75", data["clock_skew_sec"])
	}
	router, _ := data["router_last_test"].(map[string]any)
	if router["ok"] != true || router["message"] == "" || router["at"] == nil {
		t.Fatalf("router_last_test = %v", data["router_last_test"])
	}
}

func TestHealthDBGagal503(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "mati.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlite.Migrate(db); err != nil {
		t.Fatal(err)
	}
	conns := sqlite.NewConnectionRepo(db)
	_ = db.Close()
	h := deliveryhttp.NewRouter(deliveryhttp.Deps{
		Health: usecase.NewHealthUsecase(db, conns, stubSkew{}),
	})
	rec := doRequest(t, h, "GET", "/api/health", nil, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("health db mati = %d, want 503", rec.Code)
	}
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data["db"] != "gagal" {
		t.Fatalf("db = %v, want gagal", data["db"])
	}
}

func TestCSRFPUTDanDelete(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	body := packageBody()
	rec := doRequest(t, h, "POST", "/api/admin/packages", body, csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("buat paket = %d (%s)", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &created)
	id := fmt.Sprintf("%.0f", created["id"].(float64))
	if rec := doRequest(t, h, "PUT", "/api/admin/packages/"+id, body, nil, cookie); rec.Code != 403 {
		t.Fatalf("PUT tanpa CSRF = %d, want 403", rec.Code)
	}
	if rec := doRequest(t, h, "DELETE", "/api/admin/packages/"+id, nil, nil, cookie); rec.Code != 403 {
		t.Fatalf("DELETE tanpa CSRF = %d, want 403", rec.Code)
	}
	if rec := doRequest(t, h, "DELETE", "/api/admin/packages/"+id, nil, csrf, cookie); rec.Code != 200 {
		t.Fatalf("DELETE dengan CSRF = %d, want 200", rec.Code)
	}
}
