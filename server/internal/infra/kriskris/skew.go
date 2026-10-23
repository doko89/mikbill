package kriskris

import (
	"sync"
	"time"
)

// SkewRecorder menyimpan sampel clock skew terakhir dari header Date
// respons KrisKris. Dipakai bersama oleh semua Client agar health-check
// bisa membaca selisih jam walau tiap request memakai Client baru.
type SkewRecorder struct {
	mu   sync.Mutex
	skew time.Duration
	ok   bool
}

// Observe mencatat satu sampel skew.
func (r *SkewRecorder) Observe(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skew = d
	r.ok = true
}

// Skew mengembalikan sampel terakhir; ok=false bila belum ada sampel.
func (r *SkewRecorder) Skew() (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.skew, r.ok
}
