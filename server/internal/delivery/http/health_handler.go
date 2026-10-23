package http

import (
	"net/http"
	"time"

	"github.com/mikbil/mikbil/internal/usecase"
)

// HealthHandler menangani GET /api/health (tech_spec 4.1).
type HealthHandler struct {
	health *usecase.HealthUsecase
}

// NewHealthHandler membuat HealthHandler.
func NewHealthHandler(health *usecase.HealthUsecase) *HealthHandler {
	return &HealthHandler{health: health}
}

func fmtTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// Get mengembalikan waktu server, status DB, clock skew, dan uji router terakhir.
func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	st := h.health.Status()
	var skew any
	if st.Skew != nil {
		skew = *st.Skew
	}
	var router any
	if st.Router != nil {
		var ok any
		if st.Router.OK != nil {
			ok = *st.Router.OK
		}
		router = map[string]any{
			"at":      fmtTimePtr(st.Router.At),
			"ok":      ok,
			"message": st.Router.Message,
		}
	}
	db := "ok"
	status := http.StatusOK
	if !st.DBOK {
		db = "gagal"
		status = http.StatusServiceUnavailable
	}
	WriteOK(w, status, map[string]any{
		"time": st.Time.Format(time.RFC3339),
		"db":   db, "clock_skew_sec": skew, "router_last_test": router,
	})
}
