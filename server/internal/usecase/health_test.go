package usecase

import (
	"errors"
	"testing"

	"github.com/mikbil/mikbil/internal/domain"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping() error { return f.err }

type fakeSkew struct {
	sec float64
	ok  bool
}

func (f fakeSkew) ClockSkewSec() (float64, bool) { return f.sec, f.ok }

func TestHealthTanpaKoneksi(t *testing.T) {
	u := NewHealthUsecase(fakePinger{}, &fakeConns{}, fakeSkew{})
	st := u.Status()
	if st.Time.IsZero() {
		t.Fatal("waktu kosong")
	}
	if !st.DBOK {
		t.Fatal("DBOK seharusnya true")
	}
	if st.Skew != nil || st.Router != nil {
		t.Fatalf("skew/router = %v/%v, want nil", st.Skew, st.Router)
	}
}

func TestHealthLengkap(t *testing.T) {
	conns := newFakeConns()
	c, err := conns.CreateActive(domain.Connection{Name: "default", Host: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := conns.SaveTestResult(c.ID, true, "koneksi router berhasil"); err != nil {
		t.Fatal(err)
	}
	u := NewHealthUsecase(fakePinger{}, conns, fakeSkew{sec: -12.5, ok: true})
	st := u.Status()
	if st.Skew == nil || *st.Skew != -12.5 {
		t.Fatalf("skew = %v, want -12.5", st.Skew)
	}
	if st.Router == nil || st.Router.OK == nil || !*st.Router.OK {
		t.Fatalf("router = %+v, want ok true", st.Router)
	}
	if st.Router.Message != "koneksi router berhasil" {
		t.Fatalf("router = %+v, want message", st.Router)
	}
}

func TestHealthDBGagal(t *testing.T) {
	u := NewHealthUsecase(fakePinger{err: errors.New("tutup")}, &fakeConns{}, fakeSkew{})
	if st := u.Status(); st.DBOK {
		t.Fatal("DBOK seharusnya false")
	}
}
