package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/infra/crypto"
	"github.com/mikbil/mikbil/internal/repository/sqlite"
	"github.com/mikbil/mikbil/internal/usecase"
)

const testEncKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

type testHasher struct{}

func (testHasher) Hash(pw string) (string, error) { return crypto.HashPassword(pw) }
func (testHasher) Check(hash, pw string) bool     { return crypto.CheckPassword(hash, pw) }

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
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
	h := testHasher{}
	gw := newFakeGateway()
	connUC := usecase.NewConnectionUsecase(conns, box, gw)
	profUC := usecase.NewProfileUsecase(connUC)
	provUC := usecase.NewProvisioningUsecase(set, connUC)
	hotUC := usecase.NewHotspotUsecase(set)
	return deliveryhttp.NewRouter(deliveryhttp.Deps{
		Setup: usecase.NewSetupUsecase(admins, conns, set, h, box, connUC, provUC),
		Auth:  usecase.NewAuthUsecase(admins, sessions, set, h, crypto.RandomBytes),
		Conn:  connUC, Prof: profUC, Prov: provUC, Hot: hotUC,
		Pkgs: usecase.NewPackageUsecase(sqlite.NewPackageRepo(db), profUC),
		Set:  usecase.NewSettingsUsecase(set, box),
	})
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
	Code  string          `json:"code"`
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
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

func mustEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("respons bukan envelope JSON: %q", rec.Body.String())
	}
	return e
}

