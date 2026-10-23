package usecase

import (
	"errors"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// ProfileCreator port pembuatan profile di router (dipenuhi ProfileUsecase).
type ProfileCreator interface {
	Create(in CreateProfileInput) (domain.HotspotProfile, error)
}

// PackageUsecase menangani CRUD paket voucher.
type PackageUsecase struct {
	pkgs     PackageRepository
	profiles ProfileCreator
}

// NewPackageUsecase membuat PackageUsecase.
func NewPackageUsecase(pkgs PackageRepository, profiles ProfileCreator) *PackageUsecase {
	return &PackageUsecase{pkgs: pkgs, profiles: profiles}
}

// PackageInput adalah masukan tambah/ubah paket.
type PackageInput struct {
	Name           string
	Description    string
	PriceIDR       int
	ValiditySecs   int
	ValidityLabel  string
	ProfileName    string
	RateLimit      string
	SessionTimeout int
	SharedUsers    int
	IsActive       bool
	SortOrder      int
	CreateProfile  bool
}

func validatePackage(in PackageInput) (PackageInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.ValidityLabel = strings.TrimSpace(in.ValidityLabel)
	in.ProfileName = strings.TrimSpace(in.ProfileName)
	in.RateLimit = strings.TrimSpace(in.RateLimit)
	switch {
	case in.Name == "":
		return in, InputError{"nama paket wajib diisi"}
	case len(in.Name) > 64:
		return in, InputError{"nama paket maksimal 64 karakter"}
	case len(in.Description) > 500:
		return in, InputError{"deskripsi maksimal 500 karakter"}
	case in.PriceIDR < 1000:
		return in, InputError{"harga paket minimal Rp1.000"}
	case in.ValiditySecs <= 0:
		return in, InputError{"masa aktif paket wajib diisi"}
	case in.ValidityLabel == "":
		return in, InputError{"label masa aktif wajib diisi"}
	case len(in.ValidityLabel) > 32:
		return in, InputError{"label masa aktif maksimal 32 karakter"}
	case in.ProfileName == "":
		return in, InputError{"nama profile wajib diisi"}
	case len(in.ProfileName) > 64:
		return in, InputError{"nama profile maksimal 64 karakter"}
	case in.SessionTimeout < 0:
		return in, InputError{"batas sesi tidak boleh negatif"}
	case in.SharedUsers < 0:
		return in, InputError{"shared users tidak boleh negatif"}
	}
	if in.SharedUsers == 0 {
		in.SharedUsers = 1
	}
	return in, nil
}

// maybeCreateProfile membuat profile di router bila diminta; gagal berarti paket batal disimpan.
func (u *PackageUsecase) maybeCreateProfile(in PackageInput) error {
	if !in.CreateProfile {
		return nil
	}
	_, err := u.profiles.Create(CreateProfileInput{
		Name: in.ProfileName, RateLimit: in.RateLimit,
		SessionTimeout: in.SessionTimeout, SharedUsers: in.SharedUsers,
	})
	return err
}

func inputToPackage(in PackageInput) domain.Package {
	return domain.Package{
		Name: in.Name, Description: in.Description, PriceIDR: in.PriceIDR,
		ValiditySecs: in.ValiditySecs, ValidityLabel: in.ValidityLabel,
		ProfileName: in.ProfileName, RateLimit: in.RateLimit,
		SessionTimeout: in.SessionTimeout, SharedUsers: in.SharedUsers,
		IsActive: in.IsActive, SortOrder: in.SortOrder,
	}
}

// List mengembalikan semua paket untuk admin.
func (u *PackageUsecase) List() ([]domain.Package, error) {
	return u.pkgs.List()
}

// ListActive mengembalikan paket aktif urut sort_order untuk katalog public.
func (u *PackageUsecase) ListActive() ([]domain.Package, error) {
	return u.pkgs.ListActive()
}

// Create menyimpan paket baru.
func (u *PackageUsecase) Create(in PackageInput) (domain.Package, error) {
	in, err := validatePackage(in)
	if err != nil {
		return domain.Package{}, err
	}
	if err := u.maybeCreateProfile(in); err != nil {
		return domain.Package{}, err
	}
	return u.pkgs.Create(inputToPackage(in))
}

// Update mengubah paket.
func (u *PackageUsecase) Update(id int64, in PackageInput) (domain.Package, error) {
	if _, err := u.pkgs.GetByID(id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Package{}, InputError{"paket tidak ditemukan"}
		}
		return domain.Package{}, err
	}
	in, err := validatePackage(in)
	if err != nil {
		return domain.Package{}, err
	}
	if err := u.maybeCreateProfile(in); err != nil {
		return domain.Package{}, err
	}
	p := inputToPackage(in)
	p.ID = id
	return u.pkgs.Update(p)
}

// Delete menghapus paket; voucher yang sudah terjual tidak ikut terhapus.
func (u *PackageUsecase) Delete(id int64) error {
	if _, err := u.pkgs.GetByID(id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return InputError{"paket tidak ditemukan"}
		}
		return err
	}
	return u.pkgs.Delete(id)
}
