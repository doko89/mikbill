package usecase

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// allowHostRe memvalidasi nama domain, opsional diawali wildcard "*.".
var allowHostRe = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

// SyncResult melaporkan hasil sinkronisasi perubahan ke router.
type SyncResult struct {
	Synced bool
	Error  string // kosong bila Synced
}

func okSync() SyncResult { return SyncResult{Synced: true} }

func failSync(err error) SyncResult { return SyncResult{Error: shortErr(err)} }

// AllowlistUsecase menangani daftar host allowlist walled garden: domain
// yang boleh diakses tanpa login (mis. channel pembayaran). Setiap mutasi
// disimpan ke DB lalu disinkronkan ke router (best-effort); provisioning
// berikutnya selalu merekonsiliasi penuh.
type AllowlistUsecase struct {
	repo  AllowlistRepository
	set   SettingsRepository
	conns *ConnectionUsecase
}

// NewAllowlistUsecase membuat AllowlistUsecase.
func NewAllowlistUsecase(repo AllowlistRepository, set SettingsRepository, conns *ConnectionUsecase) *AllowlistUsecase {
	return &AllowlistUsecase{repo: repo, set: set, conns: conns}
}

// List mengembalikan semua host allowlist.
func (u *AllowlistUsecase) List() ([]domain.WalledGardenAllow, error) {
	return u.repo.List()
}

// Get mengambil satu host allowlist.
func (u *AllowlistUsecase) Get(id int64) (domain.WalledGardenAllow, error) {
	a, err := u.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.WalledGardenAllow{}, InputError{"host allowlist tidak ditemukan"}
		}
		return domain.WalledGardenAllow{}, err
	}
	return a, nil
}

func (u *AllowlistUsecase) siteHost() string {
	v, found, err := u.set.Get(domain.SettingSiteURL)
	if err != nil || !found {
		return ""
	}
	su, err := url.ParseRequestURI(strings.TrimSpace(v))
	if err != nil {
		return ""
	}
	return strings.ToLower(su.Hostname())
}

// normalize merapikan dan memvalidasi nama domain allowlist.
func (u *AllowlistUsecase) normalize(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	switch {
	case host == "":
		return "", InputError{"domain wajib diisi"}
	case len(host) > 253:
		return "", InputError{"domain maksimal 253 karakter"}
	case net.ParseIP(host) != nil:
		return "", InputError{"gunakan nama domain, bukan alamat IP"}
	case !allowHostRe.MatchString(host):
		return "", InputError{"nama domain tidak valid"}
	case u.siteHost() != "" && host == u.siteHost():
		return "", InputError{"domain situs sudah dicakup provisioning otomatis"}
	}
	return host, nil
}

// syncAdd memastikan entri dst-host ada di router.
func (u *AllowlistUsecase) syncAdd(host string) error {
	return u.syncAddMany([]string{host})
}

