package sqlite

import (
	"errors"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

func TestAllowlistRepoCRUD(t *testing.T) {
	db := openMigrated(t)
	r := NewAllowlistRepo(db)

	a, err := r.Create("api.dana.id", true)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 || a.DstHost != "api.dana.id" || !a.Enabled {
		t.Fatalf("allowlist tersimpan tidak lengkap: %+v", a)
	}
	if _, err := r.Create("api.dana.id", true); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("duplikat host = %v, want ErrAlreadyExists", err)
	}
	got, err := r.GetByID(a.ID)
	if err != nil || got.DstHost != "api.dana.id" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}
	if _, err := r.GetByID(9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("id asing = %v, want ErrNotFound", err)
	}

	a.DstHost = "api.gopay.co.id"
	a.Enabled = false
	upd, err := r.Update(a)
	if err != nil {
		t.Fatal(err)
	}
	if upd.DstHost != "api.gopay.co.id" || upd.Enabled {
		t.Fatalf("update gagal: %+v", upd)
	}
	if _, err := r.Update(domain.WalledGardenAllow{ID: 9999}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update asing = %v, want ErrNotFound", err)
	}

	if _, err := r.Create("m.bankmandiri.co.id", true); err != nil {
		t.Fatal(err)
	}
	list, err := r.List()
	if err != nil || len(list) != 2 || list[0].DstHost != "api.gopay.co.id" {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if err := r.Delete(9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("hapus asing = %v, want ErrNotFound", err)
	}
	if err := r.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetByID(a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("host seharusnya sudah terhapus")
	}
}
