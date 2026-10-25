package usecase

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

// fakeThemeAssets adalah ThemeAssetRepository memori untuk test.
type fakeThemeAssets struct {
	rows map[string]domain.ThemeAsset
}

func newFakeThemeAssets() *fakeThemeAssets {
	return &fakeThemeAssets{rows: map[string]domain.ThemeAsset{}}
}

func (f *fakeThemeAssets) Get(key string) (domain.ThemeAsset, bool, error) {
	a, ok := f.rows[key]
	return a, ok, nil
}

func (f *fakeThemeAssets) Set(key, mime string, data []byte) error {
	f.rows[key] = domain.ThemeAsset{Key: key, MIME: mime, Data: data}
	return nil
}

func (f *fakeThemeAssets) Delete(key string) error {
	delete(f.rows, key)
	return nil
}

// testPNG mengenkode PNG solid berukuran w×h.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{5, 150, 105, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSettingsGetTemaBawaan(t *testing.T) {
	uc := NewSettingsUsecase(newFakeSettings(), fakeBox{})
	v, err := uc.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v.BrandName != "Mikbil" || v.BrandTagline != "Beli voucher WiFi mandiri" ||
		v.AccentColor != "#059669" || v.ThemeMode != "system" {
		t.Fatalf("tema bawaan = %+v", v)
	}
}

func TestSettingsUpdateTemaValidasi(t *testing.T) {
	seed := func(t *testing.T) *SettingsUsecase {
		set := newFakeSettings()
		seedSiteSettings(t, set)
		return NewSettingsUsecase(set, fakeBox{})
	}
	long40 := strings.Repeat("a", 41)
	long80 := strings.Repeat("b", 81)
	cases := []struct {
		nama string
		in   SettingsUpdate
	}{
		{"nama kosong", SettingsUpdate{BrandName: strptr("")}},
		{"nama spasi", SettingsUpdate{BrandName: strptr("   ")}},
		{"nama panjang", SettingsUpdate{BrandName: strptr(long40)}},
		{"tagline panjang", SettingsUpdate{BrandTagline: strptr(long80)}},
		{"hex tanpa pagar", SettingsUpdate{AccentColor: strptr("059669")}},
		{"hex pendek", SettingsUpdate{AccentColor: strptr("#fff")}},
		{"hex bukan digit", SettingsUpdate{AccentColor: strptr("#zzzzzz")}},
		{"hex panjang", SettingsUpdate{AccentColor: strptr("#05966900")}},
		{"mode asing", SettingsUpdate{ThemeMode: strptr("amoled")}},
		{"mode kosong", SettingsUpdate{ThemeMode: strptr("")}},
	}
	for _, c := range cases {
		if _, err := seed(t).Update(c.in); err == nil {
			t.Fatalf("%s: lolos, want InputError", c.nama)
		} else if _, ok := err.(InputError); !ok {
			t.Fatalf("%s: err = %T(%v), want InputError", c.nama, err, err)
		}
	}
}

func TestSettingsUpdateTemaTersimpan(t *testing.T) {
	set := newFakeSettings()
	seedSiteSettings(t, set)
	uc := NewSettingsUsecase(set, fakeBox{})

	v, err := uc.Update(SettingsUpdate{
		BrandName:    strptr("  OpiWifi  "),
		BrandTagline: strptr("Internet cepat"),
		AccentColor:  strptr("#1D4ED8"),
		ThemeMode:    strptr("dark"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.BrandName != "OpiWifi" || v.BrandTagline != "Internet cepat" ||
		v.AccentColor != "#1D4ED8" || v.ThemeMode != "dark" {
		t.Fatalf("tema = %+v", v)
	}
	if got, _, _ := set.Get(domain.SettingBrandName); got != "OpiWifi" {
		t.Fatalf("brand tersimpan = %q", got)
	}
	// Tagline boleh dikosongkan; Get lalu menampilkan bawaan.
	if _, err := uc.Update(SettingsUpdate{BrandTagline: strptr("")}); err != nil {
		t.Fatal(err)
	}
	if v, _ := uc.Get(); v.BrandTagline != domain.DefaultBrandTagline {
		t.Fatalf("tagline kosong = %q, want bawaan", v.BrandTagline)
	}
}

func TestValidateLogo(t *testing.T) {
	ok := testPNG(t, 8, 8)
	if err := ValidateLogo("image/png", ok); err != nil {
		t.Fatalf("png valid = %v", err)
	}
	cases := []struct {
		nama string
		mime string
		data []byte
	}{
		{"bukan gambar", "text/plain", []byte("halo")},
		{"gif ditolak", "image/gif", []byte("GIF89a")},
		{"terlalu besar", "image/png", bytes.Repeat([]byte{0}, 512*1024+1)},
		{"dimensi lebar", "image/png", testPNG(t, 1025, 1)},
		{"dimensi tinggi", "image/png", testPNG(t, 1, 1025)},
		{"png rusak", "image/png", []byte{0x89, 'P', 'N', 'G', 0, 1, 2}},
	}
	for _, c := range cases {
		if err := ValidateLogo(c.mime, c.data); err == nil {
			t.Fatalf("%s: lolos, want InputError", c.nama)
		} else if _, ok := err.(InputError); !ok {
			t.Fatalf("%s: err = %T(%v), want InputError", c.nama, err, err)
		}
	}
}

func TestThemeUsecaseLogoRoundTrip(t *testing.T) {
	uc := NewThemeUsecase(newFakeSettings(), newFakeThemeAssets())

	def, err := uc.Get()
	if err != nil {
		t.Fatal(err)
	}
	if def.BrandName != "Mikbil" || def.LogoVersion != "" {
		t.Fatalf("awal = %+v", def)
	}
	if _, found, _ := uc.GetLogo(); found {
		t.Fatal("logo ada sebelum diunggah")
	}

	data := testPNG(t, 16, 16)
	saved, err := uc.SaveLogo("image/png", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.LogoVersion) != 8 {
		t.Fatalf("versi = %q, want 8 heks", saved.LogoVersion)
	}
	got, found, err := uc.GetLogo()
	if err != nil || !found {
		t.Fatalf("ambil = found %v err %v", found, err)
	}
	if !bytes.Equal(got.LogoData, data) || got.LogoMIME != "image/png" {
		t.Fatal("bytes/mime logo berubah setelah round-trip")
	}
	if got.LogoVersion != saved.LogoVersion {
		t.Fatal("versi logo tidak konsisten")
	}

	if err := uc.DeleteLogo(); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := uc.GetLogo(); found {
		t.Fatal("logo masih ada setelah hapus")
	}
	if v, _ := uc.Get(); v.LogoVersion != "" {
		t.Fatalf("versi setelah hapus = %q", v.LogoVersion)
	}
	// Hapus ganda bukan error.
	if err := uc.DeleteLogo(); err != nil {
		t.Fatalf("hapus ganda = %v", err)
	}
}