func TestSetupStatusAwal(t *testing.T) {
	h := newTestRouter(t)
	rec := doRequest(t, h, "GET", "/api/setup/status", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	e := mustEnvelope(t, rec)
	if !e.OK {
		t.Fatal("status seharusnya ok")
	}
	var data map[string]bool
	_ = json.Unmarshal(e.Data, &data)
	if data["setup_completed"] || data["has_admin"] {
		t.Fatalf("status awal = %v, want keduanya false", data)
	}
}

func TestSetupFlowLengkap(t *testing.T) {
	h := newTestRouter(t)

	// 1. Admin.
	rec := doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "Owner", "username": "owner", "password": "katasandi123",
	}, nil)
	if rec.Code != 201 {
		t.Fatalf("buat admin = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "Lain", "username": "owner", "password": "katasandi123",
	}, nil)
	if rec.Code != 409 {
		t.Fatalf("admin ganda = %d, want 409", rec.Code)
	}
	rec = doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "X", "username": "ab", "password": "pendek",
	}, nil)
	if rec.Code != 400 || mustEnvelope(t, rec).OK {
		t.Fatalf("validasi admin lemah = %d, want 400 berenvelope gagal", rec.Code)
	}

	// Sinyal guard /setup: admin ada walau setup belum selesai.
	rec = doRequest(t, h, "GET", "/api/setup/status", nil, nil)
	var mid map[string]bool
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &mid)
	if !mid["has_admin"] || mid["setup_completed"] {
		t.Fatalf("status tengah = %v, want admin true dan belum selesai", mid)
	}

	// 2. Situs.
	rec = doRequest(t, h, "POST", "/api/setup/site", map[string]string{
		"deploy_mode": "vps", "site_url": "http://tanpa-tls.id", "hotspot_login_base": "http://192.168.88.1/login",
	}, nil)
	if rec.Code != 400 {
		t.Fatalf("VPS tanpa HTTPS = %d, want 400", rec.Code)
	}
	rec = doRequest(t, h, "POST", "/api/setup/site", map[string]string{
		"deploy_mode": "vps", "site_url": "https://wifi.contoh.com", "hotspot_login_base": "http://192.168.88.1/login",
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("simpan situs = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// 3. Koneksi (port default 8728).
	rec = doRequest(t, h, "POST", "/api/setup/connection", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia", "use_tls": false,
	}, nil)
	if rec.Code != 201 {
		t.Fatalf("simpan koneksi = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	var conn map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &conn)
	if conn["port"] != float64(8728) {
		t.Fatalf("port default = %v, want 8728", conn["port"])
	}
	if _, exposed := conn["enc_password"]; exposed {
		t.Fatal("password terenkripsi terekspos di respons")
	}

	// 4. KrisKris.
	rec = doRequest(t, h, "POST", "/api/setup/kriskris", map[string]string{
		"api_key": "key", "api_secret": "sec", "callback_secret": "cb",
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("simpan kriskris = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// Stub fase 4: 501 berenvelope.
	rec = doRequest(t, h, "POST", "/api/setup/kriskris/test", map[string]string{}, nil)
	if rec.Code != 501 {
		t.Fatalf("kriskris/test = %d, want 501", rec.Code)
	}
	if e := mustEnvelope(t, rec); e.OK || e.Code != "not_implemented" {
		t.Fatalf("kriskris/test envelope = %+v, want gagal + code", e)
	}

	// Tes koneksi router nyata (gateway palsu): mencatat hasil ke koneksi tersimpan.
	rec = doRequest(t, h, "POST", "/api/setup/connection/test", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia", "use_tls": false,
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("connection/test = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var testRes map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &testRes)
	if testRes["success"] != true {
		t.Fatalf("connection/test success = %v", testRes)
	}

	// Provision nyata (gateway palsu): kembalikan langkah-langkah.
	rec = doRequest(t, h, "POST", "/api/setup/provision", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("provision = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var provRes map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &provRes)
	steps, _ := provRes["steps"].([]any)
	if len(steps) != 4 {
		t.Fatalf("provision steps = %d, want 4 (mode VPS)", len(steps))
	}

	// 5. Finish (syarat: tes koneksi sukses).
	rec = doRequest(t, h, "POST", "/api/setup/finish", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("finish = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// Setelah finish: setup/* 404, status tetap 200.
	for _, path := range []string{"/api/setup/admin", "/api/setup/site", "/api/setup/finish"} {
		rec = doRequest(t, h, "POST", path, map[string]string{}, nil)
		if rec.Code != 404 || mustEnvelope(t, rec).OK {
			t.Fatalf("%s setelah finish = %d, want 404 berenvelope", path, rec.Code)
		}
	}
	rec = doRequest(t, h, "GET", "/api/setup/status", nil, nil)
	var st map[string]bool
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &st)
	if !st["setup_completed"] || !st["has_admin"] {
		t.Fatalf("status akhir = %v, want keduanya true", st)
	}
}

func TestSetupFinishButuhPrasyarat(t *testing.T) {
	h := newTestRouter(t)
	rec := doRequest(t, h, "POST", "/api/setup/finish", nil, nil)
	if rec.Code != 400 {
		t.Fatalf("finish tanpa admin = %d, want 400", rec.Code)
	}
	doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "O", "username": "owner", "password": "katasandi123",
	}, nil)
	rec = doRequest(t, h, "POST", "/api/setup/finish", nil, nil)
	if rec.Code != 400 {
		t.Fatalf("finish tanpa koneksi = %d, want 400", rec.Code)
	}
	doRequest(t, h, "POST", "/api/setup/connection", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia",
	}, nil)
	rec = doRequest(t, h, "POST", "/api/setup/finish", nil, nil)
	if rec.Code != 400 {
		t.Fatalf("finish tanpa tes sukses = %d, want 400", rec.Code)
	}
}

func login(t *testing.T, h http.Handler, username, password string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := doRequest(t, h, "POST", "/api/admin/login", map[string]string{
		"username": username, "password": password,
	}, nil)
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	return rec, data
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == deliveryhttp.SessionCookie {
			return c
		}
	}
	return nil
}

func TestAuthLoginLogoutMe(t *testing.T) {
	h := newTestRouter(t)
	doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "Owner", "username": "owner", "password": "katasandi123",
	}, nil)

	rec, _ := login(t, h, "owner", "salah")
	if rec.Code != 401 || mustEnvelope(t, rec).OK {
		t.Fatalf("login salah = %d, want 401 berenvelope", rec.Code)
	}
	rec, _ = login(t, h, "tidak-ada", "katasandi123")
	if rec.Code != 401 {
		t.Fatalf("login user asing = %d, want 401", rec.Code)
	}

	rec, data := login(t, h, "owner", "katasandi123")
	if rec.Code != 200 {
		t.Fatalf("login benar = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	csrf, _ := data["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("login tidak mengembalikan csrf_token")
	}
	cookie := sessionCookie(rec)
	if cookie == nil {
		t.Fatal("login tidak menyetel cookie sesi")
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie sesi harus HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}

	rec = doRequest(t, h, "GET", "/api/admin/me", nil, nil)
	if rec.Code != 401 {
		t.Fatalf("me tanpa cookie = %d, want 401", rec.Code)
	}
	rec = doRequest(t, h, "GET", "/api/admin/me", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("me dengan cookie = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// Logout tanpa CSRF ditolak.
	rec = doRequest(t, h, "POST", "/api/admin/logout", nil, nil, cookie)
	if rec.Code != 403 {
		t.Fatalf("logout tanpa CSRF = %d, want 403", rec.Code)
	}
	rec = doRequest(t, h, "POST", "/api/admin/logout", nil,
		map[string]string{deliveryhttp.CSRFHeader: csrf}, cookie)
	if rec.Code != 200 {
		t.Fatalf("logout = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, "GET", "/api/admin/me", nil, nil, cookie)
	if rec.Code != 401 {
		t.Fatalf("me setelah logout = %d, want 401", rec.Code)
	}
}

func TestSecureCookieMengikutiSkema(t *testing.T) {
	h := newTestRouter(t)
	doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "O", "username": "owner", "password": "katasandi123",
	}, nil)
	doRequest(t, h, "POST", "/api/setup/site", map[string]string{
		"deploy_mode": "local", "site_url": "http://mikbil.lan",
		"local_server_ip": "10.10.10.2", "hotspot_login_base": "http://192.168.88.1/login",
	}, nil)
	rec, _ := login(t, h, "owner", "katasandi123")
	if c := sessionCookie(rec); c == nil || c.Secure {
		t.Fatal("mode lokal (http) cookie tidak boleh Secure")
	}

	h2 := newTestRouter(t)
	doRequest(t, h2, "POST", "/api/setup/admin", map[string]string{
		"name": "O", "username": "owner", "password": "katasandi123",
	}, nil)
	doRequest(t, h2, "POST", "/api/setup/site", map[string]string{
		"deploy_mode": "vps", "site_url": "https://wifi.contoh.com",
		"hotspot_login_base": "http://192.168.88.1/login",
	}, nil)
	rec, _ = login(t, h2, "owner", "katasandi123")
	if c := sessionCookie(rec); c == nil || !c.Secure {
		t.Fatal("mode VPS (https) cookie harus Secure")
	}
}

func TestSetupKrisKrisCallbackPerMode(t *testing.T) {
	site := func(t *testing.T, h http.Handler, mode, url string) {
		t.Helper()
		rec := doRequest(t, h, "POST", "/api/setup/site", map[string]string{
			"deploy_mode": mode, "site_url": url,
			"local_server_ip": "10.10.10.2", "hotspot_login_base": "http://192.168.88.1/login",
		}, nil)
		if rec.Code != 200 {
			t.Fatalf("situs %s = %d (%s)", mode, rec.Code, rec.Body.String())
		}
	}
	// Mode lokal: callback secret opsional.
	hl := newTestRouter(t)
	site(t, hl, "local", "http://mikbil.lan")
	rec := doRequest(t, hl, "POST", "/api/setup/kriskris", map[string]string{
		"api_key": "k", "api_secret": "s",
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("lokal tanpa callback = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	// Mode VPS: callback secret wajib.
	hv := newTestRouter(t)
	site(t, hv, "vps", "https://wifi.contoh.com")
	rec = doRequest(t, hv, "POST", "/api/setup/kriskris", map[string]string{
		"api_key": "k", "api_secret": "s",
	}, nil)
	if rec.Code != 400 {
		t.Fatalf("vps tanpa callback = %d, want 400", rec.Code)
	}
	rec = doRequest(t, hv, "POST", "/api/setup/kriskris", map[string]string{
		"api_key": "k", "api_secret": "s", "callback_secret": "c",
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("vps dengan callback = %d (%s), want 200", rec.Code, rec.Body.String())
	}
}

func TestAPI404Berenvelope(t *testing.T) {
	h := newTestRouter(t)
	rec := doRequest(t, h, "GET", "/api/tidak-ada", nil, nil)
	if rec.Code != 404 {
		t.Fatalf("GET unknown = %d, want 404", rec.Code)
	}
	if e := mustEnvelope(t, rec); e.OK || e.Error == "" {
		t.Fatalf("envelope unknown = %+v", e)
	}
	rec = doRequest(t, h, "DELETE", "/api/juga-tidak-ada", nil, nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"ok":false`) {
		t.Fatalf("DELETE unknown = %d %s, want 404 berenvelope", rec.Code, rec.Body.String())
	}
}
