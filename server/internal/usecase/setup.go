package usecase

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// SetupUsecase menangani wizard penyiapan awal (first-run).
type SetupUsecase struct {
	admins   AdminRepository
	conns    ConnectionRepository
	set      SettingsRepository
	hash     PasswordHasher
	box      SecretBox
	router   *ConnectionUsecase
	prov     *ProvisioningUsecase
	invoices InvoiceClientFactory
	builder  InvoiceClientBuilder
}

// SetInvoiceFactory memasang pabrik client KrisKris (diisi main saat serve).
func (u *SetupUsecase) SetInvoiceFactory(f InvoiceClientFactory) {
	u.invoices = f
}

// SetInvoiceBuilder memasang pembangun client dari kredensial eksplisit.
func (u *SetupUsecase) SetInvoiceBuilder(b InvoiceClientBuilder) {
	u.builder = b
}

// NewSetupUsecase membuat SetupUsecase.
func NewSetupUsecase(admins AdminRepository, conns ConnectionRepository, set SettingsRepository, hash PasswordHasher, box SecretBox, router *ConnectionUsecase, prov *ProvisioningUsecase) *SetupUsecase {
	return &SetupUsecase{admins: admins, conns: conns, set: set, hash: hash, box: box, router: router, prov: prov}
}

// Completed melaporkan apakah penyiapan sudah selesai.
func (u *SetupUsecase) Completed() (bool, error) {
	v, found, err := u.set.Get(domain.SettingSetupCompleted)
	if err != nil {
		return false, err
	}
	return found && v == "1", nil
}

// Status melaporkan status penyiapan untuk guard first-run di frontend.
func (u *SetupUsecase) Status() (completed, hasAdmin bool, err error) {
	completed, err = u.Completed()
	if err != nil {
		return false, false, err
	}
	n, err := u.admins.Count()
	if err != nil {
		return false, false, err
	}
	return completed, n > 0, nil
}

// CreateAdminInput adalah masukan langkah 1 wizard.
type CreateAdminInput struct {
	Name     string
	Username string
	Password string
}

// CreateAdmin membuat akun admin pertama.
func (u *SetupUsecase) CreateAdmin(in CreateAdminInput) (domain.Admin, error) {
	name := strings.TrimSpace(in.Name)
	username := strings.TrimSpace(in.Username)
	switch {
	case name == "":
		return domain.Admin{}, InputError{"nama wajib diisi"}
	case len(name) > 100:
		return domain.Admin{}, InputError{"nama maksimal 100 karakter"}
	case username == "":
		return domain.Admin{}, InputError{"username wajib diisi"}
	case len(username) < 3 || len(username) > 32:
		return domain.Admin{}, InputError{"username 3 sampai 32 karakter"}
	case len(in.Password) < 8:
		return domain.Admin{}, InputError{"password minimal 8 karakter"}
	}
	n, err := u.admins.Count()
	if err != nil {
		return domain.Admin{}, err
	}
	if n > 0 {
		return domain.Admin{}, ConflictError{"akun admin sudah dibuat"}
	}
	hash, err := u.hash.Hash(in.Password)
	if err != nil {
		return domain.Admin{}, err
	}
	a, err := u.admins.Create(domain.Admin{Name: name, Username: username, PasswordHash: hash})
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return domain.Admin{}, ConflictError{"username sudah dipakai"}
		}
		return domain.Admin{}, err
	}
	a.PasswordHash = ""
	return a, nil
}

// SaveSiteInput adalah masukan langkah 2 wizard.
type SaveSiteInput struct {
	DeployMode      string
	SiteURL         string
	LocalServerIP   string
	HotspotLoginURL string
}

