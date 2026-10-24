package usecase

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

// fakeBox adalah SecretBox memori untuk test.
type fakeBox struct{}

func (fakeBox) Seal(p string) (string, error) { return "enc:" + p, nil }
func (fakeBox) Open(c string) (string, error) {
	if len(c) < 4 || c[:4] != "enc:" {
		return "", errors.New("cipher buruk")
	}
	return c[4:], nil
}

// fakeConns adalah ConnectionRepository memori untuk test.
type fakeConns struct {
	mu     sync.Mutex
	rows   map[int64]domain.Connection
	nextID int64
}

func newFakeConns() *fakeConns { return &fakeConns{rows: map[int64]domain.Connection{}, nextID: 1} }

func (f *fakeConns) insert(c domain.Connection) domain.Connection {
	if c.IsActive {
		for id, r := range f.rows {
			r.IsActive = false
			f.rows[id] = r
		}
	}
	c.ID = f.nextID
	f.nextID++
	f.rows[c.ID] = c
	return c
}

func (f *fakeConns) CreateActive(c domain.Connection) (domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c.IsActive = true
	return f.insert(c), nil
}

func (f *fakeConns) GetActive() (domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var best domain.Connection
	found := false
	for _, r := range f.rows {
		if r.IsActive && (!found || r.ID > best.ID) {
			best, found = r, true
		}
	}
	if !found {
		return domain.Connection{}, domain.ErrNotFound
	}
	return best, nil
}

func (f *fakeConns) List() ([]domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Connection{}
	for _, r := range f.rows {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeConns) GetByID(id int64) (domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return domain.Connection{}, domain.ErrNotFound
	}
	return r, nil
}

func (f *fakeConns) Create(c domain.Connection) (domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rows) == 0 {
		c.IsActive = true
	}
	return f.insert(c), nil
}

func (f *fakeConns) Update(c domain.Connection) (domain.Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.rows[c.ID]
	if !ok {
		return domain.Connection{}, domain.ErrNotFound
	}
	if c.IsActive {
		for id, r := range f.rows {
			r.IsActive = false
			f.rows[id] = r
		}
	}
	c.LastTestAt, c.LastTestOK, c.LastTestMsg = old.LastTestAt, old.LastTestOK, old.LastTestMsg
	f.rows[c.ID] = c
	return c, nil
}

func (f *fakeConns) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(f.rows, id)
	if r.IsActive {
		var best int64
		for cid := range f.rows {
			if cid > best {
				best = cid
			}
		}
		if best != 0 {
			nr := f.rows[best]
			nr.IsActive = true
			f.rows[best] = nr
		}
	}
	return nil
}

func (f *fakeConns) SaveTestResult(id int64, ok bool, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, found := f.rows[id]
	if !found {
		return domain.ErrNotFound
	}
	r.LastTestOK = &ok
	r.LastTestMsg = msg
	f.rows[id] = r
	return nil
}

// fakeSettings adalah SettingsRepository memori untuk test.
type fakeSettings struct {
	mu sync.Mutex
	kv map[string]string
}

func newFakeSettings() *fakeSettings { return &fakeSettings{kv: map[string]string{}} }

func (f *fakeSettings) Get(key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.kv[key]
	return v, ok, nil
}

func (f *fakeSettings) Set(key, val string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kv[key] = val
	return nil
}

// fakeSession meniru RouterSession dan mencatat semua panggilan tulis.
type fakeSession struct {
	mu            sync.Mutex
	closed        bool
	testErr       error
	userProfiles  []domain.HotspotProfile
	serverProfile []domain.HotspotServerProfile
	walledGarden  []domain.WalledGardenEntry
	staticDNS     []domain.StaticDNSEntry
	allowRemote   bool
	writes        []string
	users         []domain.HotspotUser
	failWGAddr    bool // tolak entri dst-address seperti router pada menu yang salah
}

func (s *fakeSession) Close() error { s.closed = true; return nil }

func (s *fakeSession) Test() (domain.RouterResource, error) {
	if s.testErr != nil {
		return domain.RouterResource{}, s.testErr
	}
	return domain.RouterResource{BoardName: "CHR", Version: "7.14"}, nil
}

