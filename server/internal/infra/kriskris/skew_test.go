package kriskris

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestSkewRecorderNol(t *testing.T) {
	var r SkewRecorder
	if _, ok := r.Skew(); ok {
		t.Fatal("perekam kosong seharusnya ok=false")
	}
	r.Observe(3 * time.Second)
	d, ok := r.Skew()
	if !ok || d != 3*time.Second {
		t.Fatalf("skew = %v %v, want 3s true", d, ok)
	}
}

func TestClientSharedMenerimaSkew(t *testing.T) {
	srv := fakeServer(t, "rahasia", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true,
			"data": map[string]any{"id": "inv_1", "status": "cancelled"}})
	})
	defer srv.Close()
	rec := &SkewRecorder{}
	c := testClient(srv)
	c.Shared = rec
	if err := c.CancelInvoice(context.Background(), "inv_1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.Skew(); !ok {
		t.Fatal("perekam bersama tidak menerima sampel skew")
	}
}
