package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// WebhookVerifyFunc memverifikasi signature callback (implementasi: infra/kriskris).
type WebhookVerifyFunc func(secret string, body []byte, signature string) bool

// Ukuran batch poller (tech_spec 6): maks 50 order per siklus, 1 request.
const pollBatchSize = 50

// PaymentUsecase menangani webhook, poller, dan penerbitan voucher.
type PaymentUsecase struct {
	orders   OrderRepository
	vouchers VoucherRepository
	webhooks WebhookRepository
	audits   AuditRepository
	pkgs     PackageRepository
	set      SettingsRepository
	box      SecretBox
	conns    *ConnectionUsecase
	invoices InvoiceClientFactory
	verify   WebhookVerifyFunc

	// RandString membuat kredensial voucher; diisi main (infra/crypto).
	RandString func(n int) (string, error)
	// Now jam; bisa diganti di test.
	Now func() time.Time
}

// NewPaymentUsecase membuat PaymentUsecase.
func NewPaymentUsecase(orders OrderRepository, vouchers VoucherRepository,
	webhooks WebhookRepository, audits AuditRepository, pkgs PackageRepository,
	set SettingsRepository, box SecretBox, conns *ConnectionUsecase,
	invoices InvoiceClientFactory, verify WebhookVerifyFunc) *PaymentUsecase {
	return &PaymentUsecase{
		orders: orders, vouchers: vouchers, webhooks: webhooks, audits: audits,
		pkgs: pkgs, set: set, box: box, conns: conns,
		invoices: invoices, verify: verify, Now: time.Now,
	}
}

// webhookBody adalah body callback KrisKris (tech_spec 4.2).
type webhookBody struct {
	Event      string `json:"event"`
	DeliveryID string `json:"delivery_id"`
	InvoiceID  string `json:"invoice_id"`
	Status     string `json:"status"`
	PaidAt     string `json:"paid_at"`
}

// HandleWebhook memproses satu callback. Signature salah berarti UnauthorizedError.
// Sukses mengembalikan orderID yang harus diproses async ("" bila tidak ada).
func (u *PaymentUsecase) HandleWebhook(rawBody []byte, event, deliveryID, signature string) (string, error) {
	enc, found, err := u.set.Get(domain.SettingKrisKrisCallbackSec)
	if err != nil || !found {
		return "", errors.New("callback secret belum dikonfigurasi")
	}
	secret, err := u.box.Open(enc)
	if err != nil {
		return "", err
	}
	if u.verify == nil || !u.verify(secret, rawBody, signature) {
		return "", UnauthorizedError{"signature callback tidak valid"}
	}
	if deliveryID == "" {
		var b webhookBody
		if json.Unmarshal(rawBody, &b) == nil && b.DeliveryID != "" {
			deliveryID = b.DeliveryID
		}
	}
	if event == "" {
		var b webhookBody
		if json.Unmarshal(rawBody, &b) == nil {
			event = b.Event
		}
	}
	var b webhookBody
	_ = json.Unmarshal(rawBody, &b)
	if deliveryID == "" {
		deliveryID = "tanpa-id-" + strconv.FormatInt(u.Now().UnixNano(), 10)
	}
	if event == domain.EventTestPing {
		_ = u.webhooks.Record(domain.WebhookDelivery{
			DeliveryID: deliveryID, Event: event, InvoiceID: b.InvoiceID,
		})
		return "", nil
	}
	if event != domain.EventInvoicePaid && event != domain.EventInvoiceExpired {
		_ = u.webhooks.Record(domain.WebhookDelivery{
			DeliveryID: deliveryID, Event: event, InvoiceID: b.InvoiceID,
		})
		return "", nil
	}
	dup, err := u.webhooks.Exists(deliveryID)
	if err != nil {
		return "", err
	}
	if dup {
		return "", nil
	}
	if err := u.webhooks.Record(domain.WebhookDelivery{
		DeliveryID: deliveryID, Event: event, InvoiceID: b.InvoiceID,
	}); err != nil {
		return "", err
	}
	o, err := u.orders.GetByInvoice(b.InvoiceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if event == domain.EventInvoiceExpired {
		if o.Status == domain.OrderPending {
			_ = u.orders.MarkExpired(o.ID)
		}
		return "", nil
	}
	return o.ID, nil
}

// ProcessPaidOrder menerbitkan voucher untuk order lunas. IDEMPOTEN dan
// merupakan SATU-SATUNYA pintu pembuatan voucher.
func (u *PaymentUsecase) ProcessPaidOrder(orderID string) error {
	o, err := u.orders.GetByID(orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if o.Status == domain.OrderPaid {
		return nil
	}
	if o.Status != domain.OrderPending && o.Status != domain.OrderProcess {
		return nil
	}
	if o.Status == domain.OrderPending {
		claimed, err := u.orders.ClaimProcessing(orderID)
		if err != nil {
			return err
		}
		if !claimed {
			after, err := u.orders.GetByID(orderID)
			if err != nil || after.Status == domain.OrderPaid {
				return err
			}
			return nil
		}
	}
	p, err := u.pkgs.GetByID(o.PackageID)
	if err != nil {
		return u.fail(orderID, "paket tidak ditemukan")
	}
	length := u.voucherLength()
	if u.RandString == nil {
		return u.fail(orderID, "generator kredensial belum dikonfigurasi")
	}
	if v, err := u.vouchers.GetByOrderID(orderID); err == nil {
		_ = v
		return u.orders.MarkPaid(orderID, u.Now())
	}
	code, err := u.RandString(length)
	if err != nil {
		return u.fail(orderID, "gagal membuat kredensial")
	}
	sess, _, err := u.conns.ActiveSession()
	if err != nil {
		return u.fail(orderID, "router: "+shortErr(err))
	}
	defer sess.Close()
	comment := domain.OrderCommentPrefix + orderID
	_, found, err := sess.FindHotspotUserByComment(comment)
	if err != nil {
		return u.fail(orderID, "router: "+shortErr(err))
	}
	if !found {
		// Tanpa limit-uptime: masa aktif mengikuti session-timeout profile di router.
		err = sess.AddHotspotUser(domain.HotspotUser{
			Name: code, Password: code, Profile: p.ProfileName,
			Comment: comment,
		})
		if err != nil {
			return u.fail(orderID, "router: "+shortErr(err))
		}
	}
	enc, err := u.box.Seal(code)
	if err != nil {
		return u.fail(orderID, "gagal menyimpan voucher")
	}
	paidAt := u.Now()
	if err := u.orders.CompletePaid(orderID, domain.Voucher{
		OrderID: orderID, Username: code,
		EncPassword: enc, ProfileName: p.ProfileName,
	}, paidAt); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return u.orders.MarkPaid(orderID, paidAt)
		}
		return u.fail(orderID, "gagal menyimpan voucher")
	}
	_ = u.audits.Log("system", "order_paid", "order", orderID, "{}")
	return nil
}

