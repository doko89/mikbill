package usecase

import (
	"errors"
	"testing"
)

func newSetupForKrisKrisTest() (*SetupUsecase, *fakeSettings) {
	set := newFakeSettings()
	return NewSetupUsecase(nil, nil, set, nil, nil, nil, nil), set
}

func TestKrisKrisBuatBacaBatalkan(t *testing.T) {
	uc, set := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{createOut: InvoiceInfo{ID: "inv_tes", Status: "pending"}}
	uc.SetInvoiceBuilder(func(key, secret string) (InvoiceClient, error) {
		if key != "k" || secret != "s" {
			t.Fatalf("builder = %q/%q, want k/s", key, secret)
		}
		return inv, nil
	})
	if err := uc.TestKrisKris("k", "s"); err != nil {
		t.Fatalf("tes = %v, want sukses", err)
	}
	if inv.createCalls != 1 {
		t.Fatalf("create dipanggil %d kali, want 1", inv.createCalls)
	}
	if len(inv.cancelled) != 1 || inv.cancelled[0] != "inv_tes" {
		t.Fatalf("cancelled = %v, want [inv_tes]", inv.cancelled)
	}
	if _, found, _ := set.Get("kriskris_api_key"); found {
		t.Fatal("tes eksplisit tidak boleh menyimpan kredensial")
	}
}

func TestKrisKrisCreate401(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{createErrs: []error{InvoiceError{Msg: "salah", Status: 401}}}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	err := uc.TestKrisKris("k", "salah")
	var input InputError
	if !errors.As(err, &input) || input.Msg != "API key atau secret salah (401)" {
		t.Fatalf("tes 401 = %v, want InputError 401", err)
	}
	if len(inv.cancelled) != 0 {
		t.Fatalf("cancelled = %v, want kosong saat create gagal", inv.cancelled)
	}
}

func TestKrisKrisCreate500(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{createErrs: []error{InvoiceError{Msg: "rusak", Status: 500}}}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	err := uc.TestKrisKris("k", "s")
	var input InputError
	if !errors.As(err, &input) || input.Msg != "tes koneksi gagal (HTTP 500): rusak" {
		t.Fatalf("tes 500 = %v, want pesan memuat HTTP 500", err)
	}
}

func TestKrisKrisGetGagalTetapBatal(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{
		createOut: InvoiceInfo{ID: "inv_tes", Status: "pending"},
		getErr:    InvoiceError{Msg: "rusak", Status: 500},
	}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	if err := uc.TestKrisKris("k", "s"); err == nil {
		t.Fatal("get gagal lolos, want error")
	}
	if len(inv.cancelled) != 1 {
		t.Fatalf("cancelled = %v, want tetap dibatalkan", inv.cancelled)
	}
}

func TestKrisKrisEksplisitParsial(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) {
		t.Fatal("builder tidak boleh dipanggil untuk kredensial parsial")
		return nil, nil
	})
	if err := uc.TestKrisKris("k", ""); err == nil {
		t.Fatal("secret kosong lolos, want InputError")
	}
}

func TestKrisKrisTersimpan(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	if err := uc.TestKrisKris("", ""); err == nil {
		t.Fatal("tanpa factory lolos, want NotImplementedError")
	} else {
		var notimpl NotImplementedError
		if !errors.As(err, &notimpl) {
			t.Fatalf("tanpa factory = %T(%v)", err, err)
		}
	}
	inv := &fakeInvoiceClient{createOut: InvoiceInfo{ID: "inv_tes", Status: "pending"}}
	uc.SetInvoiceFactory(func() (InvoiceClient, error) { return inv, nil })
	if err := uc.TestKrisKris("", ""); err != nil {
		t.Fatalf("tes tersimpan = %v, want sukses", err)
	}
	if len(inv.cancelled) != 1 {
		t.Fatalf("cancelled = %v, want 1", inv.cancelled)
	}
}