func (s *fakeSession) ListUserProfiles() ([]domain.HotspotProfile, error) {
	return append([]domain.HotspotProfile{}, s.userProfiles...), nil
}

func (s *fakeSession) CreateUserProfile(p domain.HotspotProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "create-profile:"+p.Name)
	s.userProfiles = append(s.userProfiles, p)
	return nil
}

func (s *fakeSession) AddHotspotUser(u domain.HotspotUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "add-user:"+u.Name)
	s.users = append(s.users, u)
	return nil
}

func (s *fakeSession) FindHotspotUserByComment(comment string) (domain.HotspotUser, bool, error) {
	for _, u := range s.users {
		if u.Comment == comment {
			return u, true, nil
		}
	}
	return domain.HotspotUser{}, false, nil
}

func (s *fakeSession) ListWalledGarden() ([]domain.WalledGardenEntry, error) {
	return append([]domain.WalledGardenEntry{}, s.walledGarden...), nil
}

func (s *fakeSession) AddWalledGarden(e domain.WalledGardenEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWGAddr && e.DstAddress != "" {
		return errors.New("unknown parameter dst-address")
	}
	s.writes = append(s.writes, "add-wg:"+e.DstHost+e.DstAddress)
	e.ID = string(rune(len(s.walledGarden) + 1))
	e.Comment = domain.RouterCommentMikbil
	s.walledGarden = append(s.walledGarden, e)
	return nil
}

func (s *fakeSession) RemoveWalledGarden(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "remove-wg:"+id)
	kept := s.walledGarden[:0]
	for _, e := range s.walledGarden {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	s.walledGarden = kept
	return nil
}

func (s *fakeSession) ListServerProfiles() ([]domain.HotspotServerProfile, error) {
	return append([]domain.HotspotServerProfile{}, s.serverProfile...), nil
}

func (s *fakeSession) SetServerProfileLoginBy(id, loginBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "set-login-by:"+id)
	for i, sp := range s.serverProfile {
		if sp.ID == id {
			s.serverProfile[i].LoginBy = loginBy
		}
	}
	return nil
}

func (s *fakeSession) ListStaticDNS() ([]domain.StaticDNSEntry, error) {
	return append([]domain.StaticDNSEntry{}, s.staticDNS...), nil
}

func (s *fakeSession) AddStaticDNS(e domain.StaticDNSEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "add-dns:"+e.Name)
	e.ID = string(rune(len(s.staticDNS) + 1))
	e.Comment = domain.RouterCommentMikbil
	s.staticDNS = append(s.staticDNS, e)
	return nil
}

func (s *fakeSession) RemoveStaticDNS(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "remove-dns:"+id)
	kept := s.staticDNS[:0]
	for _, e := range s.staticDNS {
		if e.ID != id {
			kept = append(kept, e)
		}
	}
	s.staticDNS = kept
	return nil
}

func (s *fakeSession) DNSAllowRemoteRequests() (bool, error) { return s.allowRemote, nil }

func (s *fakeSession) SetDNSAllowRemoteRequests(allow bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, "set-allow-remote")
	s.allowRemote = allow
	return nil
}

// fakeGateway meniru RouterGateway; dial gagal bila failDial true.
type fakeGateway struct {
	mu       sync.Mutex
	sess     *fakeSession
	failDial bool
	lastDial RouterDialParams
}

func (g *fakeGateway) Dial(p RouterDialParams) (RouterSession, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastDial = p
	if g.failDial {
		return nil, errors.New("koneksi ditolak")
	}
	return g.sess, nil
}

// fakeAllowlist adalah AllowlistRepository memori untuk test.
type fakeAllowlist struct {
	mu     sync.Mutex
	rows   map[int64]domain.WalledGardenAllow
	nextID int64
}

func newFakeAllowlist() *fakeAllowlist {
	return &fakeAllowlist{rows: map[int64]domain.WalledGardenAllow{}, nextID: 1}
}

