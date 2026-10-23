package usecase

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// SessionTTL adalah masa berlaku sesi login admin.
const SessionTTL = 24 * time.Hour

// TokenBytes adalah panjang token sesi mentah (byte).
const TokenBytes = 32

// AuthUsecase menangani login/logout/sesi admin.
type AuthUsecase struct {
	admins   AdminRepository
	sessions SessionRepository
	set      SettingsRepository
	hash     PasswordHasher
	random   func(n int) ([]byte, error)
	now      func() time.Time
}

// NewAuthUsecase membuat AuthUsecase.
func NewAuthUsecase(admins AdminRepository, sessions SessionRepository, set SettingsRepository, hash PasswordHasher, random func(n int) ([]byte, error)) *AuthUsecase {
	return &AuthUsecase{admins: admins, sessions: sessions, set: set, hash: hash, random: random, now: time.Now}
}

// dummyHash dipakai agar timing login gagal seragam (hindari enumerasi user).
const dummyHash = "$2a$12$7fohYGTT6AIkL06BhVi7WuXBnMeohvJqpfPUuvUwe1mG6aH3guwzS"

// Login memverifikasi kredensial dan membuat sesi baru.
// Mengembalikan token mentah (untuk cookie), admin, dan sesi.
func (u *AuthUsecase) Login(username, password string) (token string, admin domain.Admin, sess domain.Session, err error) {
	username = strings.TrimSpace(username)
	admin, err = u.admins.GetByUsername(username)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", domain.Admin{}, domain.Session{}, err
	}
	hash := dummyHash
	if err == nil {
		hash = admin.PasswordHash
	}
	if !u.hash.Check(hash, password) || errors.Is(err, domain.ErrNotFound) {
		return "", domain.Admin{}, domain.Session{}, UnauthorizedError{"username atau password salah"}
	}
	raw, err := u.random(TokenBytes)
	if err != nil {
		return "", domain.Admin{}, domain.Session{}, err
	}
	csrfRaw, err := u.random(TokenBytes)
	if err != nil {
		return "", domain.Admin{}, domain.Session{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	sess = domain.Session{
		ID:        HashToken(token),
		AdminID:   admin.ID,
		CSRFToken: hex.EncodeToString(csrfRaw),
		ExpiresAt: u.now().Add(SessionTTL),
	}
	if err := u.sessions.Create(sess); err != nil {
		return "", domain.Admin{}, domain.Session{}, err
	}
	admin.PasswordHash = ""
	return token, admin, sess, nil
}

// Authenticate memvalidasi token sesi dan mengembalikan admin + sesi.
func (u *AuthUsecase) Authenticate(token string) (domain.Admin, domain.Session, error) {
	sess, err := u.sessions.Get(HashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Admin{}, domain.Session{}, UnauthorizedError{"sesi tidak valid, silakan login ulang"}
		}
		return domain.Admin{}, domain.Session{}, err
	}
	if sess.Expired(u.now()) {
		_ = u.sessions.Delete(sess.ID)
		return domain.Admin{}, domain.Session{}, UnauthorizedError{"sesi kedaluwarsa, silakan login ulang"}
	}
	admin, err := u.admins.GetByID(sess.AdminID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Admin{}, domain.Session{}, UnauthorizedError{"akun tidak ditemukan"}
		}
		return domain.Admin{}, domain.Session{}, err
	}
	admin.PasswordHash = ""
	return admin, sess, nil
}

// Logout mencabut sesi.
func (u *AuthUsecase) Logout(token string) error {
	_ = u.sessions.DeleteExpired()
	return u.sessions.Delete(HashToken(token))
}

// SecureCookie melaporkan apakah cookie sesi harus bertanda Secure.
// True kecuali site_url memakai http:// (mode server lokal).
func (u *AuthUsecase) SecureCookie() bool {
	v, found, err := u.set.Get(domain.SettingSiteURL)
	if err != nil || !found {
		return true
	}
	return !strings.HasPrefix(v, "http://")
}

// HashToken menghitung id sesi dari token mentah cookie.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