// SaveSite menyimpan alamat Mikbil.
func (u *SetupUsecase) SaveSite(in SaveSiteInput) error {
	mode := strings.TrimSpace(in.DeployMode)
	siteURL := strings.TrimSpace(in.SiteURL)
	loginBase := strings.TrimSpace(in.HotspotLoginURL)
	if mode != domain.DeployVPS && mode != domain.DeployLocal {
		return InputError{"mode deploy harus vps atau local"}
	}
	su, err := url.ParseRequestURI(siteURL)
	if err != nil || (su.Scheme != "https" && su.Scheme != "http") || su.Host == "" {
		return InputError{"alamat situs tidak valid"}
	}
	if mode == domain.DeployVPS && su.Scheme != "https" {
		return InputError{"mode VPS wajib memakai HTTPS"}
	}
	localIP := strings.TrimSpace(in.LocalServerIP)
	if mode == domain.DeployLocal {
		if net.ParseIP(localIP) == nil {
			return InputError{"IP server lokal tidak valid"}
		}
	}
	lu, err := url.ParseRequestURI(loginBase)
	if err != nil || (lu.Scheme != "https" && lu.Scheme != "http") || lu.Host == "" {
		return InputError{"alamat login hotspot tidak valid"}
	}
	for k, v := range map[string]string{
		domain.SettingDeployMode:       mode,
		domain.SettingSiteURL:          siteURL,
		domain.SettingLocalServerIP:    localIP,
		domain.SettingHotspotLoginBase: loginBase,
	} {
		if err := u.set.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// SaveConnectionInput adalah masukan langkah 3 wizard.
type SaveConnectionInput struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
}

// SaveConnection menyimpan koneksi router (password dienkripsi).
func (u *SetupUsecase) SaveConnection(in SaveConnectionInput) (domain.Connection, error) {
	host := strings.TrimSpace(in.Host)
	username := strings.TrimSpace(in.Username)
	port := in.Port
	if port == 0 {
		if in.UseTLS {
			port = 8729
		} else {
			port = 8728
		}
	}
	switch {
	case host == "":
		return domain.Connection{}, InputError{"host router wajib diisi"}
	case port < 1 || port > 65535:
		return domain.Connection{}, InputError{"port router 1 sampai 65535"}
	case username == "":
		return domain.Connection{}, InputError{"username router wajib diisi"}
	case in.Password == "":
		return domain.Connection{}, InputError{"password router wajib diisi"}
	}
	enc, err := u.box.Seal(in.Password)
	if err != nil {
		return domain.Connection{}, err
	}
	c, err := u.conns.CreateActive(domain.Connection{
		Name: "default", Host: host, Port: port,
		Username: username, EncPassword: enc, UseTLS: in.UseTLS,
	})
	if err != nil {
		return domain.Connection{}, err
	}
	c.EncPassword = ""
	return c, nil
}

// TestConnection menguji koneksi router tanpa menyimpan.
// Bila parameter cocok dengan koneksi tersimpan, hasilnya dicatat untuk
// memenuhi syarat finish.
func (u *SetupUsecase) TestConnection(in SaveConnectionInput) (TestResult, error) {
	res, err := u.router.TestParams(ConnectionInput{
		Host: in.Host, Port: in.Port, Username: in.Username,
		Password: in.Password, UseTLS: in.UseTLS,
	})
	if err != nil {
		return res, err
	}
	if saved, serr := u.conns.GetActive(); serr == nil && sameTarget(saved, in) {
		_ = u.router.MarkTested(saved.ID, res.Success, res.Message)
	}
	return res, nil
}

func sameTarget(saved domain.Connection, in SaveConnectionInput) bool {
	port := in.Port
	if port == 0 {
		port = defaultPort(in.UseTLS)
	}
	return saved.Host == strings.TrimSpace(in.Host) &&
		saved.Port == port &&
		saved.Username == strings.TrimSpace(in.Username) &&
		saved.UseTLS == in.UseTLS
}

// SaveKrisKrisInput adalah masukan langkah 4 wizard.
type SaveKrisKrisInput struct {
	APIKey         string
	APISecret      string
	CallbackSecret string
}

// SaveKrisKris menyimpan kredensial KrisKris (secret dienkripsi).
// Callback secret wajib di mode VPS; opsional di mode lokal (tanpa webhook).
func (u *SetupUsecase) SaveKrisKris(in SaveKrisKrisInput) error {
	key := strings.TrimSpace(in.APIKey)
	switch {
	case key == "":
		return InputError{"API key wajib diisi"}
	case in.APISecret == "":
		return InputError{"API secret wajib diisi"}
	}
	mode, _, _ := u.set.Get(domain.SettingDeployMode)
	if mode != domain.DeployLocal && in.CallbackSecret == "" {
		return InputError{"callback secret wajib diisi di mode VPS"}
	}
	encSecret, err := u.box.Seal(in.APISecret)
	if err != nil {
		return err
	}
	writes := map[string]string{
		domain.SettingKrisKrisAPIKey:    key,
		domain.SettingKrisKrisAPISecret: encSecret,
	}
	if in.CallbackSecret != "" {
		encCallback, err := u.box.Seal(in.CallbackSecret)
		if err != nil {
			return err
		}
		writes[domain.SettingKrisKrisCallbackSec] = encCallback
	}
	for k, v := range writes {
		if err := u.set.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// TestKrisKris memvalidasi kredensial via batch status id dummy:
// sukses bila dapat respons envelope valid (bukan 401).
// Kredensial eksplisit (keduanya wajib) dites tanpa menyimpan;
// bila kosong, dipakai kredensial tersimpan.
func (u *SetupUsecase) TestKrisKris(apiKey, apiSecret string) error {
	var client InvoiceClient
	var err error
	switch {
	case apiKey != "" || apiSecret != "":
		if apiKey == "" || apiSecret == "" {
			return InputError{"API key dan API secret wajib diisi untuk tes"}
		}
		if u.builder == nil {
			return NotImplementedError{"tes koneksi KrisKris tersedia mulai fase 4"}
		}
		client, err = u.builder(apiKey, apiSecret)
		if err != nil {
			return err
		}
	default:
		if u.invoices == nil {
			return NotImplementedError{"tes koneksi KrisKris tersedia mulai fase 4"}
		}
		client, err = u.invoices()
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.BatchStatus(ctx, []string{probeInvoiceID})
	if err == nil {
		return nil
	}
	var invErr InvoiceError
	if !errors.As(err, &invErr) {
		return InputError{"tes koneksi gagal: " + shortErr(err)}
	}
	switch {
	case invErr.Status == 401:
		return InputError{"API key atau secret salah (401)"}
	case invErr.Status >= 500:
		// Batch 5xx: coba GET satu invoice; 200 maupun 404 sama-sama
		// membuktikan kredensial valid (404 = auth lolos, id memang asing).
		return probeGetInvoice(client, ctx)
	case invErr.Status > 0:
		return InputError{fmt.Sprintf("tes koneksi gagal (HTTP %d): %s", invErr.Status, shortErr(err))}
	default:
		return InputError{"tes koneksi gagal: " + shortErr(err)}
	}
}

// probeInvoiceID adalah id dummy berbentuk invoice asli untuk tes koneksi.
const probeInvoiceID = "inv-uji-koneksi"

// probeGetInvoice adalah probe cadangan saat batch status 5xx.
func probeGetInvoice(client InvoiceClient, ctx context.Context) error {
	_, err := client.GetInvoice(ctx, probeInvoiceID)
	if err == nil {
		return nil
	}
	var invErr InvoiceError
	if !errors.As(err, &invErr) {
		return InputError{"tes koneksi gagal: " + shortErr(err)}
	}
	switch {
	case invErr.Status == 404:
		return nil
	case invErr.Status == 401:
		return InputError{"API key atau secret salah (401)"}
	case invErr.Status > 0:
		return InputError{fmt.Sprintf("tes koneksi gagal (HTTP %d): %s", invErr.Status, shortErr(err))}
	default:
		return InputError{"tes koneksi gagal: " + shortErr(err)}
	}
}

// Provision menjalankan provisioning router, hasil per langkah.
func (u *SetupUsecase) Provision() ([]domain.ProvisionStep, error) {
	return u.prov.Apply()
}

// Finish menandai penyiapan selesai.
// Syarat: admin ada + koneksi router tersimpan + tes koneksi terakhir sukses.
func (u *SetupUsecase) Finish() error {
	n, err := u.admins.Count()
	if err != nil {
		return err
	}
	if n == 0 {
		return InputError{"buat akun admin dulu sebelum selesai"}
	}
	c, err := u.conns.GetActive()
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return InputError{"simpan koneksi router dulu sebelum selesai"}
		}
		return err
	}
	if c.LastTestOK == nil || !*c.LastTestOK {
		return InputError{"tes koneksi router harus sukses dulu sebelum selesai"}
	}
	return u.set.Set(domain.SettingSetupCompleted, "1")
}