// syncAddMany memastikan banyak entri dst-host ada di router dalam satu sesi.
func (u *AllowlistUsecase) syncAddMany(hosts []string) error {
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	wg, err := sess.ListWalledGarden()
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, e := range wg {
		if e.Comment == domain.RouterCommentMikbil && !e.Disabled && e.DstHost != "" {
			present[e.DstHost] = true
		}
	}
	var firstErr error
	for _, h := range hosts {
		if present[h] {
			continue
		}
		if err := sess.AddWalledGarden(domain.WalledGardenEntry{DstHost: h}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		present[h] = true
	}
	return firstErr
}

// syncRemove menghapus semua entri dst-host milik Mikbil dari router.
func (u *AllowlistUsecase) syncRemove(host string) error {
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	wg, err := sess.ListWalledGarden()
	if err != nil {
		return err
	}
	var firstErr error
	for _, e := range wg {
		if e.Comment != domain.RouterCommentMikbil || e.DstHost != host {
			continue
		}
		if err := sess.RemoveWalledGarden(e.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Create menambah host allowlist; bila aktif langsung ditulis ke router.
func (u *AllowlistUsecase) Create(dstHost string, enabled bool) (domain.WalledGardenAllow, SyncResult, error) {
	host, err := u.normalize(dstHost)
	if err != nil {
		return domain.WalledGardenAllow{}, SyncResult{}, err
	}
	a, err := u.repo.Create(host, enabled)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return domain.WalledGardenAllow{}, SyncResult{}, ConflictError{"domain sudah ada di allowlist"}
		}
		return domain.WalledGardenAllow{}, SyncResult{}, err
	}
	if !enabled {
		return a, okSync(), nil
	}
	if err := u.syncAdd(host); err != nil {
		return a, failSync(err), nil
	}
	return a, okSync(), nil
}

// Update mengubah host allowlist; disable/hapus nama langsung
// menghapus entri yang tidak terpakai dari router.
func (u *AllowlistUsecase) Update(id int64, dstHost string, enabled bool) (domain.WalledGardenAllow, SyncResult, error) {
	old, err := u.Get(id)
	if err != nil {
		return domain.WalledGardenAllow{}, SyncResult{}, err
	}
	host, err := u.normalize(dstHost)
	if err != nil {
		return domain.WalledGardenAllow{}, SyncResult{}, err
	}
	a, err := u.repo.Update(domain.WalledGardenAllow{ID: id, DstHost: host, Enabled: enabled})
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return domain.WalledGardenAllow{}, SyncResult{}, ConflictError{"domain sudah ada di allowlist"}
		}
		return domain.WalledGardenAllow{}, SyncResult{}, err
	}
	var syncErr error
	if host != old.DstHost {
		if err := u.syncRemove(old.DstHost); err != nil {
			syncErr = err
		}
	}
	if enabled {
		if err := u.syncAdd(host); err != nil && syncErr == nil {
			syncErr = err
		}
	} else if err := u.syncRemove(host); err != nil && syncErr == nil {
		syncErr = err
	}
	if syncErr != nil {
		return a, failSync(syncErr), nil
	}
	return a, okSync(), nil
}

// BulkItem adalah satu butir host bulk yang gagal divalidasi.
type BulkItem struct {
	Host  string
	Error string
}

// BulkResult adalah hasil tambah banyak host allowlist.
type BulkResult struct {
	Added       []domain.WalledGardenAllow
	Skipped     []string // sudah ada di allowlist
	Failed      []BulkItem
	RouterError string // "" bila sinkronisasi router OK
}

// maxBulkHosts membatasi jumlah domain per permintaan bulk.
const maxBulkHosts = 100

// CreateBulk menambah banyak host sekaligus; yang sudah ada dilewati,
// yang tidak valid dilaporkan per butir. Sinkronisasi router satu sesi.
func (u *AllowlistUsecase) CreateBulk(rawHosts []string, enabled bool) (BulkResult, error) {
	res := BulkResult{
		Added: []domain.WalledGardenAllow{}, Skipped: []string{}, Failed: []BulkItem{},
	}
	if len(rawHosts) == 0 {
		return res, InputError{"daftar domain kosong"}
	}
	if len(rawHosts) > maxBulkHosts {
		return res, InputError{"maksimal 100 domain per permintaan"}
	}
	seen := map[string]bool{}
	for _, raw := range rawHosts {
		host, err := u.normalize(raw)
		if err != nil {
			res.Failed = append(res.Failed, BulkItem{Host: strings.TrimSpace(raw), Error: err.Error()})
			continue
		}
		if seen[host] {
			continue
		}
		seen[host] = true
		a, err := u.repo.Create(host, enabled)
		if err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				res.Skipped = append(res.Skipped, host)
				continue
			}
			return BulkResult{}, err
		}
		res.Added = append(res.Added, a)
	}
	if !enabled || len(res.Added) == 0 {
		return res, nil
	}
	hosts := make([]string, 0, len(res.Added))
	for _, a := range res.Added {
		hosts = append(hosts, a.DstHost)
	}
	if err := u.syncAddMany(hosts); err != nil {
		res.RouterError = shortErr(err)
	}
	return res, nil
}

// Delete menghapus host allowlist dan entrinya dari router.
func (u *AllowlistUsecase) Delete(id int64) (SyncResult, error) {
	old, err := u.Get(id)
	if err != nil {
		return SyncResult{}, err
	}
	if err := u.repo.Delete(id); err != nil {
		return SyncResult{}, err
	}
	if err := u.syncRemove(old.DstHost); err != nil {
		return failSync(err), nil
	}
	return okSync(), nil
}
