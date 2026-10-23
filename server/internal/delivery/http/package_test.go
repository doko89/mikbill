package http_test

import (
	"encoding/json"
	"testing"
)

func packageBody() map[string]any {
	return map[string]any{
		"name": "Harian", "description": "Internet 12 jam", "price_idr": 5000,
		"validity_seconds": 43200, "validity_label": "12 Jam",
		"profile_name": "harian", "rate_limit": "5M/10M",
		"session_timeout": 0, "shared_users": 1,
		"is_active": true, "sort_order": 1,
	}
}

func TestPublicPackagesHanyaAktif(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	b := packageBody()
	b["is_active"] = false
	if rec := doRequest(t, h, "POST", "/api/admin/packages", b, csrf, cookie); rec.Code != 201 {
		t.Fatalf("buat paket nonaktif = %d (%s)", rec.Code, rec.Body.String())
	}
	b2 := packageBody()
	b2["name"] = "Mingguan"
	b2["sort_order"] = 0
	if rec := doRequest(t, h, "POST", "/api/admin/packages", b2, csrf, cookie); rec.Code != 201 {
		t.Fatalf("buat paket aktif = %d (%s)", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, h, "GET", "/api/packages", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("GET /api/packages = %d, want 200", rec.Code)
	}
	var items []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &items)
	if len(items) != 1 || items[0]["name"] != "Mingguan" {
		t.Fatalf("katalog public = %v, want hanya Mingguan", items)
	}
	for _, key := range []string{"id", "name", "description", "price_idr", "validity_label", "profile_name"} {
		if _, ok := items[0][key]; !ok {
			t.Fatalf("kunci %s hilang dari katalog public", key)
		}
	}
}

func TestAdminPackagesCRUD(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	rec := doRequest(t, h, "POST", "/api/admin/packages", packageBody(), csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("create = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &created)
	id := created["id"].(float64)

	// Validasi harga.
	bad := packageBody()
	bad["price_idr"] = 500
	if rec := doRequest(t, h, "POST", "/api/admin/packages", bad, csrf, cookie); rec.Code != 400 {
		t.Fatalf("harga murah = %d, want 400", rec.Code)
	}
	// Wajib login.
	if rec := doRequest(t, h, "GET", "/api/admin/packages", nil, nil); rec.Code != 401 {
		t.Fatalf("tanpa sesi = %d, want 401", rec.Code)
	}
	// Wajib CSRF untuk mutasi.
	if rec := doRequest(t, h, "POST", "/api/admin/packages", packageBody(), nil, cookie); rec.Code != 403 {
		t.Fatalf("tanpa CSRF = %d, want 403", rec.Code)
	}

	// Update.
	upd := packageBody()
	upd["price_idr"] = 8000
	rec = doRequest(t, h, "PUT", "/api/admin/packages/1", upd, csrf, cookie)
	_ = id
	if rec.Code != 200 {
		t.Fatalf("update = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &got)
	if got["price_idr"] != float64(8000) {
		t.Fatalf("harga setelah update = %v, want 8000", got["price_idr"])
	}
	if rec := doRequest(t, h, "PUT", "/api/admin/packages/9999", upd, csrf, cookie); rec.Code != 400 {
		t.Fatalf("update asing = %d, want 400", rec.Code)
	}

	// List admin memuat semua.
	rec = doRequest(t, h, "GET", "/api/admin/packages", nil, nil, cookie)
	var all []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &all)
	if len(all) != 1 {
		t.Fatalf("list admin = %d, want 1", len(all))
	}

	// Hapus.
	if rec := doRequest(t, h, "DELETE", "/api/admin/packages/1", nil, csrf, cookie); rec.Code != 200 {
		t.Fatalf("delete = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	if rec := doRequest(t, h, "DELETE", "/api/admin/packages/9999", nil, csrf, cookie); rec.Code != 400 {
		t.Fatalf("hapus asing = %d, want 400", rec.Code)
	}
}

func TestAdminPackagesBuatProfileBaru(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	// Siapkan koneksi aktif agar ProfileUsecase bisa dial via gateway palsu.
	rec := doRequest(t, h, "POST", "/api/admin/connections", map[string]any{
		"name": "utama", "host": "192.168.88.1", "port": 8728,
		"username": "admin", "password": "rahasia", "is_active": true,
	}, csrf, cookie)
	if rec.Code != 201 {
		t.Fatalf("buat koneksi = %d (%s)", rec.Code, rec.Body.String())
	}

	b := packageBody()
	b["profile_name"] = "profil-baru"
	b["create_profile"] = true
	if rec := doRequest(t, h, "POST", "/api/admin/packages", b, csrf, cookie); rec.Code != 201 {
		t.Fatalf("create + profile = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	// Profile baru tampil di daftar live.
	rec = doRequest(t, h, "GET", "/api/admin/router/profiles", nil, nil, cookie)
	var profs []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &profs)
	found := false
	for _, p := range profs {
		if p["name"] == "profil-baru" {
			found = true
		}
	}
	if !found {
		t.Fatalf("profil-baru tidak ada di router: %v", profs)
	}
}

func TestAdminPackagesBuatProfileTanpaKoneksiGagal(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	b := packageBody()
	b["create_profile"] = true
	if rec := doRequest(t, h, "POST", "/api/admin/packages", b, csrf, cookie); rec.Code != 400 {
		t.Fatalf("create tanpa koneksi = %d, want 400", rec.Code)
	}
	rec := doRequest(t, h, "GET", "/api/admin/packages", nil, nil, cookie)
	var all []map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &all)
	if len(all) != 0 {
		t.Fatalf("paket tersimpan walau profile gagal: %d", len(all))
	}
}
