package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/usecase"
)

// fakeRouterSession meniru sesi router untuk test HTTP.
type fakeRouterSession struct {
	profiles []domain.HotspotProfile
	server   []domain.HotspotServerProfile
	wg       []domain.WalledGardenEntry
	dns      []domain.StaticDNSEntry
	allow    bool
}

func (s *fakeRouterSession) Close() error { return nil }

func (s *fakeRouterSession) Test() (domain.RouterResource, error) {
	return domain.RouterResource{BoardName: "CHR", Version: "7.14"}, nil
}

func (s *fakeRouterSession) ListUserProfiles() ([]domain.HotspotProfile, error) {
	return append([]domain.HotspotProfile{}, s.profiles...), nil
}

func (s *fakeRouterSession) CreateUserProfile(p domain.HotspotProfile) error {
	s.profiles = append(s.profiles, p)
	return nil
}

func (s *fakeRouterSession) AddHotspotUser(domain.HotspotUser) error { return nil }

func (s *fakeRouterSession) FindHotspotUserByComment(string) (domain.HotspotUser, bool, error) {
	return domain.HotspotUser{}, false, nil
}

func (s *fakeRouterSession) ListWalledGarden() ([]domain.WalledGardenEntry, error) {
	return append([]domain.WalledGardenEntry{}, s.wg...), nil
}

func (s *fakeRouterSession) AddWalledGarden(e domain.WalledGardenEntry) error {
	e.Comment = domain.RouterCommentMikbil
	s.wg = append(s.wg, e)
	return nil
}

