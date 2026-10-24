package mikrotik

import "github.com/mikbil/mikbil/internal/domain"

// ListWalledGarden membaca kedua menu: /ip/hotspot/walled-garden (dst-host)
// dan /ip/hotspot/walled-garden/ip (dst-address).
func (s *Session) ListWalledGarden() ([]domain.WalledGardenEntry, error) {
	out := []domain.WalledGardenEntry{}
	r, err := s.run("/ip/hotspot/walled-garden/print")
	if err != nil {
		return nil, err
	}
	for _, m := range rows(r) {
		out = append(out, domain.WalledGardenEntry{
			ID:       m[".id"],
			DstHost:  m["dst-host"],
			Action:   m["action"],
			Comment:  m["comment"],
			Disabled: m["disabled"] == "true",
		})
	}
	r, err = s.run("/ip/hotspot/walled-garden/ip/print")
	if err != nil {
		return nil, err
	}
	for _, m := range rows(r) {
		out = append(out, domain.WalledGardenEntry{
			ID:         m[".id"],
			DstAddress: m["dst-address"],
			Action:     m["action"],
			Comment:    m["comment"],
			Disabled:   m["disabled"] == "true",
		})
	}
	return out, nil
}

// AddWalledGarden menulis ke menu yang tepat: dst-address ke
// /ip/hotspot/walled-garden/ip (action=accept), dst-host ke
// /ip/hotspot/walled-garden (action=allow).
func (s *Session) AddWalledGarden(e domain.WalledGardenEntry) error {
	if e.DstAddress != "" {
		_, err := s.run("/ip/hotspot/walled-garden/ip/add",
			"=action=accept",
			"=dst-address="+e.DstAddress,
			"=comment="+domain.RouterCommentMikbil)
		return err
	}
	_, err := s.run("/ip/hotspot/walled-garden/add",
		"=action=allow",
		"=dst-host="+e.DstHost,
		"=comment="+domain.RouterCommentMikbil)
	return err
}

// RemoveWalledGarden menghapus dari menu host dulu, lalu menu IP bila gagal.
func (s *Session) RemoveWalledGarden(id string) error {
	if _, err := s.run("/ip/hotspot/walled-garden/remove", "=.id="+id); err == nil {
		return nil
	}
	_, err := s.run("/ip/hotspot/walled-garden/ip/remove", "=.id="+id)
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
