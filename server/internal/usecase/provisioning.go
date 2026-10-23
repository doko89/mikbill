package usecase

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// ProvisioningUsecase menerapkan dan memverifikasi konfigurasi router.
type ProvisioningUsecase struct {
	set   SettingsRepository
	conns *ConnectionUsecase
}

// NewProvisioningUsecase membuat ProvisioningUsecase.
func NewProvisioningUsecase(set SettingsRepository, conns *ConnectionUsecase) *ProvisioningUsecase {
	return &ProvisioningUsecase{set: set, conns: conns}
}

// provisionPlan adalah ekspektasi provisioning dari settings.
type provisionPlan struct {
	mode    string
	siteURL string
	host    string // domain publik / domain lokal tanpa port
	localIP string
}

func (u *ProvisioningUsecase) plan() (provisionPlan, error) {
	var p provisionPlan
	mode, _, err := u.set.Get(domain.SettingDeployMode)
	if err != nil {
		return p, err
	}
	siteURL, _, err := u.set.Get(domain.SettingSiteURL)
	if err != nil {
		return p, err
	}
	localIP, _, err := u.set.Get(domain.SettingLocalServerIP)
	if err != nil {
		return p, err
	}
	if mode != domain.DeployVPS && mode != domain.DeployLocal {
		return p, InputError{"mode deploy belum diatur"}
	}
	su, err := url.ParseRequestURI(strings.TrimSpace(siteURL))
	if err != nil || su.Hostname() == "" {
		return p, InputError{"alamat situs belum valid"}
	}
	p = provisionPlan{mode: mode, siteURL: siteURL, host: su.Hostname(), localIP: strings.TrimSpace(localIP)}
	if mode == domain.DeployLocal && p.localIP == "" {
		return p, InputError{"IP server lokal belum diatur"}
	}
	return p, nil
}

// Apply menjalankan sequence provisioning tech_spec bagian 8.
func (u *ProvisioningUsecase) Apply() ([]domain.ProvisionStep, error) {
	p, err := u.plan()
	if err != nil {
		return nil, err
	}
	sess, active, err := u.conns.ActiveSession()
	if err != nil {
		if active.ID != 0 {
			_ = u.conns.MarkTested(active.ID, false, err.Error())
		}
		return []domain.ProvisionStep{{Step: "koneksi", OK: false, Message: err.Error()}}, nil
	}
	defer sess.Close()
	_ = u.conns.MarkTested(active.ID, true, "terhubung ke router")
	steps := []domain.ProvisionStep{{Step: "koneksi", OK: true, Message: "terhubung ke router"}}

	// Langkah http-pap: tambah aditif ke semua server profile yang belum punya.
	profs, err := sess.ListServerProfiles()
	if err != nil {
		steps = append(steps, domain.ProvisionStep{Step: "http-pap", OK: false, Message: shortErr(err)})
		return steps, nil
	}
	patched, failed := 0, 0
	for _, sp := range profs {
		if hasLoginBy(sp.LoginBy, "http-pap") {
			continue
		}
		next := strings.Trim(sp.LoginBy+",http-pap", ",")
		if err := sess.SetServerProfileLoginBy(sp.ID, next); err != nil {
			failed++
			continue
		}
		patched++
	}
	switch {
	case failed > 0:
		steps = append(steps, domain.ProvisionStep{Step: "http-pap", OK: false,
			Message: fmt.Sprintf("%d profile diperbarui, %d gagal", patched, failed)})
	case patched == 0:
		steps = append(steps, domain.ProvisionStep{Step: "http-pap", OK: true, Message: "http-pap sudah aktif di semua profile"})
	default:
		steps = append(steps, domain.ProvisionStep{Step: "http-pap", OK: true,
			Message: fmt.Sprintf("http-pap diaktifkan di %d profile", patched)})
	}

	// Langkah walled garden: hapus entri mikbil lama lalu tambah baru.
	wg, err := sess.ListWalledGarden()
	if err != nil {
		steps = append(steps, domain.ProvisionStep{Step: "walled-garden", OK: false, Message: shortErr(err)})
		return steps, nil
	}
	for _, e := range wg {
		if e.Comment == domain.RouterCommentMikbil {
			_ = sess.RemoveWalledGarden(e.ID)
		}
	}
	var want []domain.WalledGardenEntry
	if p.mode == domain.DeployVPS {
		want = []domain.WalledGardenEntry{{DstHost: p.host}}
	} else {
		want = []domain.WalledGardenEntry{{DstAddress: p.localIP}, {DstHost: p.host}}
	}
	added := 0
	for _, e := range want {
		if err := sess.AddWalledGarden(e); err == nil {
			added++
		}
	}
	if added != len(want) {
		steps = append(steps, domain.ProvisionStep{Step: "walled-garden", OK: false,
			Message: fmt.Sprintf("hanya %d dari %d entri tertulis", added, len(want))})
		return steps, nil
	}
	steps = append(steps, domain.ProvisionStep{Step: "walled-garden", OK: true,
		Message: fmt.Sprintf("%d entri walled garden ditulis", added)})

	// Langkah DNS statik: mode lokal saja.
	if p.mode == domain.DeployLocal {
		dns, err := sess.ListStaticDNS()
		if err != nil {
			steps = append(steps, domain.ProvisionStep{Step: "dns-statik", OK: false, Message: shortErr(err)})
			return steps, nil
		}
		for _, e := range dns {
			if e.Comment == domain.RouterCommentMikbil {
				_ = sess.RemoveStaticDNS(e.ID)
			}
		}
		if err := sess.AddStaticDNS(domain.StaticDNSEntry{Name: p.host, Address: p.localIP}); err != nil {
			steps = append(steps, domain.ProvisionStep{Step: "dns-statik", OK: false, Message: shortErr(err)})
			return steps, nil
		}
		msg := p.host + " mengarah ke " + p.localIP
		allow, err := sess.DNSAllowRemoteRequests()
		if err == nil && !allow {
			if err := sess.SetDNSAllowRemoteRequests(true); err == nil {
				msg += "; allow-remote-requests diubah ke yes"
			} else {
				msg += "; allow-remote-requests masih no (ubah manual di router)"
			}
		}
		steps = append(steps, domain.ProvisionStep{Step: "dns-statik", OK: true, Message: msg})
	}

	// Langkah verify: baca ulang.
	st, err := u.statusWith(sess, p)
	if err != nil {
		steps = append(steps, domain.ProvisionStep{Step: "verify", OK: false, Message: shortErr(err)})
		return steps, nil
	}
	allOK := true
	for _, c := range st.Checks {
		allOK = allOK && c.OK
	}
	msg := "semua pemeriksaan lolos"
	if !allOK {
		msg = "ada pemeriksaan yang belum lolos, lihat status provisioning"
	}
	steps = append(steps, domain.ProvisionStep{Step: "verify", OK: allOK, Message: msg})
	return steps, nil
}