func (f *fakeAllowlist) List() ([]domain.WalledGardenAllow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.WalledGardenAllow{}
	for _, r := range f.rows {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeAllowlist) GetByID(id int64) (domain.WalledGardenAllow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return domain.WalledGardenAllow{}, domain.ErrNotFound
	}
	return r, nil
}

func (f *fakeAllowlist) Create(dstHost string, enabled bool) (domain.WalledGardenAllow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.DstHost == dstHost {
			return domain.WalledGardenAllow{}, domain.ErrAlreadyExists
		}
	}
	a := domain.WalledGardenAllow{ID: f.nextID, DstHost: dstHost, Enabled: enabled}
	f.nextID++
	f.rows[a.ID] = a
	return a, nil
}

func (f *fakeAllowlist) Update(a domain.WalledGardenAllow) (domain.WalledGardenAllow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[a.ID]; !ok {
		return domain.WalledGardenAllow{}, domain.ErrNotFound
	}
	for id, r := range f.rows {
		if id != a.ID && r.DstHost == a.DstHost {
			return domain.WalledGardenAllow{}, domain.ErrAlreadyExists
		}
	}
	f.rows[a.ID] = a
	return a, nil
}

func (f *fakeAllowlist) Delete(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func newStack() (*fakeConns, *fakeSettings, *fakeGateway, *ConnectionUsecase, *ProvisioningUsecase, *fakeAllowlist) {
	conns := newFakeConns()
	set := newFakeSettings()
	allow := newFakeAllowlist()
	gw := &fakeGateway{sess: &fakeSession{
		serverProfile: []domain.HotspotServerProfile{{ID: "*1", Name: "hsprof1", LoginBy: "chap,cookie"}},
	}}
	connUC := NewConnectionUsecase(conns, fakeBox{}, gw)
	provUC := NewProvisioningUsecase(set, connUC, allow)
	return conns, set, gw, connUC, provUC, allow
}

func mustSaveConn(t *testing.T, connUC *ConnectionUsecase) domain.Connection {
	t.Helper()
	c, err := connUC.Create(ConnectionInput{
		Host: "192.168.88.1", Username: "admin", Password: "rahasia",
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConnectionCRUD(t *testing.T) {
	_, _, _, connUC, _, _ := newStack()

	c, err := connUC.Create(ConnectionInput{Host: "10.0.0.1", Username: "a", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8728 || !c.IsActive || c.EncPassword != "" {
		t.Fatalf("create = %+v, want port 8728 aktif tanpa password", c)
	}
	if _, err := connUC.Create(ConnectionInput{Username: "a", Password: "p"}); err == nil {
		t.Fatal("create tanpa host seharusnya gagal")
	}

	got, err := connUC.Get(c.ID)
	if err != nil || got.Host != "10.0.0.1" {
		t.Fatalf("get = %+v %v", got, err)
	}
	if _, err := connUC.Get(999); err == nil {
		t.Fatal("get id asing seharusnya gagal")
	}

	up, err := connUC.Update(c.ID, ConnectionInput{
		Name: "kantor", Host: "10.0.0.2", Port: 8729,
		Username: "a", UseTLS: true, IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if up.Host != "10.0.0.2" || up.Port != 8729 || !up.UseTLS {
		t.Fatalf("update = %+v", up)
	}

	list, err := connUC.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if err := connUC.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := connUC.Delete(c.ID); err == nil {
		t.Fatal("hapus ganda seharusnya gagal")
	}
}

func TestConnectionTestMenyimpanHasil(t *testing.T) {
	conns, _, gw, connUC, _, _ := newStack()
	c := mustSaveConn(t, connUC)

	res, err := connUC.Test(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.BoardName != "CHR" {
		t.Fatalf("test sukses = %+v", res)
	}
	saved, _ := conns.GetByID(c.ID)
	if saved.LastTestOK == nil || !*saved.LastTestOK {
		t.Fatal("hasil tes sukses tidak tercatat")
	}

	gw.failDial = true
	res, err = connUC.Test(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatal("test gagal seharusnya success=false")
	}
	saved, _ = conns.GetByID(c.ID)
	if saved.LastTestOK == nil || *saved.LastTestOK {
		t.Fatal("hasil tes gagal tidak tercatat")
	}

	if _, err := connUC.Test(999); err == nil {
		t.Fatal("test id asing seharusnya gagal")
	}
}

func TestProfileListCreate(t *testing.T) {
	_, _, gw, connUC, _, _ := newStack()
	profUC := NewProfileUsecase(connUC)
	gw.sess.userProfiles = []domain.HotspotProfile{{Name: "default", SharedUsers: 1}}

	if _, err := profUC.List(); err == nil {
		t.Fatal("list tanpa koneksi aktif seharusnya gagal")
	}
	mustSaveConn(t, connUC)
	ps, err := profUC.List()
	if err != nil || len(ps) != 1 || ps[0].Name != "default" {
		t.Fatalf("list = %v %v", ps, err)
	}

	p, err := profUC.Create(CreateProfileInput{Name: "paket-5m", RateLimit: "5M/5M", SharedUsers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "paket-5m" || p.SharedUsers != 2 {
		t.Fatalf("create = %+v", p)
	}
	if _, err := profUC.Create(CreateProfileInput{}); err == nil {
		t.Fatal("create tanpa nama seharusnya gagal")
	}
}

func seedSiteVPS(t *testing.T, set *fakeSettings) {
	t.Helper()
	_ = set.Set(domain.SettingDeployMode, domain.DeployVPS)
	_ = set.Set(domain.SettingSiteURL, "https://wifi.contoh.com")
	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
}

func allStepsOK(steps []domain.ProvisionStep) bool {
	for _, s := range steps {
		if !s.OK {
			return false
		}
	}
	return true
}

func TestProvisioningApplyVPS(t *testing.T) {
	_, set, gw, connUC, provUC, _ := newStack()
	seedSiteVPS(t, set)
	mustSaveConn(t, connUC)
	// Entri lama milik mikbil harus diganti, entri lain dipertahankan.
	gw.sess.walledGarden = []domain.WalledGardenEntry{
		{ID: "a", DstHost: "lama.id", Comment: domain.RouterCommentMikbil},
		{ID: "b", DstHost: "lain.id", Comment: "manual"},
	}

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 || !allStepsOK(steps) {
		t.Fatalf("steps VPS = %+v", steps)
	}
	if steps[0].Step != "koneksi" || steps[3].Step != "verify" {
		t.Fatalf("urutan steps = %+v", steps)
	}
	if gw.sess.serverProfile[0].LoginBy != "chap,cookie,http-pap" {
		t.Fatalf("login-by = %q, want aditif http-pap", gw.sess.serverProfile[0].LoginBy)
	}
	foundNew, foundOld, foundManual := false, false, false
	for _, e := range gw.sess.walledGarden {
		switch e.DstHost {
		case "wifi.contoh.com":
			foundNew = true
		case "lama.id":
			foundOld = true
		case "lain.id":
			foundManual = true
		}
	}
	if !foundNew || foundOld || !foundManual {
		t.Fatalf("walled garden = %+v", gw.sess.walledGarden)
	}
}

func TestProvisioningApplyLokal(t *testing.T) {
	_, set, gw, connUC, provUC, _ := newStack()
	_ = set.Set(domain.SettingDeployMode, domain.DeployLocal)
	_ = set.Set(domain.SettingSiteURL, "http://mikbil.lan")
	_ = set.Set(domain.SettingLocalServerIP, "10.10.10.2")
	mustSaveConn(t, connUC)

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 || !allStepsOK(steps) {
		t.Fatalf("steps lokal = %+v", steps)
	}
	if steps[3].Step != "dns-statik" {
		t.Fatalf("langkah 4 = %q, want dns-statik", steps[3].Step)
	}
	if len(gw.sess.staticDNS) != 1 || gw.sess.staticDNS[0].Name != "mikbil.lan" {
		t.Fatalf("dns statik = %+v", gw.sess.staticDNS)
	}
	if !gw.sess.allowRemote {
		t.Fatal("allow-remote-requests seharusnya diubah ke yes")
	}
}

func TestProvisioningWalledGardenParsialTetapLanjut(t *testing.T) {
	_, set, gw, connUC, provUC, _ := newStack()
	_ = set.Set(domain.SettingDeployMode, domain.DeployLocal)
	_ = set.Set(domain.SettingSiteURL, "http://mikbil.lan")
	_ = set.Set(domain.SettingLocalServerIP, "10.10.10.2")
	mustSaveConn(t, connUC)
	gw.sess.failWGAddr = true

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("steps = %+v, want 5 langkah (dns+verify tetap jalan)", steps)
	}
	wg := steps[2]
	if wg.Step != "walled-garden" || wg.OK {
		t.Fatalf("walled-garden = %+v, want gagal", wg)
	}
	if !strings.Contains(wg.Message, "1 dari 2") || !strings.Contains(wg.Message, "dst-address") {
		t.Fatalf("pesan walled-garden = %q, want hitungan + error router", wg.Message)
	}
	if steps[3].Step != "dns-statik" || !steps[3].OK {
		t.Fatalf("dns-statik = %+v, want tetap jalan dan OK", steps[3])
	}
	if steps[4].Step != "verify" || steps[4].OK {
		t.Fatalf("verify = %+v, want jalan dan gagal (walled garden belum lengkap)", steps[4])
	}
}

func TestProvisioningDNSManualIdentikTidakDitulisUlang(t *testing.T) {
	_, set, gw, connUC, provUC, _ := newStack()
	_ = set.Set(domain.SettingDeployMode, domain.DeployLocal)
	_ = set.Set(domain.SettingSiteURL, "http://mikbil.lan")
	_ = set.Set(domain.SettingLocalServerIP, "10.10.10.2")
	mustSaveConn(t, connUC)
	gw.sess.staticDNS = []domain.StaticDNSEntry{
		{ID: "m", Name: "mikbil.lan", Address: "10.10.10.2", Comment: "manual"},
	}

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !allStepsOK(steps) {
		t.Fatalf("steps = %+v, want semua OK", steps)
	}
	for _, w := range gw.sess.writes {
		if strings.HasPrefix(w, "add-dns:") {
			t.Fatalf("tulis DNS duplikat: %v", gw.sess.writes)
		}
	}
	if len(gw.sess.staticDNS) != 1 {
		t.Fatalf("dns statik = %+v, want tetap 1 entri", gw.sess.staticDNS)
	}
}

func TestProvisioningKoneksiGagalBerhenti(t *testing.T) {
	_, set, gw, connUC, provUC, _ := newStack()
	seedSiteVPS(t, set)
	mustSaveConn(t, connUC)
	gw.failDial = true

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].OK || steps[0].Step != "koneksi" {
		t.Fatalf("steps gagal koneksi = %+v", steps)
	}
	if len(gw.sess.writes) != 0 {
		t.Fatalf("tulis saat koneksi gagal = %v", gw.sess.writes)
	}
}

func TestProvisioningStatusReadOnly(t *testing.T) {
	_, set, gw, connUC, provUC, allow := newStack()
	seedSiteVPS(t, set)
	mustSaveConn(t, connUC)
	if _, err := allow.Create("api.dana.id", true); err != nil {
		t.Fatal(err)
	}

	st, err := provUC.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Checks) != 4 {
		t.Fatalf("checks VPS = %+v", st.Checks)
	}
	if st.Checks[3].Name != "allowlist" || st.Checks[3].OK {
		t.Fatalf("allowlist sebelum apply = %+v, want gagal", st.Checks[3])
	}
	if st.Checks[1].OK {
		t.Fatal("http-pap seharusnya belum OK sebelum apply")
	}
	if len(gw.sess.writes) != 0 {
		t.Fatalf("status menulis ke router: %v", gw.sess.writes)
	}

	if _, err := provUC.Apply(); err != nil {
		t.Fatal(err)
	}
	st, err = provUC.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range st.Checks {
		if !c.OK {
			t.Fatalf("check setelah apply = %+v", st.Checks)
		}
	}
}

func wgHosts(sess *fakeSession) []string {
	var out []string
	for _, e := range sess.walledGarden {
		if e.DstHost != "" {
			out = append(out, e.DstHost)
		}
	}
	return out
}

func TestAllowlistCRUDSinkronRouter(t *testing.T) {
	_, set, gw, connUC, _, allow := newStack()
	seedSiteVPS(t, set)
	auc := NewAllowlistUsecase(allow, set, connUC)
	mustSaveConn(t, connUC)

	a, sync, err := auc.Create("API.DANA.id.", true)
	if err != nil {
		t.Fatal(err)
	}
	if a.DstHost != "api.dana.id" || !sync.Synced {
		t.Fatalf("create = %+v %+v, want normalisasi + sync", a, sync)
	}
	if hosts := wgHosts(gw.sess); !containsStr(hosts, "api.dana.id") {
		t.Fatalf("router tidak menerima entri: %v", hosts)
	}

	m, sync, err := auc.Create("m.bankmandiri.co.id", false)
	if err != nil || !sync.Synced {
		t.Fatalf("create nonaktif = %+v %+v %v", m, sync, err)
	}
	if hosts := wgHosts(gw.sess); containsStr(hosts, "m.bankmandiri.co.id") {
		t.Fatalf("host nonaktif tertulis ke router: %v", hosts)
	}

	if _, _, err := auc.Create("api.dana.id", true); err == nil {
		t.Fatal("duplikat seharusnya konflik")
	} else if _, ok := err.(ConflictError); !ok {
		t.Fatalf("duplikat = %T, want ConflictError", err)
	}
	for _, bad := range []string{"", "bukan domain!", "192.168.1.1", "wifi.contoh.com"} {
		if _, _, err := auc.Create(bad, true); err == nil {
			t.Fatalf("host %q seharusnya ditolak", bad)
		}
	}

	if _, sync, err := auc.Update(m.ID, "m.bankmandiri.co.id", true); err != nil || !sync.Synced {
		t.Fatalf("aktifkan = %+v %v", sync, err)
	}
	if hosts := wgHosts(gw.sess); !containsStr(hosts, "m.bankmandiri.co.id") {
		t.Fatalf("aktifkan tidak menulis router: %v", hosts)
	}

	if _, sync, err := auc.Update(a.ID, "api.dana.id", false); err != nil || !sync.Synced {
		t.Fatalf("disable = %+v %v", sync, err)
	}
	if hosts := wgHosts(gw.sess); containsStr(hosts, "api.dana.id") {
		t.Fatalf("disable tidak menghapus dari router: %v", hosts)
	}

	if sync, err := auc.Delete(m.ID); err != nil || !sync.Synced {
		t.Fatalf("hapus = %+v %v", sync, err)
	}
	if hosts := wgHosts(gw.sess); containsStr(hosts, "m.bankmandiri.co.id") {
		t.Fatalf("hapus tidak menghapus dari router: %v", hosts)
	}
	if _, err := auc.Delete(999); err == nil {
		t.Fatal("hapus id asing seharusnya gagal")
	}

	list, err := auc.List()
	if err != nil || len(list) != 1 || list[0].Enabled {
		t.Fatalf("list = %+v %v", list, err)
	}
}

func TestAllowlistSyncGagalTetapTersimpan(t *testing.T) {
	_, set, gw, connUC, _, allow := newStack()
	seedSiteVPS(t, set)
	auc := NewAllowlistUsecase(allow, set, connUC)
	mustSaveConn(t, connUC)
	gw.failDial = true

	a, sync, err := auc.Create("api.gopay.co.id", true)
	if err != nil {
		t.Fatal(err)
	}
	if sync.Synced || sync.Error == "" {
		t.Fatalf("sync gagal = %+v, want penanda gagal + pesan", sync)
	}
	if _, err := allow.GetByID(a.ID); err != nil {
		t.Fatalf("entri seharusnya tetap tersimpan: %v", err)
	}
}

func TestAllowlistBulk(t *testing.T) {
	_, set, gw, connUC, _, allow := newStack()
	seedSiteVPS(t, set)
	auc := NewAllowlistUsecase(allow, set, connUC)
	mustSaveConn(t, connUC)
	if _, err := allow.Create("api.dana.id", true); err != nil {
		t.Fatal(err)
	}

	res, err := auc.CreateBulk([]string{
		"API.DANA.id", "api.gopay.co.id", "api.gopay.co.id",
		"bukan domain", "wifi.contoh.com",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0].DstHost != "api.gopay.co.id" {
		t.Fatalf("added = %+v", res.Added)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "api.dana.id" {
		t.Fatalf("skipped = %v", res.Skipped)
	}
	if len(res.Failed) != 2 {
		t.Fatalf("failed = %+v", res.Failed)
	}
	if res.RouterError != "" {
		t.Fatalf("router_error = %q", res.RouterError)
	}
	hosts := wgHosts(gw.sess)
	if !containsStr(hosts, "api.gopay.co.id") || containsStr(hosts, "api.dana.id") {
		t.Fatalf("router = %v, want hanya host baru", hosts)
	}

	res, err = auc.CreateBulk([]string{"m.bankmandiri.co.id"}, false)
	if err != nil || len(res.Added) != 1 || res.RouterError != "" {
		t.Fatalf("bulk nonaktif = %+v %v", res, err)
	}
	if hosts := wgHosts(gw.sess); containsStr(hosts, "m.bankmandiri.co.id") {
		t.Fatalf("host nonaktif tertulis: %v", hosts)
	}

	many := make([]string, 101)
	for i := range many {
		many[i] = "host.example.id"
	}
	if _, err := auc.CreateBulk(many, true); err == nil {
		t.Fatal("bulk > 100 seharusnya ditolak")
	}
	if _, err := auc.CreateBulk(nil, true); err == nil {
		t.Fatal("bulk kosong seharusnya ditolak")
	}

	gw.failDial = true
	res, err = auc.CreateBulk([]string{"api.ovo.id"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.RouterError == "" {
		t.Fatalf("bulk router mati = %+v, want tersimpan + router_error", res)
	}
}

func TestProvisioningApplyTermasukAllowlist(t *testing.T) {
	_, set, gw, connUC, provUC, allow := newStack()
	seedSiteVPS(t, set)
	mustSaveConn(t, connUC)
	if _, err := allow.Create("api.dana.id", true); err != nil {
		t.Fatal(err)
	}
	if _, err := allow.Create("api.gopay.co.id", false); err != nil {
		t.Fatal(err)
	}

	steps, err := provUC.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !allStepsOK(steps) {
		t.Fatalf("steps = %+v", steps)
	}
	if !strings.Contains(steps[2].Message, "1 allowlist") {
		t.Fatalf("pesan walled-garden = %q, want hitungan allowlist", steps[2].Message)
	}
	hosts := wgHosts(gw.sess)
	if !containsStr(hosts, "api.dana.id") || containsStr(hosts, "api.gopay.co.id") {
		t.Fatalf("allowlist di router = %v", hosts)
	}
	st, err := provUC.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range st.Checks {
		if c.Name == "allowlist" && !c.OK {
			t.Fatalf("allowlist setelah apply = %+v", st.Checks)
		}
	}
}

func TestHotspotSnippetTemplate(t *testing.T) {
	set := newFakeSettings()
	hot := NewHotspotUsecase(set)
	if _, err := hot.ButtonSnippet(); err == nil {
		t.Fatal("snippet tanpa site_url seharusnya gagal")
	}
	_ = set.Set(domain.SettingSiteURL, "https://wifi.contoh.com")

	snip, err := hot.ButtonSnippet()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://wifi.contoh.com", "$(mac)", "$(ip)", "Beli Voucher WiFi"} {
		if !strings.Contains(snip, want) {
			t.Fatalf("snippet tidak memuat %q", want)
		}
	}

	tpl, err := hot.LoginTemplate()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://wifi.contoh.com", "$(chap-id)", "$(link-login-only)", "doVoucher", "Beli Voucher WiFi"} {
		if !strings.Contains(tpl, want) {
			t.Fatalf("template tidak memuat %q", want)
		}
	}
}
