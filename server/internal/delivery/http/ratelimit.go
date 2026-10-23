package http

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Batas rate-limit sesuai tech_spec bagian 10 (per jendela 1 menit).
const rateWindow = time.Minute

// window adalah satu jendela hitung tetap.
type window struct {
	count int
	reset time.Time
}

// rateLimiter adalah pembatas laju fixed-window dalam memori.
type rateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	buckets map[string]*window
	calls   int
}

// newRateLimiter membuat rateLimiter.
func newRateLimiter() *rateLimiter {
	return &rateLimiter{now: time.Now, buckets: map[string]*window{}}
}

// allow mencatat satu hit untuk kunci dan melaporkan apakah masih di bawah batas.
// retryAfter adalah sisa jendela bila ditolak.
func (l *rateLimiter) allow(key string, limit int, per time.Duration) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	// Sapu entri kedaluwarsa secara oportunistik agar map tidak membengkak.
	if l.calls%256 == 0 {
		for k, w := range l.buckets {
			if !now.Before(w.reset) {
				delete(l.buckets, k)
			}
		}
	}
	w, ok := l.buckets[key]
	if !ok || !now.Before(w.reset) {
		w = &window{reset: now.Add(per)}
		l.buckets[key] = w
	}
	if w.count >= limit {
		return false, time.Until(w.reset)
	}
	w.count++
	return true, 0
}

// clientIP mengambil IP klien dari RemoteAddr (sudah dinormalisasi
// middleware RealIP: X-Forwarded-For bila di belakang reverse proxy).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipKey adalah kunci rate-limit per IP.
func ipKey(r *http.Request) string { return clientIP(r) }

// ipPathKey adalah kunci rate-limit per IP + path (untuk get-order per order).
func ipPathKey(r *http.Request) string { return clientIP(r) + "|" + r.URL.Path }

// limit mengembalikan middleware yang menolak request berlebih dengan 429 berenvelope.
func (l *rateLimiter) limit(limit int, per time.Duration, key func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ok, wait := l.allow(key(r), limit, per); !ok {
				secs := int(wait.Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				WriteErrCode(w, http.StatusTooManyRequests,
					"terlalu banyak permintaan, coba lagi sebentar", "rate_limited")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
