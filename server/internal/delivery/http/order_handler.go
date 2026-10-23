package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/usecase"
)

// OrderHandler menangani API pesanan public (tech_spec 4.1).
type OrderHandler struct {
	orders *usecase.OrderUsecase
}

// NewOrderHandler membuat OrderHandler.
func NewOrderHandler(orders *usecase.OrderUsecase) *OrderHandler {
	return &OrderHandler{orders: orders}
}

func fmtTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

type createOrderBody struct {
	PackageID int64 `json:"package_id"`
}

// Create menangani POST /api/orders.
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b createOrderBody
	if !decodeJSON(w, r, &b) {
		return
	}
	if b.PackageID <= 0 {
		WriteErr(w, http.StatusBadRequest, "package_id wajib diisi")
		return
	}
	res, err := h.orders.Create(r.Context(), b.PackageID)
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	o := res.Order
	WriteOK(w, http.StatusCreated, map[string]any{
		"order_id": o.ID, "qr_payload": o.QRPayload,
		"base_amount": o.BaseAmount, "unique_code": o.UniqueCode,
		"total_amount": o.TotalAmount,
		"expires_at":   res.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// Get menangani GET /api/orders/:id.
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.orders.Detail(chi.URLParam(r, "id"))
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := map[string]any{
		"order_id":     d.Order.ID,
		"status":       d.Order.Status,
		"package":      publicPackageDTO(d.Package),
		"total_amount": d.Order.TotalAmount,
		"expires_at":   fmtTime(d.Order.ExpiresAt),
		"paid_at":      fmtTime(d.Order.PaidAt),
	}
	// qr_payload hanya saat pending agar halaman bayar tahan refresh.
	if d.Order.Status == domain.OrderPending {
		out["qr_payload"] = d.Order.QRPayload
	}
	if d.Voucher != nil {
		out["voucher"] = map[string]any{
			"username": d.Voucher.Username, "password": d.Voucher.Password,
			"profile_name": d.Voucher.ProfileName,
		}
		out["login_url"] = d.LoginURL
	}
	WriteOK(w, http.StatusOK, out)
}

// Cancel menangani POST /api/orders/:id/cancel.
func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	o, err := h.orders.Cancel(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]any{"status": o.Status})
}
