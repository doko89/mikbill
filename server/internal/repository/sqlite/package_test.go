package sqlite

import (
	"errors"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

func samplePackage(name string, active bool, sort int) domain.Package {
	return domain.Package{
		Name: name, Description: "desc " + name, PriceIDR: 5000,
		ValiditySecs: 43200, ValidityLabel: "12 Jam", ProfileName: "prof-" + name,
		RateLimit: "5M/10M", SessionTimeout: 0, SharedUsers: 1,
		IsActive: active, SortOrder: sort,
	}
}

func TestPackageRepoCRUD(t *testing.T) {
	db := openMigrated(t)
	r := NewPackageRepo(db)

	p, err := r.Create(samplePackage("harian", true, 2))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatalf("paket tersimpan tidak lengkap: %+v", p)
	}
	got, err := r.GetByID(p.ID)
	if err != nil || got.Name != "harian" || got.RateLimit != "5M/10M" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}
	if _, err := r.GetByID(9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("id asing = %v, want ErrNotFound", err)
	}

	p.PriceIDR = 7000
	p.IsActive = false
	upd, err := r.Update(p)
	if err != nil {
		t.Fatal(err)
	}
	if upd.PriceIDR != 7000 || upd.IsActive {
		t.Fatalf("update gagal: %+v", upd)
	}
	if _, err := r.Update(domain.Package{ID: 9999}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update asing = %v, want ErrNotFound", err)
	}

	if err := r.Delete(9999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("hapus asing = %v, want ErrNotFound", err)
	}
	if err := r.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetByID(p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("paket seharusnya sudah terhapus")
	}
}

func TestPackageRepoListActiveUrut(t *testing.T) {
	db := openMigrated(t)
	r := NewPackageRepo(db)
	if _, err := r.Create(samplePackage("b", true, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(samplePackage("a", true, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(samplePackage("mati", false, 0)); err != nil {
		t.Fatal(err)
	}
	active, err := r.ListActive()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].Name != "a" || active[1].Name != "b" {
		t.Fatalf("ListActive salah urut/isi: %+v", active)
	}
	all, err := r.List()
	if err != nil || len(all) != 3 {
		t.Fatalf("List = %d, %v", len(all), err)
	}
}
