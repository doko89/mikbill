package domain

// Konstanta komentar yang dipakai Mikbil di router.
const (
	// RouterCommentMikbil menandai entri walled garden / DNS statik milik Mikbil.
	RouterCommentMikbil = "mikbil"
	// OrderCommentPrefix mengawali comment user hotspot per order ("mikbil:<order_id>").
	OrderCommentPrefix = "mikbil:"
)

// HotspotProfile adalah user profile di router (/ip/hotspot/user/profile).
type HotspotProfile struct {
	Name           string
	RateLimit      string // cth. "5M/10M", kosong = tanpa batas
	SessionTimeout int    // detik, 0 = tanpa batas
	SharedUsers    int
}

// HotspotUser adalah akun voucher di router (/ip/hotspot/user).
type HotspotUser struct {
	Name        string
	Password    string
	Profile     string
	LimitUptime string // format RouterOS, cth. "12:00:00" atau "7d 00:00:00"
	Comment     string // "mikbil:<order_id>"
}

// HotspotServerProfile adalah server profile (/ip/hotspot/profile).
type HotspotServerProfile struct {
	ID      string
	Name    string
	LoginBy string // daftar comma-separated: chap,http-pap,cookie,...
}

// WalledGardenEntry adalah entri walled garden (/ip/hotspot/walled-garden).
type WalledGardenEntry struct {
	ID         string
	DstHost    string
	DstAddress string
	Action     string
	Comment    string
	Disabled   bool
}

// StaticDNSEntry adalah entri DNS statik (/ip/dns/static).
type StaticDNSEntry struct {
	ID      string
	Name    string
	Address string
	Comment string
}

// RouterResource adalah info singkat router dari /system/resource.
type RouterResource struct {
	BoardName string
	Version   string
}

// ProvisionStep adalah hasil satu langkah provisioning.
type ProvisionStep struct {
	Step    string `json:"step"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ProvisionCheck adalah satu butir pemeriksaan status provisioning.
type ProvisionCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ProvisionStatus adalah hasil verify read-only.
type ProvisionStatus struct {
	DeployMode string           `json:"deploy_mode"`
	SiteURL    string           `json:"site_url"`
	Checks     []ProvisionCheck `json:"checks"`
}
