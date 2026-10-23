package kriskris

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Vektor dihitung independen via python hashlib (lihat riwayat kerja).
func TestSignRequestVektor(t *testing.T) {
	body := []byte(`{"amount":15000,"label":"Mikbil Harian ord_ABC"}`)
	got := SignRequest("rahasia-api-uji", "2026-10-23T10:00:00Z",
		"POST", "/api/v1/external/invoices", "", body)
	want := "v1=44b0a861fa90e9fa5cce08657402f3ebb616fe1c8ef44ce6781c52f717102b5f"
	if got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func TestSignRequestQueryMempengaruhi(t *testing.T) {
	a := SignRequest("s", "2026-10-23T10:00:00Z", "GET",
		"/api/v1/external/invoices/status", "ids=a,b", nil)
	b := SignRequest("s", "2026-10-23T10:00:00Z", "GET",
		"/api/v1/external/invoices/status", "", nil)
	if a == b {
		t.Fatal("query mentah harus ikut signature")
	}
	if !strings.HasPrefix(a, "v1=") {
		t.Fatalf("signature = %s, want prefix v1=", a)
	}
}

func TestVerifyWebhook(t *testing.T) {
	body := []byte(`{"event":"invoice.paid"}`)
	if !VerifyWebhook("callback-uji", body,
		"sha256=3f1a49dd5c47391ea00d5cfc695a738146c45f96791a7574894874efc5f339e4") {
		t.Fatal("signature valid ditolak")
	}
	for name, sig := range map[string]string{
		"salah":     "sha256=3f1a49dd5c47391ea00d5cfc695a738146c45f96791a7574894874efc5f339e5",
		"tanpa-pre": "3f1a49dd5c47391ea00d5cfc695a738146c45f96791a7574894874efc5f339e4",
		"pre-salah": "v1=3f1a49dd5c47391ea00d5cfc695a738146c45f96791a7574894874efc5f339e4",
		"bukan-hex": "sha256=zzzz",
		"pendek":    "sha256=abcd",
		"kosong":    "",
	} {
		if VerifyWebhook("callback-uji", body, sig) {
			t.Fatalf("signature %s diterima", name)
		}
	}
	if VerifyWebhook("secret-lain", body,
		"sha256=3f1a49dd5c47391ea00d5cfc695a738146c45f96791a7574894874efc5f339e4") {
		t.Fatal("secret salah diterima")
	}
}

// fakeServer memverifikasi signature lalu menjawab sesuai handler data.
func fakeServer(t *testing.T, secret string, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(body)
		}
		rawQuery := ""
		if i := strings.Index(r.URL.RequestURI(), "?"); i >= 0 {
			rawQuery = r.URL.RequestURI()[i+1:]
		}
		want := SignRequest(secret, r.Header.Get("X-Kriskris-Timestamp"),
			r.Method, r.URL.Path, rawQuery, body)
		if r.Header.Get("X-Kriskris-Signature") != want {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false, "error": "signature salah", "code": "unauthorized",
			})
			return
		}
		if r.Header.Get("X-API-Key") == "" {
			t.Error("X-API-Key kosong")
		}
		w.Header().Set("Date", time.Now().UTC().Format(time.RFC1123))
		h(w, r)
	}))
}

func testClient(srv *httptest.Server) *Client {
	return &Client{BaseURL: srv.URL, APIKey: "kunci", APISecret: "rahasia"}
}

func TestClientCreateGetBatchCancel(t *testing.T) {
	srv := fakeServer(t, "rahasia", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == pathInvoices:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{
				"id": "inv_1", "status": "pending", "amount": 15042,
				"base_amount": 15000, "unique_code": 42,
				"label": "Mikbil Harian", "qr_payload": "QRISI...",
				"expires_at": "2026-10-23T10:15:00Z",
			}})
		case r.Method == "GET" && r.URL.Path == pathStatus:
			if r.URL.Query().Get("ids") != "inv_1,inv_asing" {
				t.Errorf("query ids = %q", r.URL.Query().Get("ids"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []any{
				map[string]any{"id": "inv_1", "found": true, "status": "paid",
					"paid_at": "2026-10-23T10:05:00Z"},
				map[string]any{"id": "inv_asing", "found": false},
			}})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, pathInvoices+"/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{
				"id": "inv_1", "status": "paid", "amount": 15042,
				"base_amount": 15000, "unique_code": 42,
				"paid_at": "2026-10-23T10:05:00Z",
			}})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/cancel"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true,
				"data": map[string]any{"id": "inv_1", "status": "cancelled"}})
		default:
			t.Errorf("rute tak dikenal: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()
	c := testClient(srv)
	ctx := context.Background()

	inv, err := c.CreateInvoice(ctx, 15000, "Mikbil Harian")
	if err != nil {
		t.Fatal(err)
	}
	if inv.ID != "inv_1" || inv.UniqueCode != 42 || inv.QRPayload != "QRISI..." {
		t.Fatalf("invoice = %+v", inv)
	}
	if inv.ExpiresAt == nil {
		t.Fatal("expires_at tidak terurai")
	}

	got, err := c.GetInvoice(ctx, "inv_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "paid" || got.PaidAt == nil {
		t.Fatalf("detail = %+v", got)
	}

	st, err := c.BatchStatus(ctx, []string{"inv_1", "inv_asing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 || !st[0].Found || st[0].Status != "paid" || st[1].Found {
		t.Fatalf("batch = %+v", st)
	}

	if err := c.CancelInvoice(ctx, "inv_1"); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.ClockSkew(); !ok {
		t.Fatal("clock skew tidak terpantau dari header Date")
	}
}

func TestClientErrorEnvelope(t *testing.T) {
	newSrv := func(status int, body map[string]any) (*httptest.Server, *Client) {
		srv := fakeServer(t, "rahasia", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(body)
		})
		t.Cleanup(srv.Close)
		return srv, testClient(srv)
	}
	ctx := context.Background()

	_, c := newSrv(422, map[string]any{
		"ok": false, "error": "kode habis", "code": "duplicate_pending",
	})
	_, err := c.CreateInvoice(ctx, 15000, "x")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "duplicate_pending" {
		t.Fatalf("err = %v, want duplicate_pending", err)
	}

	_, c = newSrv(429, map[string]any{
		"ok": false, "error": "batas laju", "code": "rate_limited", "retry_after": 7,
	})
	_, err = c.CreateInvoice(ctx, 15000, "x")
	apiErr, ok = err.(*APIError)
	if !ok || apiErr.RetryAfter != 7 {
		t.Fatalf("err = %v, want retry_after 7", err)
	}

	_, c = newSrv(401, map[string]any{"ok": false, "error": "kunci salah"})
	_, err = c.CreateInvoice(ctx, 15000, "x")
	apiErr, ok = err.(*APIError)
	if !ok || apiErr.StatusCode != 401 {
		t.Fatalf("err = %v, want 401", err)
	}
}

func TestClientSignatureSalahDitolak(t *testing.T) {
	srv := fakeServer(t, "rahasia-benar", func(w http.ResponseWriter, r *http.Request) {
		t.Error("seharusnya tidak sampai ke handler")
	})
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: "kunci", APISecret: "rahasia-salah"}
	if _, err := c.GetInvoice(context.Background(), "inv_1"); err == nil {
		t.Fatal("signature salah lolos")
	}
}