// Status memverifikasi kondisi router tanpa menulis apa pun (read-only).
func (u *ProvisioningUsecase) Status() (domain.ProvisionStatus, error) {
	p, err := u.plan()
	if err != nil {
		return domain.ProvisionStatus{}, err
	}
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return domain.ProvisionStatus{
			DeployMode: p.mode, SiteURL: p.siteURL,
			Checks: []domain.ProvisionCheck{{Name: "koneksi", OK: false, Message: err.Error()}},
		}, nil
	}
	defer sess.Close()
	return u.statusWith(sess, p)
}

func (u *ProvisioningUsecase) statusWith(sess RouterSession, p provisionPlan) (domain.ProvisionStatus, error) {
	st := domain.ProvisionStatus{DeployMode: p.mode, SiteURL: p.siteURL}
	st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "koneksi", OK: true, Message: "terhubung ke router"})

	profs, err := sess.ListServerProfiles()
	if err != nil {
		st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "http-pap", OK: false, Message: shortErr(err)})
	} else if len(profs) == 0 {
		st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "http-pap", OK: false, Message: "tidak ada hotspot server profile di router"})
	} else {
		missing := 0
		for _, sp := range profs {
			if !hasLoginBy(sp.LoginBy, "http-pap") {
				missing++
			}
		}
		if missing > 0 {
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "http-pap", OK: false,
				Message: fmt.Sprintf("%d dari %d profile belum punya http-pap", missing, len(profs))})
		} else {
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "http-pap", OK: true,
				Message: fmt.Sprintf("http-pap aktif di %d profile", len(profs))})
		}
	}

	wg, err := sess.ListWalledGarden()
	if err != nil {
		st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "walled-garden", OK: false, Message: shortErr(err)})
	} else {
		var want, found []string
		if p.mode == domain.DeployVPS {
			want = []string{"dst-host=" + p.host}
		} else {
			want = []string{"dst-address=" + p.localIP, "dst-host=" + p.host}
		}
		for _, e := range wg {
			if e.Comment != domain.RouterCommentMikbil || e.Disabled {
				continue
			}
			if e.DstHost != "" {
				found = append(found, "dst-host="+e.DstHost)
			}
			if e.DstAddress != "" {
				found = append(found, "dst-address="+e.DstAddress)
			}
		}
		missing := false
		for _, w := range want {
			if !containsStr(found, w) {
				missing = true
			}
		}
		if missing {
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "walled-garden", OK: false,
				Message: "entri walled garden Mikbil belum lengkap"})
		} else {
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "walled-garden", OK: true, Message: "entri walled garden lengkap"})
		}
	}

	if p.mode == domain.DeployLocal {
		dns, err := sess.ListStaticDNS()
		if err != nil {
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "dns-statik", OK: false, Message: shortErr(err)})
		} else {
			ok := false
			for _, e := range dns {
				if e.Name == p.host && e.Address == p.localIP {
					ok = true
				}
			}
			allow, aerr := sess.DNSAllowRemoteRequests()
			msg := p.host + " mengarah ke " + p.localIP
			if aerr == nil && !allow {
				ok = false
				msg += ", tetapi allow-remote-requests masih no"
			}
			if !ok && aerr == nil && allow {
				msg = "DNS statik " + msg + " belum ada"
			}
			st.Checks = append(st.Checks, domain.ProvisionCheck{Name: "dns-statik", OK: ok, Message: msg})
		}
	}
	return st, nil
}

func hasLoginBy(loginBy, want string) bool {
	for _, part := range strings.Split(loginBy, ",") {
		if strings.TrimSpace(part) == want {
			return true
		}
	}
	return false
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
