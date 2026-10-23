package http_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdminSettingsGetButuhLogin(t *testing.T) {
	h := newTestRouter(t)
	if rec := doRequest(t, h, "GET", "/api/admin/settings", nil, nil); rec.Code != 401 {
		t.Fatalf("tanpa sesi = %d, want 401", rec.Code)
	}
}

func TestAdminSettingsAlur(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	// Bawaan: panjang 8, secret belum terisi.
	rec := doRequest(t, h, "GET", "/api/admin/settings", nil, nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("GET = %d, want 200", rec.Code)
	}
	var awal map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &awal)
	if awal["voucher_length"] != float64(8) {
		t.Fatalf("panjang bawaan = %v, want 8", awal["voucher_length"])
	}
	if awal["kriskris_api_secret_set"] != false || awal["kriskris_callback_secret_set"] != false {
		t.Fatalf("penanda secret = %v", awal)
	}

	// PUT tanpa CSRF ditolak.
	bad := map[string]any{"voucher_length": 10}
	if rec := doRequest(t, h, "PUT", "/api/admin/settings", bad, nil, cookie); rec.Code != 403 {
		t.Fatalf("tanpa CSRF = %d, want 403", rec.Code)
	}
	// Panjang di luar batas ditolak.
	for _, n := range []int{5, 64} {
		b := map[string]any{"voucher_length": n}
		if rec := doRequest(t, h, "PUT", "/api/admin/settings", b, csrf, cookie); rec.Code != 400 {
			t.Fatalf("panjang %d = %d, want 400", n, rec.Code)
		}
	}

	// Simpan situs + voucher + kriskris.
	rec = doRequest(t, h, "PUT", "/api/admin/settings", map[string]any{
		"deploy_mode": "local", "site_url": "http://mikbil.lan",
		"local_server_ip": "10.10.10.2", "hotspot_login_base": "http://192.168.88.1/login",
		"voucher_length": 10, "kriskris_api_key": "kk-key",
		"kriskris_api_secret": "s3cr3t", "kriskris_callback_secret": "cb-s3cr3t",
	}, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("PUT = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var simpan map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &simpan)
	if simpan["site_url"] != "http://mikbil.lan" || simpan["voucher_length"] != float64(10) {
		t.Fatalf("tersimpan = %v", simpan)
	}
	if simpan["kriskris_api_key"] != "kk-key" || simpan["kriskris_api_secret_set"] != true {
		t.Fatalf("kriskris = %v", simpan)
	}
	// Secret tidak pernah dibaca balik.
	body := rec.Body.String()
	if strings.Contains(body, "s3cr3t") || strings.Contains(body, "cb-s3cr3t") {
		t.Fatalf("respons memuat secret: %s", body)
	}

	// Mode VPS dengan URL http ditolak (aturan gabungan).
	rec = doRequest(t, h, "PUT", "/api/admin/settings", map[string]any{
		"deploy_mode": "vps",
	}, csrf, cookie)
	if rec.Code != 400 {
		t.Fatalf("vps http = %d, want 400", rec.Code)
	}
}
