package http_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testLogoPNG membuat PNG kecil valid untuk uji unggah.
func testLogoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{5, 150, 105, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// doMultipart mengirim request multipart field tunggal "logo".
func doMultipart(t *testing.T, h http.Handler, method, path string, data []byte, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("logo", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
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

func TestThemePublikBawaan(t *testing.T) {
	h := newTestRouter(t)
	rec := doRequest(t, h, "GET", "/api/theme", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("GET /api/theme = %d, want 200", rec.Code)
	}
	var data map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &data)
	if data["brand_name"] != "Mikbil" || data["brand_tagline"] != "Beli voucher WiFi mandiri" ||
		data["accent_color"] != "#059669" || data["theme_mode"] != "system" {
		t.Fatalf("tema bawaan = %v", data)
	}
	logoURL, present := data["logo_url"]
	if !present || logoURL != nil {
		t.Fatalf("logo_url bawaan = %v, want null", logoURL)
	}

	// Logo belum ada: 404 berenvelope.
	rec = doRequest(t, h, "GET", "/api/theme/logo", nil, nil)
	if rec.Code != 404 || mustEnvelope(t, rec).OK {
		t.Fatalf("logo kosong = %d, want 404 berenvelope", rec.Code)
	}
}

func TestAdminSettingsTemaValidasi(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)

	for _, b := range []map[string]any{
		{"brand_name": ""},
		{"brand_name": "   "},
		{"brand_name": strings.Repeat("x", 41)},
		{"brand_tagline": strings.Repeat("y", 81)},
		{"accent_color": "059669"},
		{"accent_color": "#zzz"},
		{"accent_color": "#05966900"},
		{"theme_mode": "amoled"},
	} {
		if rec := doRequest(t, h, "PUT", "/api/admin/settings", b, csrf, cookie); rec.Code != 400 {
			t.Fatalf("%v = %d, want 400", b, rec.Code)
		}
	}

	// Nilai valid tersimpan dan terbaca di GET admin + publik.
	rec := doRequest(t, h, "PUT", "/api/admin/settings", map[string]any{
		"brand_name": "OpiWifi", "brand_tagline": "Internet cepat",
		"accent_color": "#1D4ED8", "theme_mode": "dark",
	}, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("PUT tema = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var simpan map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &simpan)
	if simpan["brand_name"] != "OpiWifi" || simpan["theme_mode"] != "dark" {
		t.Fatalf("tersimpan = %v", simpan)
	}
	rec = doRequest(t, h, "GET", "/api/theme", nil, nil)
	var pub map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &pub)
	if pub["brand_name"] != "OpiWifi" || pub["accent_color"] != "#1D4ED8" {
		t.Fatalf("publik = %v", pub)
	}
}

func TestThemeLogoRoundTrip(t *testing.T) {
	h := newTestRouter(t)
	cookie, csrf := loginAdmin(t, h)
	logo := testLogoPNG(t)

	// Tanpa sesi: 401; tanpa CSRF: 403.
	if rec := doMultipart(t, h, "PUT", "/api/admin/theme/logo", logo, nil); rec.Code != 401 {
		t.Fatalf("tanpa sesi = %d, want 401", rec.Code)
	}
	if rec := doMultipart(t, h, "PUT", "/api/admin/theme/logo", logo, nil, cookie); rec.Code != 403 {
		t.Fatalf("tanpa CSRF = %d, want 403", rec.Code)
	}
	// Bukan gambar ditolak.
	if rec := doMultipart(t, h, "PUT", "/api/admin/theme/logo", []byte("bukan gambar"), csrf, cookie); rec.Code != 400 {
		t.Fatalf("teks = %d, want 400", rec.Code)
	}

	// Unggah PNG valid.
	rec := doMultipart(t, h, "PUT", "/api/admin/theme/logo", logo, csrf, cookie)
	if rec.Code != 200 {
		t.Fatalf("unggah = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var up map[string]string
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &up)
	logoURL := up["logo_url"]
	if !strings.HasPrefix(logoURL, "/api/theme/logo?v=") || len(logoURL) != len("/api/theme/logo?v=")+8 {
		t.Fatalf("logo_url = %q", logoURL)
	}

	// GET publik: bytes sama + header cache + ETag.
	rec = doRequest(t, h, "GET", "/api/theme/logo", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("GET logo = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), logo) {
		t.Fatal("bytes logo berubah setelah round-trip")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %q", cc)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag kosong")
	}
	rec = doRequest(t, h, "GET", "/api/theme/logo", nil,
		map[string]string{"If-None-Match": etag})
	if rec.Code != 304 {
		t.Fatalf("If-None-Match = %d, want 304", rec.Code)
	}

	// Tema publik memuat logo_url versi sama.
	rec = doRequest(t, h, "GET", "/api/theme", nil, nil)
	var pub map[string]any
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &pub)
	if pub["logo_url"] != logoURL {
		t.Fatalf("logo_url publik = %v, want %s", pub["logo_url"], logoURL)
	}

	// Hapus tanpa CSRF ditolak; dengan CSRF 200.
	if rec := doRequest(t, h, "DELETE", "/api/admin/theme/logo", nil, nil, cookie); rec.Code != 403 {
		t.Fatalf("hapus tanpa CSRF = %d, want 403", rec.Code)
	}
	if rec := doRequest(t, h, "DELETE", "/api/admin/theme/logo", nil, csrf, cookie); rec.Code != 200 {
		t.Fatalf("hapus = %d, want 200", rec.Code)
	}
	if rec := doRequest(t, h, "GET", "/api/theme/logo", nil, nil); rec.Code != 404 {
		t.Fatalf("logo setelah hapus = %d, want 404", rec.Code)
	}
	rec = doRequest(t, h, "GET", "/api/theme", nil, nil)
	_ = json.Unmarshal(mustEnvelope(t, rec).Data, &pub)
	if pub["logo_url"] != nil {
		t.Fatalf("logo_url setelah hapus = %v, want null", pub["logo_url"])
	}
}