// fail menandai order gagal + audit, lalu mengembalikan error untuk log.
func (u *PaymentUsecase) fail(orderID, reason string) error {
	_ = u.orders.MarkFailed(orderID, reason)
	_ = u.audits.Log("system", "order_failed", "order", orderID, `{"alasan":"`+reason+`"}`)
	return errors.New(reason)
}

func (u *PaymentUsecase) voucherLength() int {
	v, found, err := u.set.Get(domain.SettingVoucherLength)
	if err != nil || !found {
		return 8
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 6 || n > 32 {
		return 8
	}
	return n
}

// PollOnce menjalankan satu siklus poller: batch status untuk order segar,
// konfirmasi GET sekali untuk order yang expires_at-nya lewat.
func (u *PaymentUsecase) PollOnce(ctx context.Context) error {
	now := u.Now()
	client, err := u.invoices()
	if err != nil {
		return nil
	}
	fresh, err := u.orders.PendingFresh(pollBatchSize, now)
	if err != nil {
		return err
	}
	byInvoice := map[string]domain.Order{}
	ids := make([]string, 0, len(fresh))
	for _, o := range fresh {
		if o.KrisKrisInvoice == "" {
			continue
		}
		byInvoice[o.KrisKrisInvoice] = o
		ids = append(ids, o.KrisKrisInvoice)
	}
	if len(ids) > 0 {
		states, err := client.BatchStatus(ctx, ids)
		if err == nil {
			for _, s := range states {
				o, ok := byInvoice[s.ID]
				if !ok || !s.Found {
					continue
				}
				switch s.Status {
				case domain.OrderPaid:
					_ = u.ProcessPaidOrder(o.ID)
				case domain.OrderExpired:
					if o.Status == domain.OrderPending {
						_ = u.orders.MarkExpired(o.ID)
					}
				case domain.OrderCancelled:
					if o.Status == domain.OrderPending {
						_ = u.orders.MarkCancelled(o.ID)
					}
				}
			}
		}
	}
	stale, err := u.orders.PendingStale(pollBatchSize, now)
	if err != nil {
		return err
	}
	for _, o := range stale {
		if o.KrisKrisInvoice == "" {
			_ = u.orders.MarkExpired(o.ID)
			continue
		}
		inv, err := client.GetInvoice(ctx, o.KrisKrisInvoice)
		if err != nil {
			continue
		}
		if inv.Status == domain.OrderPaid {
			_ = u.ProcessPaidOrder(o.ID)
		} else {
			_ = u.orders.MarkExpired(o.ID)
		}
	}
	return nil
}
