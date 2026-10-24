package mikrotik

import (
	"errors"
	"strings"

	"github.com/mikbil/mikbil/internal/domain"
)

// parseAllowRemoteRequests menafsirkan nilai allow-remote-requests dari
// /ip/dns/print. RouterOS v6 mengembalikan "yes"/"no", v7 "true"/"false".
func parseAllowRemoteRequests(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1":
		return true
	default:
		return false
	}
}

// ListStaticDNS membaca /ip/dns/static/print.
func (s *Session) ListStaticDNS() ([]domain.StaticDNSEntry, error) {
	r, err := s.run("/ip/dns/static/print")
	if err != nil {
		return nil, err
	}
	out := make([]domain.StaticDNSEntry, 0, len(r.Re))
	for _, m := range rows(r) {
		out = append(out, domain.StaticDNSEntry{
			ID:      m[".id"],
			Name:    m["name"],
			Address: m["address"],
			Comment: m["comment"],
		})
	}
	return out, nil
}

// AddStaticDNS menjalankan /ip/dns/static/add.
func (s *Session) AddStaticDNS(e domain.StaticDNSEntry) error {
	_, err := s.run("/ip/dns/static/add",
		"=name="+e.Name, "=address="+e.Address,
		"=comment="+domain.RouterCommentMikbil)
	return err
}

// RemoveStaticDNS menjalankan /ip/dns/static/remove.
func (s *Session) RemoveStaticDNS(id string) error {
	_, err := s.run("/ip/dns/static/remove", "=.id="+id)
	return err
}

// DNSAllowRemoteRequests membaca /ip/dns/print allow-remote-requests.
func (s *Session) DNSAllowRemoteRequests() (bool, error) {
	r, err := s.run("/ip/dns/print")
	if err != nil {
		return false, err
	}
	if len(r.Re) == 0 {
		return false, errors.New("/ip/dns/print tidak mengembalikan data")
	}
	return parseAllowRemoteRequests(r.Re[0].Map["allow-remote-requests"]), nil
}

// SetDNSAllowRemoteRequests menjalankan /ip/dns/set allow-remote-requests=.
func (s *Session) SetDNSAllowRemoteRequests(allow bool) error {
	v := "no"
	if allow {
		v = "yes"
	}
	_, err := s.run("/ip/dns/set", "=allow-remote-requests="+v)
	return err
}