func (s *fakeRouterSession) RemoveWalledGarden(id string) error {
	kept := s.wg[:0]
	for _, e := range s.wg {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	s.wg = kept
	return nil
}

func (s *fakeRouterSession) ListServerProfiles() ([]domain.HotspotServerProfile, error) {
	return append([]domain.HotspotServerProfile{}, s.server...), nil
}

func (s *fakeRouterSession) SetServerProfileLoginBy(id, loginBy string) error {
	for i, sp := range s.server {
		if sp.ID == id {
			s.server[i].LoginBy = loginBy
		}
	}
	return nil
}

func (s *fakeRouterSession) ListStaticDNS() ([]domain.StaticDNSEntry, error) {
	return append([]domain.StaticDNSEntry{}, s.dns...), nil
}

func (s *fakeRouterSession) AddStaticDNS(e domain.StaticDNSEntry) error {
	e.Comment = domain.RouterCommentMikbil
	s.dns = append(s.dns, e)
	return nil
}

func (s *fakeRouterSession) RemoveStaticDNS(id string) error {
	kept := s.dns[:0]
	for _, e := range s.dns {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	s.dns = kept
	return nil
}

func (s *fakeRouterSession) DNSAllowRemoteRequests() (bool, error) { return s.allow, nil }

func (s *fakeRouterSession) SetDNSAllowRemoteRequests(allow bool) error {
	s.allow = allow
	return nil
}

// fakeGateway meniru usecase.RouterGateway untuk test HTTP.
type fakeGateway struct {
	sess *fakeRouterSession
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{sess: &fakeRouterSession{
		profiles: []domain.HotspotProfile{{Name: "default", SharedUsers: 1}},
		server:   []domain.HotspotServerProfile{{ID: "*1", Name: "hsprof1", LoginBy: "chap"}},
	}}
}

func (g *fakeGateway) Dial(usecase.RouterDialParams) (usecase.RouterSession, error) {
	return g.sess, nil
}

// loginAdmin menyiapkan admin + sesi login, mengembalikan cookie dan header CSRF.
func loginAdmin(t *testing.T, h http.Handler) (*http.Cookie, map[string]string) {
	t.Helper()
	doRequest(t, h, "POST", "/api/setup/admin", map[string]string{
		"name": "O", "username": "owner", "password": "katasandi123",
	}, nil)
	rec, data := login(t, h, "owner", "katasandi123")
	if rec.Code != 200 {
		t.Fatalf("login = %d (%s)", rec.Code, rec.Body.String())
	}
	csrf, _ := data["csrf_token"].(string)
	return sessionCookie(rec), map[string]string{deliveryhttp.CSRFHeader: csrf}
}

func seedSite(t *testing.T, h http.Handler) {
	t.Helper()
	rec := doRequest(t, h, "POST", "/api/setup/site", map[string]string{
		"deploy_mode": "vps", "site_url": "https://wifi.contoh.com",
		"hotspot_login_base": "http://192.168.88.1/login",
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("seed site = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestAdminConnectionsCRUD(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	// Tanpa sesi: 401.
	rec := doRequest(t, h, "GET", "/api/admin/connections", nil, nil)
	if rec.Code != 401 {
		t.Fatalf("list tanpa sesi = %d, want 401", rec.Code)
	}
	// Tanpa CSRF: 403.
	rec = doRequest(t, h, "POST", "/api/admin/connections", map[string]string{}, nil, cookie)
	if rec.Code != 403 {
		t.Fatalf("create tanpa CSRF = %d, want 403", rec.Code)
	}

	// Create.
	rec = doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"name": "utama", "host": "192.168.88.1", "username": "admin",
		"password": "rahasia", "use_tls": false,
	}, csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("create = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &created)
	if created["port"] != float64(8728) || created["is_active"] != true {
		t.Fatalf("created = %v", created)
	}
	for _, k := range []string{"password", "enc_password"} {
		if _, exposed := created[k]; exposed {
			t.Fatalf("kunci %q terekspos di respons", k)
		}
	}
	id := fmt.Sprintf("%.0f", created["id"])

	// List: satu entries, tanpa password.
	rec = doRequest(t, h, "GET", "/api/admin/connections", nil, nil, cookie)
	var list []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &list)
	if len(list) != 1 || list[0]["host"] != "192.168.88.1" {
		t.Fatalf("list = %v", list)
	}

	// Update tanpa password: password lama dipertahankan (test tetap bisa dial).
	rec = doRequest(t, h, "PUT", "/api/admin/connections/"+id, map[string]any{
		"name": "utama", "host": "192.168.88.2", "port": 8728,
		"username": "admin", "use_tls": false, "is_active": true,
	}, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("update = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// Test koneksi tersimpan.
	rec = doRequest(t, h, "POST", "/api/admin/connections/"+id+"/test", nil, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("test = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var tres map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &tres)
	if tres["success"] != true {
		t.Fatalf("test success = %v", tres)
	}

	// ID tidak valid.
	rec = doRequest(t, h, "POST", "/api/admin/connections/999/test", nil, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("test id asing = %d, want 400", rec.Code)
	}

	// Delete.
	rec = doRequest(t, h, "DELETE", "/api/admin/connections/"+id, nil, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("delete = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, "GET", "/api/admin/connections", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &list)
	if len(list) != 0 {
		t.Fatalf("list setelah hapus = %v", list)
	}
}

func TestAdminRouterProfiles(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	// Tanpa koneksi aktif: 400 dengan pesan jelas.
	rec := doRequest(t, h, "GET", "/api/admin/router/profiles", nil, nil, cookie)
	if rec.Code != 400 {
		t.Fatalf("profiles tanpa koneksi = %d, want 400", rec.Code)
	}

	doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia",
	}, csrf, cookie)

	rec = doRequest(t, h, "GET", "/api/admin/router/profiles", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("list profiles = %d (%s)", rec.Code, rec.Body.String())
	}
	var ps []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &ps)
	if len(ps) != 1 || ps[0]["name"] != "default" {
		t.Fatalf("profiles = %v", ps)
	}

	rec = doRequest(t, h, "POST", "/api/admin/router/profiles", map[string]any{
		"name": "paket-5m", "rate_limit": "5M/5M", "shared_users": 1,
	}, csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("create profile = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, "POST", "/api/admin/router/profiles", map[string]any{
		"name": "",
	}, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("create tanpa nama = %d, want 400", rec.Code)
	}
}

func TestAdminProvisioning(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	seedSite(t, h)
	doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia",
	}, csrf, cookie)

	rec := doRequest(t, h, "GET", "/api/admin/provisioning/status", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var st map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &st)
	if st["deploy_mode"] != "vps" {
		t.Fatalf("status = %v", st)
	}

	rec = doRequest(t, h, "POST", "/api/admin/provisioning/apply", nil, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("apply = %d (%s)", rec.Code, rec.Body.String())
	}
	var ap map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &ap)
	steps, _ := ap["steps"].([]any)
	if len(steps) != 4 {
		t.Fatalf("apply steps = %d, want 4", len(steps))
	}
}

func TestAdminAllowlistCRUD(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	seedSite(t, h)
	doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia",
	}, csrf, cookie)

	rec := doRequest(t, h, "GET", "/api/admin/walled-garden", nil, nil)
	if rec.Code != 401 {
		t.Fatalf("list tanpa sesi = %d, want 401", rec.Code)
	}
	rec = doRequest(t, h, "POST", "/api/admin/walled-garden",
		map[string]string{"dst_host": "api.dana.id"}, nil, cookie)
	if rec.Code != 403 {
		t.Fatalf("create tanpa CSRF = %d, want 403", rec.Code)
	}

	rec = doRequest(t, h, "GET", "/api/admin/walled-garden", nil, nil, cookie)
	var list []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &list)
	if rec.Code != 200 || len(list) != 0 {
		t.Fatalf("list awal = %d %v", rec.Code, list)
	}

	for _, bad := range []string{"", "bukan domain!", "192.168.1.1", "wifi.contoh.com"} {
		rec = doRequest(t, h, "POST", "/api/admin/walled-garden",
			map[string]string{"dst_host": bad}, csrf, cookie)
		if rec.Code != 400 {
			t.Fatalf("create %q = %d, want 400", bad, rec.Code)
		}
	}

	rec = doRequest(t, h, "POST", "/api/admin/walled-garden",
		map[string]string{"dst_host": "API.DANA.id."}, csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("create = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &created)
	if created["dst_host"] != "api.dana.id" || created["enabled"] != true {
		t.Fatalf("created = %v", created)
	}
	if created["router_synced"] != true {
		t.Fatalf("router_synced = %v, want true", created["router_synced"])
	}
	id := fmt.Sprintf("%.0f", created["id"])

	rec = doRequest(t, h, "POST", "/api/admin/walled-garden",
		map[string]string{"dst_host": "api.dana.id"}, csrf, cookie)
	if rec.Code != 409 {
		t.Fatalf("duplikat = %d, want 409", rec.Code)
	}

	rec = doRequest(t, h, "PUT", "/api/admin/walled-garden/"+id,
		map[string]any{"enabled": false}, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("disable = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &updated)
	if updated["dst_host"] != "api.dana.id" || updated["enabled"] != false {
		t.Fatalf("updated = %v", updated)
	}

	rec = doRequest(t, h, "PUT", "/api/admin/walled-garden/999",
		map[string]any{"enabled": true}, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("update id asing = %d, want 400", rec.Code)
	}

	rec = doRequest(t, h, "DELETE", "/api/admin/walled-garden/"+id, nil, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("delete = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, "DELETE", "/api/admin/walled-garden/"+id, nil, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("hapus ganda = %d, want 400", rec.Code)
	}

	rec = doRequest(t, h, "GET", "/api/admin/walled-garden", nil, nil, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &list)
	if len(list) != 0 {
		t.Fatalf("list setelah hapus = %v", list)
	}
}

func TestAdminAllowlistBulk(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	seedSite(t, h)
	doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"host": "192.168.88.1", "username": "admin", "password": "rahasia",
	}, csrf, cookie)

	rec := doRequest(t, h, "POST", "/api/admin/walled-garden/bulk",
		map[string]any{"hosts": []string{"api.dana.id"}}, nil, cookie)
	if rec.Code != 403 {
		t.Fatalf("bulk tanpa CSRF = %d, want 403", rec.Code)
	}

	rec = doRequest(t, h, "POST", "/api/admin/walled-garden/bulk",
		map[string]any{"hosts": []string{"api.dana.id", "*.dana.id", "salah!"}},
		csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("bulk = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var res map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &res)
	if len(res["added"].([]any)) != 2 || len(res["failed"].([]any)) != 1 {
		t.Fatalf("bulk = %v", res)
	}
	if res["router_error"] != "" {
		t.Fatalf("router_error = %v", res["router_error"])
	}

	rec = doRequest(t, h, "POST", "/api/admin/walled-garden/bulk",
		map[string]any{"hosts": []string{"api.dana.id"}}, csrf, cookie)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &res)
	if len(res["added"].([]any)) != 0 || len(res["skipped"].([]any)) != 1 {
		t.Fatalf("bulk ulang = %v, want skipped", res)
	}

	many := make([]string, 101)
	for i := range many {
		many[i] = "host.example.id"
	}
	rec = doRequest(t, h, "POST", "/api/admin/walled-garden/bulk",
		map[string]any{"hosts": many}, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("bulk > 100 = %d, want 400", rec.Code)
	}
}

func TestAdminHotspotTemplateSnippet(t *testing.T) {
	h := newTestRouter(t)
	cookie, _ := loginAdmin(t, h)
	seedSite(t, h)

	rec := doRequest(t, h, "GET", "/api/admin/hotspot/button-snippet", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("snippet = %d (%s)", rec.Code, rec.Body.String())
	}
	var sn map[string]string
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &sn)
	if !strings.Contains(sn["snippet"], "https://wifi.contoh.com") {
		t.Fatalf("snippet tidak memuat site_url: %q", sn["snippet"])
	}

	rec = doRequest(t, h, "GET", "/api/admin/hotspot/login-template", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("template = %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "login.html") {
		t.Fatalf("content-disposition = %q", cd)
	}
	body := rec.Body.String()
	for _, want := range []string{"https://wifi.contoh.com", "$(chap-id)", "Beli Voucher WiFi"} {
		if !strings.Contains(body, want) {
			t.Fatalf("template tidak memuat %q", want)
		}
	}
}
