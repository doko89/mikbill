package usecase

import (
	"errors"
	"sync"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

// fakePackages adalah PackageRepository memori untuk test.
type fakePackages struct {
	mu     sync.Mutex
	rows   map[int64]domain.Package
	nextID int64
}

func newFakePackages() *fakePackages {
	return &fakePackages{rows: map[int64]domain.Package{}, nextID: 1}
}

func (f *fakePackages) List() ([]domain.Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Package{}
	for _, p := range f.rows {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakePackages) ListActive() ([]domain.Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Package{}
	for _, p := range f.rows {
		if p.IsActive {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePackages) GetByID(id int64) (domain.Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.rows[id]
	if !ok {
		return domain.Package{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakePackages) Create(p domain.Package) (domain.Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p.ID = f.nextID
	f.nextID++
	f.rows[p.ID] = p
	return p, nil
}

func (f *fakePackages) Update(p domain.Package) (domain.Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[p.ID]; !ok {
		return domain.Package{}, domain.ErrNotFound
	}
	f.rows[p.ID] = p
	return p, nil
}

func (f *fakePackages) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

// fakeProfileCreator meniru pembuatan profile di router.
type fakeProfileCreator struct {
	mu       sync.Mutex
	created  []string
	failNext error
}

func (f *fakeProfileCreator) Create(in CreateProfileInput) (domain.HotspotProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return domain.HotspotProfile{}, err
	}
	f.created = append(f.created, in.Name)
	return domain.HotspotProfile{Name: in.Name}, nil
}

func validPackageInput() PackageInput {
	return PackageInput{
		Name: "Harian", Description: "Internet 12 jam", PriceIDR: 5000,
		ValiditySecs: 43200, ValidityLabel: "12 Jam", ProfileName: "harian",
		RateLimit: "5M/10M", SessionTimeout: 0, SharedUsers: 1,
		IsActive: true, SortOrder: 0,
	}
}

func TestPackageCreateValidasiHarga(t *testing.T) {
	uc := NewPackageUsecase(newFakePackages(), &fakeProfileCreator{})
	for _, price := range []int{0, 999, -100} {
		in := validPackageInput()
		in.PriceIDR = price
		if _, err := uc.Create(in); err == nil {
			t.Fatalf("harga %d seharusnya ditolak", price)
		}
	}
	in := validPackageInput()
	in.PriceIDR = 1000
	if _, err := uc.Create(in); err != nil {
		t.Fatalf("harga 1000 seharusnya lolos: %v", err)
	}
}

func TestPackageCreateWajib(t *testing.T) {
	uc := NewPackageUsecase(newFakePackages(), &fakeProfileCreator{})
	bad := validPackageInput()
	bad.Name = ""
	if _, err := uc.Create(bad); err == nil {
		t.Fatal("nama kosong seharusnya ditolak")
	}
	bad = validPackageInput()
	bad.ProfileName = ""
	if _, err := uc.Create(bad); err == nil {
		t.Fatal("profile kosong seharusnya ditolak")
	}
	bad = validPackageInput()
	bad.ValiditySecs = 0
	if _, err := uc.Create(bad); err == nil {
		t.Fatal("validity 0 seharusnya ditolak")
	}
}

func TestPackageCreateProfileGagalBatalSimpan(t *testing.T) {
	pkgs := newFakePackages()
	prof := &fakeProfileCreator{failNext: InputError{"gagal membuat profile di router: dial gagal"}}
	uc := NewPackageUsecase(pkgs, prof)
	in := validPackageInput()
	in.CreateProfile = true
	if _, err := uc.Create(in); err == nil {
		t.Fatal("seharusnya gagal saat profile gagal dibuat")
	}
	list, _ := pkgs.List()
	if len(list) != 0 {
		t.Fatalf("paket ikut tersimpan walau profile gagal: %d baris", len(list))
	}
}

func TestPackageCreateProfileSukses(t *testing.T) {
	pkgs := newFakePackages()
	prof := &fakeProfileCreator{}
	uc := NewPackageUsecase(pkgs, prof)
	in := validPackageInput()
	in.CreateProfile = true
	p, err := uc.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || len(prof.created) != 1 || prof.created[0] != "harian" {
		t.Fatalf("profile tidak dibuat dulu: %+v", prof.created)
	}
}

func TestPackageUpdateHapus(t *testing.T) {
	pkgs := newFakePackages()
	uc := NewPackageUsecase(pkgs, &fakeProfileCreator{})
	p, err := uc.Create(validPackageInput())
	if err != nil {
		t.Fatal(err)
	}
	in := validPackageInput()
	in.PriceIDR = 999
	if _, err := uc.Update(p.ID, in); err == nil {
		t.Fatal("update harga murah seharusnya ditolak")
	}
	list, _ := pkgs.List()
	if list[0].PriceIDR != 5000 {
		t.Fatal("paket berubah walau update ditolak")
	}
	if _, err := uc.Update(999, validPackageInput()); err == nil {
		t.Fatal("update id asing seharusnya gagal")
	}
	if err := uc.Delete(999); err == nil {
		t.Fatal("hapus id asing seharusnya gagal")
	}
	if err := uc.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pkgs.GetByID(p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("paket seharusnya sudah terhapus")
	}
}
