package usecase

import (
	"errors"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// ConnectionUsecase menangani CRUD koneksi router dan uji koneksi.
type ConnectionUsecase struct {
	conns ConnectionRepository
	box   SecretBox
	gw    RouterGateway
}

// NewConnectionUsecase membuat ConnectionUsecase.
func NewConnectionUsecase(conns ConnectionRepository, box SecretBox, gw RouterGateway) *ConnectionUsecase {
	return &ConnectionUsecase{conns: conns, box: box, gw: gw}
}

// ConnectionInput adalah masukan tambah/ubah koneksi.
type ConnectionInput struct {
	Name     string
	Host     string
	Port     int
	Username string
	Password string // kosong saat ubah = pertahankan password lama
	UseTLS   bool
	IsActive bool
}

// TestResult adalah hasil uji koneksi router.
type TestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	BoardName string `json:"board_name,omitempty"`
	Version   string `json:"version,omitempty"`
}

func defaultPort(useTLS bool) int {
	if useTLS {
		return 8729
	}
	return 8728
}

func validateConnection(in ConnectionInput, needPassword bool) (ConnectionInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	if in.Name == "" {
		in.Name = "default"
	}
	if in.Port == 0 {
		in.Port = defaultPort(in.UseTLS)
	}
	switch {
	case in.Host == "":
		return in, InputError{"host router wajib diisi"}
	case in.Port < 1 || in.Port > 65535:
		return in, InputError{"port router 1 sampai 65535"}
	case in.Username == "":
		return in, InputError{"username router wajib diisi"}
	case needPassword && in.Password == "":
		return in, InputError{"password router wajib diisi"}
	}
	return in, nil
}

// List mengembalikan semua koneksi (tanpa password).
func (u *ConnectionUsecase) List() ([]domain.Connection, error) {
	cs, err := u.conns.List()
	if err != nil {
		return nil, err
	}
	for i := range cs {
		cs[i].EncPassword = ""
	}
	return cs, nil
}

// Get mengambil satu koneksi (tanpa password).
func (u *ConnectionUsecase) Get(id int64) (domain.Connection, error) {
	c, err := u.conns.GetByID(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Connection{}, InputError{"koneksi tidak ditemukan"}
		}
		return domain.Connection{}, err
	}
	c.EncPassword = ""
	return c, nil
}

// Create menyimpan koneksi baru (password dienkripsi).
func (u *ConnectionUsecase) Create(in ConnectionInput) (domain.Connection, error) {
	in, err := validateConnection(in, true)
	if err != nil {
		return domain.Connection{}, err
	}
	enc, err := u.box.Seal(in.Password)
	if err != nil {
		return domain.Connection{}, err
	}
	c, err := u.conns.Create(domain.Connection{
		Name: in.Name, Host: in.Host, Port: in.Port,
		Username: in.Username, EncPassword: enc,
		UseTLS: in.UseTLS, IsActive: in.IsActive,
	})
	if err != nil {
		return domain.Connection{}, err
	}
	c.EncPassword = ""
	return c, nil
}

// Update mengubah koneksi; password kosong mempertahankan yang lama.
func (u *ConnectionUsecase) Update(id int64, in ConnectionInput) (domain.Connection, error) {
	old, err := u.conns.GetByID(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Connection{}, InputError{"koneksi tidak ditemukan"}
		}
		return domain.Connection{}, err
	}
	in, err = validateConnection(in, false)
	if err != nil {
		return domain.Connection{}, err
	}
	enc := old.EncPassword
	if in.Password != "" {
		enc, err = u.box.Seal(in.Password)
		if err != nil {
			return domain.Connection{}, err
		}
	}
	c, err := u.conns.Update(domain.Connection{
		ID: id, Name: in.Name, Host: in.Host, Port: in.Port,
		Username: in.Username, EncPassword: enc,
		UseTLS: in.UseTLS, IsActive: in.IsActive,
	})
	if err != nil {
		return domain.Connection{}, err
	}
	c.EncPassword = ""
	return c, nil
}

// Delete menghapus koneksi.
func (u *ConnectionUsecase) Delete(id int64) error {
	if _, err := u.conns.GetByID(id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return InputError{"koneksi tidak ditemukan"}
		}
		return err
	}
	return u.conns.Delete(id)
}

// dialParams membuka password koneksi tersimpan menjadi parameter dial.
func (u *ConnectionUsecase) dialParams(c domain.Connection) (RouterDialParams, error) {
	pw, err := u.box.Open(c.EncPassword)
	if err != nil {
		return RouterDialParams{}, err
	}
	return RouterDialParams{
		Host: c.Host, Port: c.Port, Username: c.Username,
		Password: pw, UseTLS: c.UseTLS,
	}, nil
}

// dialAndTest dial + login + /system/resource/print.
func (u *ConnectionUsecase) dialAndTest(p RouterDialParams) TestResult {
	sess, err := u.gw.Dial(p)
	if err != nil {
		return TestResult{Success: false, Message: "gagal terhubung: " + shortErr(err)}
	}
	defer sess.Close()
	res, err := sess.Test()
	if err != nil {
		return TestResult{Success: false, Message: "login gagal atau router menolak: " + shortErr(err)}
	}
	msg := "koneksi router berhasil"
	if res.BoardName != "" {
		msg += " (" + res.BoardName + " " + res.Version + ")"
	}
	return TestResult{Success: true, Message: msg, BoardName: res.BoardName, Version: res.Version}
}

// Test menguji koneksi tersimpan dan menyimpan hasilnya.
func (u *ConnectionUsecase) Test(id int64) (TestResult, error) {
	c, err := u.conns.GetByID(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return TestResult{}, InputError{"koneksi tidak ditemukan"}
		}
		return TestResult{}, err
	}
	p, err := u.dialParams(c)
	if err != nil {
		return TestResult{}, err
	}
	res := u.dialAndTest(p)
	if err := u.conns.SaveTestResult(id, res.Success, res.Message); err != nil {
		return TestResult{}, err
	}
	return res, nil
}

// TestParams menguji parameter koneksi tanpa menyimpan.
func (u *ConnectionUsecase) TestParams(in ConnectionInput) (TestResult, error) {
	in, err := validateConnection(in, true)
	if err != nil {
		return TestResult{}, err
	}
	return u.dialAndTest(RouterDialParams{
		Host: in.Host, Port: in.Port, Username: in.Username,
		Password: in.Password, UseTLS: in.UseTLS,
	}), nil
}

// ActiveSession membuka sesi ke koneksi aktif (dipakai profile/provisioning).
func (u *ConnectionUsecase) ActiveSession() (RouterSession, domain.Connection, error) {
	c, err := u.conns.GetActive()
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.Connection{}, InputError{"belum ada koneksi router aktif"}
		}
		return nil, domain.Connection{}, err
	}
	p, err := u.dialParams(c)
	if err != nil {
		return nil, c, err
	}
	sess, err := u.gw.Dial(p)
	if err != nil {
		return nil, c, InputError{"gagal terhubung ke router: " + shortErr(err)}
	}
	return sess, c, nil
}

// MarkTested mencatat hasil uji koneksi tanpa membuka sesi baru.
func (u *ConnectionUsecase) MarkTested(id int64, ok bool, msg string) error {
	return u.conns.SaveTestResult(id, ok, msg)
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160] + "..."
	}
	return s
}
