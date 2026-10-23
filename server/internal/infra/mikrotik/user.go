package mikrotik

import (
	"strconv"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// AddHotspotUser menjalankan /ip/hotspot/user/add.
func (s *Session) AddHotspotUser(u domain.HotspotUser) error {
	args := []string{
		"/ip/hotspot/user/add",
		"=name=" + u.Name,
		"=password=" + u.Password,
		"=profile=" + u.Profile,
	}
	if u.LimitUptime != "" {
		args = append(args, "=limit-uptime="+u.LimitUptime)
	}
	if u.Comment != "" {
		args = append(args, "=comment="+u.Comment)
	}
	_, err := s.run(args...)
	return err
}

// FindHotspotUserByComment mencari user via ?comment= (untuk idempotency voucher).
func (s *Session) FindHotspotUserByComment(comment string) (domain.HotspotUser, bool, error) {
	r, err := s.run("/ip/hotspot/user/print", "?comment="+comment)
	if err != nil {
		return domain.HotspotUser{}, false, err
	}
	if len(r.Re) == 0 {
		return domain.HotspotUser{}, false, nil
	}
	m := r.Re[0].Map
	return domain.HotspotUser{
		Name:        m["name"],
		Profile:     m["profile"],
		LimitUptime: m["limit-uptime"],
		Comment:     m["comment"],
	}, true, nil
}

// parseDuration membaca durasi RouterOS menjadi detik.
func parseDuration(v string) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return secs, true
	}
	total := 0
	rest := v
	if i := strings.Index(rest, "d"); i > 0 {
		if d, err := strconv.Atoi(strings.TrimSpace(rest[:i])); err == nil {
			total += d * 86400
			rest = strings.TrimSpace(rest[i+1:])
			if rest == "" {
				return total, true
			}
		}
	}
	if rest == "" {
		return total, true
	}
	if strings.Contains(rest, ":") {
		parts := strings.Split(rest, ":")
		if len(parts) > 3 {
			return 0, false
		}
		mult := 1
		for i := len(parts) - 1; i >= 0; i-- {
			n, err := strconv.Atoi(parts[i])
			if err != nil {
				return 0, false
			}
			total += n * mult
			mult *= 60
		}
		return total, true
	}
	unit := map[byte]int{'w': 604800, 'd': 86400, 'h': 3600, 'm': 60, 's': 1}
	num := ""
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c >= '0' && c <= '9' {
			num += string(c)
			continue
		}
		m, ok := unit[c]
		if !ok || num == "" {
			return 0, false
		}
		n, _ := strconv.Atoi(num)
		total += n * m
		num = ""
	}
	if num != "" {
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, false
		}
		total += n
	}
	return total, true
}
