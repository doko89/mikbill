package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/mikbil/mikbil/internal/usecase"
)

// Batas body webhook 64KB (tech_spec 10).
const webhookMaxBody = 64 << 10

// WebhookHandler menangani callback KrisKris (tech_spec 4.2).
type WebhookHandler struct {
	pay *usecase.PaymentUsecase
}

// NewWebhookHandler membuat WebhookHandler.
func NewWebhookHandler(pay *usecase.PaymentUsecase) *WebhookHandler {
	return &WebhookHandler{pay: pay}
}

// KrisKris menangani POST /api/webhooks/kriskris. Selalu balas 200 cepat
// (< 10 detik) bila signature valid; pemrosesan voucher berjalan async.
func (h *WebhookHandler) KrisKris(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookMaxBody))
	if err != nil {
		WriteErr(w, http.StatusBadRequest, "isi callback tidak valid")
		return
	}
	orderID, err := h.pay.HandleWebhook(
		raw,
		r.Header.Get("X-Kriskris-Event"),
		r.Header.Get("X-Kriskris-Delivery"),
		r.Header.Get("X-Kriskris-Signature"),
	)
	if err != nil {
		var unauth usecase.UnauthorizedError
		if errors.As(err, &unauth) {
			WriteErr(w, http.StatusUnauthorized, unauth.Msg)
			return
		}
		writeUsecaseErr(w, err)
		return
	}
	if orderID != "" {
		go func() { _ = h.pay.ProcessPaidOrder(orderID) }()
	}
	WriteOK(w, http.StatusOK, map[string]bool{"ok": true})
}
