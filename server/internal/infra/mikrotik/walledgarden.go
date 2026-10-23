package mikrotik

import "github.com/mikbil/mikbil/internal/domain"

// ListWalledGarden membaca /ip/hotspot/walled-garden/print.
func (s *Session) ListWalledGarden() ([]domain.WalledGardenEntry, error) {
	r, err := s.run("/ip/hotspot/walled-garden/print")
	if err != nil {
		return nil, err
	}
	out := make([]domain.WalledGardenEntry, 0, len(r.Re))
	for _, m := range rows(r) {
		out = append(out, domain.WalledGardenEntry{
			ID:         m[".id"],
			DstHost:    m["dst-host"],
			DstAddress: m["dst-address"],
			Action:     m["action"],
			Comment:    m["comment"],
			Disabled:   m["disabled"] == "true",
		})
	}
	return out, nil
}

// AddWalledGarden menjalankan /ip/hotspot/walled-garden/add.
func (s *Session) AddWalledGarden(e domain.WalledGardenEntry) error {
	args := []string{"/ip/hotspot/walled-garden/add", "=action=allow"}
	if e.DstHost != "" {
		args = append(args, "=dst-host="+e.DstHost)
	}
	if e.DstAddress != "" {
		args = append(args, "=dst-address="+e.DstAddress)
	}
	args = append(args, "=comment="+domain.RouterCommentMikbil)
	_, err := s.run(args...)
	return err
}

// RemoveWalledGarden menjalankan /ip/hotspot/walled-garden/remove.
func (s *Session) RemoveWalledGarden(id string) error {
	_, err := s.run("/ip/hotspot/walled-garden/remove", "=.id="+id)
	return err
}

// ListServerProfiles membaca /ip/hotspot/profile/print.
func (s *Session) ListServerProfiles() ([]domain.HotspotServerProfile, error) {
	r, err := s.run("/ip/hotspot/profile/print")
	if err != nil {
		return nil, err
	}
	out := make([]domain.HotspotServerProfile, 0, len(r.Re))
	for _, m := range rows(r) {
		out = append(out, domain.HotspotServerProfile{
			ID:      m[".id"],
			Name:    m["name"],
			LoginBy: m["login-by"],
		})
	}
	return out, nil
}

// SetServerProfileLoginBy menjalankan /ip/hotspot/profile/set.
func (s *Session) SetServerProfileLoginBy(id, loginBy string) error {
	_, err := s.run("/ip/hotspot/profile/set", "=.id="+id, "=login-by="+loginBy)
	return err
}
