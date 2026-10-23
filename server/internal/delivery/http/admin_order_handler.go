package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mikbil/mikbil/internal/usecase"
)

// AdminOrderHandler menangani API transaksi admin (tech_spec 4.4).
type AdminOrderHandler struct {
	admin *usecase.AdminOrderUsecase
}

// NewAdminOrderHandler membuat AdminOrderHandler.
func NewAdminOrderHandler(admin *usecase.AdminOrderUsecase) *AdminOrderHandler {
	return &AdminOrderHandler{admin: admin}
}

func adminOrderItemDTO(it usecase.AdminOrderItem) map[string]any {
	o := it.Order
	return map[string]any{
		"order_id": o.ID, "status": o.Status,
		"package_id": o.PackageID, "package_name": it.PackageName,
		"base_amount": o.BaseAmount, "unique_code": o.UniqueCode,
		"total_amount":        o.TotalAmount,
		"kriskris_invoice_id": o.KrisKrisInvoice,
		"expires_at":          fmtTime(o.ExpiresAt),
		"paid_at":             fmtTime(o.PaidAt),
		"failed_reason":       o.FailedReason,
		"created_at":          o.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// List menangani GET /api/admin/orders?status=&q=&page=.
func (h *AdminOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			WriteErr(w, http.StatusBadRequest, "page tidak valid")
			return
		}
		page = n
	}
	res, err := h.admin.List(
		r.URL.Query().Get("status"), r.URL.Query().Get("q"), page)
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, adminOrderItemDTO(it))
	}
	WriteOK(w, http.StatusOK, map[string]any{
		"items": items, "page": res.Page,
		"per_page": res.PerPage, "total": res.Total,
	})
}

// Get menangani GET /api/admin/orders/:id.
func (h *AdminOrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.admin.Detail(chi.URLParam(r, "id"))
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := adminOrderItemDTO(usecase.AdminOrderItem{Order: d.Order, PackageName: d.Package.Name})
	out["package"] = packageDTO(d.Package)
	if d.Voucher != nil {
		out["voucher"] = map[string]any{
			"username": d.Voucher.Username, "password": d.Voucher.Password,
			"profile_name": d.Voucher.ProfileName,
		}
		out["login_url"] = d.LoginURL
	}
	WriteOK(w, http.StatusOK, out)
}

// Retry menangani POST /api/admin/orders/:id/retry.
func (h *AdminOrderHandler) Retry(w http.ResponseWriter, r *http.Request) {
	d, err := h.admin.Retry(chi.URLParam(r, "id"))
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := adminOrderItemDTO(usecase.AdminOrderItem{Order: d.Order, PackageName: d.Package.Name})
	if d.Voucher != nil {
		out["voucher"] = map[string]any{
			"username": d.Voucher.Username, "password": d.Voucher.Password,
			"profile_name": d.Voucher.ProfileName,
		}
		out["login_url"] = d.LoginURL
	}
	WriteOK(w, http.StatusOK, out)
}

// Summary menangani GET /api/admin/orders/summary.
func (h *AdminOrderHandler) Summary(w http.ResponseWriter, _ *http.Request) {
	s, err := h.admin.Summary()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	recent := make([]map[string]any, 0, len(s.Recent))
	for _, it := range s.Recent {
		recent = append(recent, adminOrderItemDTO(it))
	}
	WriteOK(w, http.StatusOK, map[string]any{
		"revenue_today": s.RevenueToday, "pending_count": s.PendingCount,
		"failed_count": s.FailedCount, "recent": recent,
	})
}
