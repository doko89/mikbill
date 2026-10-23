package mikrotik

import (
	"strconv"

	"github.com/mikbil/mikbil/internal/domain"
)

// ListUserProfiles membaca /ip/hotspot/user/profile/print.
func (s *Session) ListUserProfiles() ([]domain.HotspotProfile, error) {
	r, err := s.run("/ip/hotspot/user/profile/print")
	if err != nil {
		return nil, err
	}
	out := make([]domain.HotspotProfile, 0, len(r.Re))
	for _, m := range rows(r) {
		timeout := 0
		if v := m["session-timeout"]; v != "" {
			timeout = parseRouterOSTime(v)
		}
		shared := 0
		if v := m["shared-users"]; v != "" {
			shared, _ = strconv.Atoi(v)
		}
		out = append(out, domain.HotspotProfile{
			Name:           m["name"],
			RateLimit:      m["rate-limit"],
			SessionTimeout: timeout,
			SharedUsers:    shared,
		})
	}
	return out, nil
}

// CreateUserProfile menjalankan /ip/hotspot/user/profile/add.
func (s *Session) CreateUserProfile(p domain.HotspotProfile) error {
	args := []string{"/ip/hotspot/user/profile/add", "=name=" + p.Name}
	if p.RateLimit != "" {
		args = append(args, "=rate-limit="+p.RateLimit)
	}
	if p.SessionTimeout > 0 {
		args = append(args, "=session-timeout="+strconv.Itoa(p.SessionTimeout))
	}
	if p.SharedUsers > 0 {
		args = append(args, "=shared-users="+strconv.Itoa(p.SharedUsers))
	}
	args = append(args, "=idle-timeout=5m")
	_, err := s.run(args...)
	return err
}

// parseRouterOSTime membaca durasi RouterOS ("1h", "12:00:00", "7d 00:00:00", detik polos).
func parseRouterOSTime(v string) int {
	secs, ok := parseDuration(v)
	if !ok {
		return 0
	}
	return secs
}
