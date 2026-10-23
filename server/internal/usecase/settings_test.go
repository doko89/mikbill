package usecase

import (
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

func strptr(s string) *string { return &s }
func intptr(n int) *int       { return &n }

func seedSiteSettings(t *testing.T, set *fakeSettings) {
	t.Helper()
	for k, v := range map[string]string{
		domain.SettingDeployMode:       "local",
		domain.SettingSiteURL:          "http://mikbil.lan",
		domain.SettingLocalServerIP:    "10.10.10.2",
		domain.SettingHotspotLoginBase: "http://192.168.88.1/login",
	} {
		if err := set.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSettingsGetBawaan(t *testing.T) {
	uc := NewSettingsUsecase(newFakeSettings(), fakeBox{})
	v, err := uc.Get()
	if err != nil {
		t.Fatal(err)
	}
	if v.VoucherLength != 8 {
		t.Fatalf("panjang bawaan = %d, want 8", v.VoucherLength)
	}
	if v.KrisKrisSecretSet || v.KrisKrisCallbackSet {
		t.Fatalf("secret baru = %+v, want belum terisi", v)
	}
}

func TestSettingsUpdateParsial(t *testing.T) {
	set := newFakeSettings()
	seedSiteSettings(t, set)
	uc := NewSettingsUsecase(set, fakeBox{})

	v, err := uc.Update(SettingsUpdate{VoucherLength: intptr(12)})
	if err != nil {
		t.Fatal(err)
	}
	if v.VoucherLength != 12 || v.SiteURL != "http://mikbil.lan" {
		t.Fatalf("parsial = %+v", v)
	}
	if got, _, _ := set.Get(domain.SettingVoucherLength); got != "12" {
		t.Fatalf("tersimpan = %q, want 12", got)
	}
}

func TestSettingsUpdateValidasi(t *testing.T) {
	seed := func(t *testing.T) *SettingsUsecase {
		set := newFakeSettings()
		seedSiteSettings(t, set)
		return NewSettingsUsecase(set, fakeBox{})
	}
	cases := []struct {
		nama string
		in   SettingsUpdate
	}{
		{"panjang pendek", SettingsUpdate{VoucherLength: intptr(5)}},
		{"panjang jauh", SettingsUpdate{VoucherLength: intptr(64)}},
		{"mode asing", SettingsUpdate{DeployMode: strptr("awan")}},
		{"vps http", SettingsUpdate{DeployMode: strptr("vps")}},
		{"url rusak", SettingsUpdate{SiteURL: strptr("bukan-url")}},
		{"login rusak", SettingsUpdate{HotspotLoginBase: strptr("ftp://x")}},
		{"ip rusak", SettingsUpdate{LocalServerIP: strptr("999.1.1.1")}},
		{"key kosong", SettingsUpdate{KrisKrisAPIKey: strptr("  ")}},
		{"secret kosong", SettingsUpdate{KrisKrisAPISecret: strptr("")}},
		{"callback kosong", SettingsUpdate{KrisKrisCallbackSecret: strptr("")}},
	}
	for _, c := range cases {
		if _, err := seed(t).Update(c.in); err == nil {
			t.Fatalf("%s: lolos, want InputError", c.nama)
		} else if _, ok := err.(InputError); !ok {
			t.Fatalf("%s: err = %T(%v), want InputError", c.nama, err, err)
		}
	}
}

func TestSettingsUpdateKrisKrisTerenkripsi(t *testing.T) {
	set := newFakeSettings()
	seedSiteSettings(t, set)
	uc := NewSettingsUsecase(set, fakeBox{})

	v, err := uc.Update(SettingsUpdate{
		KrisKrisAPIKey:         strptr("kk-key"),
		KrisKrisAPISecret:      strptr("s3cr3t"),
		KrisKrisCallbackSecret: strptr("cb-s3cr3t"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.KrisKrisAPIKey != "kk-key" || !v.KrisKrisSecretSet || !v.KrisKrisCallbackSet {
		t.Fatalf("tampilan = %+v", v)
	}
	if got, _, _ := set.Get(domain.SettingKrisKrisAPISecret); got != "enc:s3cr3t" {
		t.Fatalf("secret tersimpan = %q, want terenkripsi", got)
	}
	if got, _, _ := set.Get(domain.SettingKrisKrisCallbackSec); got != "enc:cb-s3cr3t" {
		t.Fatalf("callback tersimpan = %q, want terenkripsi", got)
	}
}
