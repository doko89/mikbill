package usecase

import (
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// ProfileUsecase menangani daftar dan pembuatan user profile di router (live).
type ProfileUsecase struct {
	conns *ConnectionUsecase
}

// NewProfileUsecase membuat ProfileUsecase.
func NewProfileUsecase(conns *ConnectionUsecase) *ProfileUsecase {
	return &ProfileUsecase{conns: conns}
}

// List membaca daftar user profile langsung dari router aktif.
func (u *ProfileUsecase) List() ([]domain.HotspotProfile, error) {
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	return sess.ListUserProfiles()
}

// CreateProfileInput adalah masukan pembuatan profile.
type CreateProfileInput struct {
	Name           string
	RateLimit      string
	SessionTimeout int
	SharedUsers    int
}

// Create membuat user profile baru di router aktif.
func (u *ProfileUsecase) Create(in CreateProfileInput) (domain.HotspotProfile, error) {
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		return domain.HotspotProfile{}, InputError{"nama profile wajib diisi"}
	case len(name) > 64:
		return domain.HotspotProfile{}, InputError{"nama profile maksimal 64 karakter"}
	case in.SessionTimeout < 0:
		return domain.HotspotProfile{}, InputError{"batas sesi tidak boleh negatif"}
	case in.SharedUsers < 0:
		return domain.HotspotProfile{}, InputError{"shared users tidak boleh negatif"}
	}
	shared := in.SharedUsers
	if shared == 0 {
		shared = 1
	}
	p := domain.HotspotProfile{
		Name:           name,
		RateLimit:      strings.TrimSpace(in.RateLimit),
		SessionTimeout: in.SessionTimeout,
		SharedUsers:    shared,
	}
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return domain.HotspotProfile{}, err
	}
	defer sess.Close()
	if err := sess.CreateUserProfile(p); err != nil {
		return domain.HotspotProfile{}, InputError{"gagal membuat profile di router: " + shortErr(err)}
	}
	return p, nil
}
