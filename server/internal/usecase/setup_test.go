package usecase

import (
	"errors"
	"testing"
)

func newSetupForKrisKrisTest() (*SetupUsecase, *fakeSettings) {
	set := newFakeSettings()
	return NewSetupUsecase(nil, nil, set, nil, nil, nil, nil), set
}

func TestKrisKrisEksplisitTanpaSimpan(t *testing.T) {
	uc, set := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{}
	uc.SetInvoiceBuilder(func(key, secret string) (InvoiceClient, error) {
		if key != "k" || secret != "s" {
			t.Fatalf("builder = %q/%q, want k/s", key, secret)
		}
		return inv, nil
	})
	if err := uc.TestKrisKris("k", "s"); err != nil {
		t.Fatalf("tes eksplisit = %v, want sukses", err)
	}
	if len(inv.batchIDs) != 1 || len(inv.batchIDs[0]) != 1 {
		t.Fatalf("batch dipanggil = %v, want 1x id dummy", inv.batchIDs)
	}
	if _, found, _ := set.Get("kriskris_api_key"); found {
		t.Fatal("tes eksplisit tidak boleh menyimpan kredensial")
	}
}

func TestKrisKrisEksplisit401(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{batchErr: InvoiceError{Msg: "salah", Status: 401}}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	err := uc.TestKrisKris("k", "salah")
	var input InputError
	if !errors.As(err, &input) || input.Msg != "API key atau secret salah (401)" {
		t.Fatalf("tes 401 = %v, want InputError 401", err)
	}
}

func TestKrisKrisEksplisitServerError(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	inv := &fakeInvoiceClient{
		batchErr: InvoiceError{Msg: "Terjadi kesalahan pada server.", Status: 500},
		getErr:   InvoiceError{Msg: "Terjadi kesalahan pada server.", Status: 500},
	}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	err := uc.TestKrisKris("k", "s")
	var input InputError
	if !errors.As(err, &input) || input.Msg != "tes koneksi gagal (HTTP 500): Terjadi kesalahan pada server." {
		t.Fatalf("tes 500 = %v, want pesan memuat HTTP 500", err)
	}
}

func TestKrisKrisFallbackGetSaatBatch500(t *testing.T) {
	uc, _ := newSetupForKrisKrisTest()
	// Batch 500 + GET 404 = kredensial valid (auth lolos, id asing).
	inv := &fakeInvoiceClient{
		batchErr: InvoiceError{Msg: "rusak", Status: 500},
		getErr:   InvoiceError{Msg: "bukan milik key", Status: 404},
	}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv, nil })
	if err := uc.TestKrisKris("k", "s"); err != nil {
		t.Fatalf("fallback 404 = %v, want sukses", err)
	}
	// Batch 500 + GET 200 = kredensial valid.
	inv2 := &fakeInvoiceClient{batchErr: InvoiceError{Msg: "rusak", Status: 500}}
	uc.SetInvoiceBuilder(func(_, _ string) (InvoiceClient, error) { return inv2, nil })
	if err := uc.TestKrisKris("k", "s"); err != nil {
		t.Fatalf("fallback 200 = %v, want sukses", err)
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
	inv := &fakeInvoiceClient{}
	uc.SetInvoiceFactory(func() (InvoiceClient, error) { return inv, nil })
	if err := uc.TestKrisKris("", ""); err != nil {
		t.Fatalf("tes tersimpan = %v, want sukses", err)
	}
	if len(inv.batchIDs) != 1 {
		t.Fatalf("batch tersimpan dipanggil %d kali, want 1", len(inv.batchIDs))
	}
}
