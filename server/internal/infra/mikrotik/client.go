// Package mikrotik adalah client RouterOS API klasik (kompatibel v6 dan v7).
package mikrotik

import (
	"crypto/tls"
	"net"
	"strconv"
	"time"

	"github.com/go-routeros/routeros"
	"github.com/mikbil/mikbil/internal/domain"
)

// Timeout adalah batas waktu dial, login, dan tiap perintah ke router.
const Timeout = 10 * time.Second

// DialParams adalah parameter koneksi ke router.
type DialParams struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
}

// Gateway membuka sesi RouterOS API.
type Gateway struct{}

// Dial membuka koneksi TCP/TLS lalu login. Timeout 10 detik.
func (Gateway) Dial(p DialParams) (*Session, error) {
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	if p.UseTLS {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: Timeout}, "tcp", addr, &tls.Config{
			ServerName:         p.Host,
			InsecureSkipVerify: true, // router umumnya memakai sertifikat self-signed
		})
		if err != nil {
			return nil, err
		}
		return login(conn, p.Username, p.Password)
	}
	conn, err := net.DialTimeout("tcp", addr, Timeout)
	if err != nil {
		return nil, err
	}
	return login(conn, p.Username, p.Password)
}

func login(conn net.Conn, username, password string) (*Session, error) {
	if err := conn.SetDeadline(time.Now().Add(Timeout)); err != nil {
		conn.Close()
		return nil, err
	}
	c, err := routeros.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := c.Login(username, password); err != nil {
		conn.Close()
		return nil, err
	}
	return &Session{client: c, conn: conn}, nil
}

// Session adalah sesi terautentikasi ke satu router.
type Session struct {
	client *routeros.Client
	conn   net.Conn
}

// Close menutup koneksi ke router.
func (s *Session) Close() error {
	s.client.Close()
	return nil
}

// run menjalankan satu perintah API dengan deadline 10 detik.
func (s *Session) run(sentence ...string) (*routeros.Reply, error) {
	if err := s.conn.SetDeadline(time.Now().Add(Timeout)); err != nil {
		return nil, err
	}
	return s.client.Run(sentence...)
}

// Test membaca /system/resource (fallback /ip/hotspot/print).
func (s *Session) Test() (domain.RouterResource, error) {
	var res domain.RouterResource
	r, err := s.run("/system/resource/print")
	if err == nil && len(r.Re) > 0 {
		res.BoardName = r.Re[0].Map["board-name"]
		res.Version = r.Re[0].Map["version"]
		return res, nil
	}
	if _, ferr := s.run("/ip/hotspot/print"); ferr != nil {
		if err != nil {
			return res, err
		}
		return res, ferr
	}
	return res, nil
}

// rows memetakan baris reply menjadi slice map.
func rows(r *routeros.Reply) []map[string]string {
	out := make([]map[string]string, 0, len(r.Re))
	for _, sen := range r.Re {
		out = append(out, sen.Map)
	}
	return out
}
