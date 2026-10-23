package usecase

import (
	"errors"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// RouterLastTest adalah hasil uji koneksi router terakhir.
type RouterLastTest struct {
	At      *time.Time
	OK      *bool
	Message string
}

// HealthStatus adalah status kesehatan instance.
type HealthStatus struct {
	Time   time.Time
	DBOK   bool
	Skew   *float64
	Router *RouterLastTest
}

// HealthUsecase menyusun status health dari database, clock skew, dan router.
type HealthUsecase struct {
	db    DBPinger
	conns ConnectionRepository
	skew  SkewReader
	now   func() time.Time
}

// NewHealthUsecase membuat HealthUsecase.
func NewHealthUsecase(db DBPinger, conns ConnectionRepository, skew SkewReader) *HealthUsecase {
	return &HealthUsecase{db: db, conns: conns, skew: skew, now: time.Now}
}

// Status membaca semua probe tanpa menulis apa pun.
func (u *HealthUsecase) Status() HealthStatus {
	st := HealthStatus{Time: u.now().UTC()}
	st.DBOK = u.db.Ping() == nil
	if s, ok := u.skew.ClockSkewSec(); ok {
		st.Skew = &s
	}
	c, err := u.conns.GetActive()
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return st
		}
		return st
	}
	st.Router = &RouterLastTest{At: c.LastTestAt, OK: c.LastTestOK, Message: c.LastTestMsg}
	return st
}
