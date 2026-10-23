package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

// fakeOrders adalah OrderRepository memori untuk test.
type fakeOrders struct {
	mu       sync.Mutex
	rows     map[string]domain.Order
	vouchers map[string]domain.Voucher
}

func newFakeOrders() *fakeOrders {
	return &fakeOrders{rows: map[string]domain.Order{}, vouchers: map[string]domain.Voucher{}}
}

func (f *fakeOrders) Create(o domain.Order) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[o.ID] = o
	return nil
}

func (f *fakeOrders) GetByID(id string) (domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.rows[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	return o, nil
}

func (f *fakeOrders) GetByInvoice(invoiceID string) (domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.rows {
		if o.KrisKrisInvoice == invoiceID {
			return o, nil
		}
	}
	return domain.Order{}, domain.ErrNotFound
}

func (f *fakeOrders) pending(limit int, now time.Time, stale bool) ([]domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Order{}
	for _, o := range f.rows {
		if o.Status != domain.OrderPending || o.ExpiresAt == nil {
			continue
		}
		isStale := !o.ExpiresAt.After(now)
		if isStale != stale {
			continue
		}
		out = append(out, o)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeOrders) PendingFresh(limit int, now time.Time) ([]domain.Order, error) {
	return f.pending(limit, now, false)
}

func (f *fakeOrders) PendingStale(limit int, now time.Time) ([]domain.Order, error) {
	return f.pending(limit, now, true)
}

func (f *fakeOrders) ClaimProcessing(id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.rows[id]
	if !ok || o.Status != domain.OrderPending {
		return false, nil
	}
	o.Status = domain.OrderProcess
	f.rows[id] = o
	return true, nil
}

func (f *fakeOrders) CompletePaid(orderID string, v domain.Voucher, paidAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.vouchers[orderID]; ok {
		return errors.New("UNIQUE constraint failed: vouchers.order_id")
	}
	f.vouchers[orderID] = v
	o := f.rows[orderID]
	o.Status = domain.OrderPaid
	o.PaidAt = &paidAt
	f.rows[orderID] = o
	return nil
}

func (f *fakeOrders) setStatus(id, status string, paidAt *time.Time, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o := f.rows[id]
	o.Status = status
	if paidAt != nil {
		o.PaidAt = paidAt
	}
	if reason != "" {
		o.FailedReason = reason
	}
	f.rows[id] = o
}

func (f *fakeOrders) MarkPaid(id string, paidAt time.Time) error {
	f.setStatus(id, domain.OrderPaid, &paidAt, "")
	return nil
}

func (f *fakeOrders) MarkExpired(id string) error {
	f.setStatus(id, domain.OrderExpired, nil, "")
	return nil
}

func (f *fakeOrders) MarkCancelled(id string) error {
	f.setStatus(id, domain.OrderCancelled, nil, "")
	return nil
}

func (f *fakeOrders) MarkFailed(id, reason string) error {
	f.setStatus(id, domain.OrderFailed, nil, reason)
	return nil
}

func (f *fakeOrders) match(status, q string, o domain.Order) bool {
	if status != "" && o.Status != status {
		return false
	}
	if q != "" && !strings.Contains(o.ID, q) && !strings.Contains(o.KrisKrisInvoice, q) {
		return false
	}
	return true
}

func (f *fakeOrders) List(status, q string, limit, offset int) ([]domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Order
	for _, o := range f.rows {
		if f.match(status, q, o) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if offset >= len(out) {
		return []domain.Order{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeOrders) Count(status, q string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, o := range f.rows {
		if f.match(status, q, o) {
			n++
		}
	}
	return n, nil
}

func (f *fakeOrders) CountByStatus(status string) (int, error) {
	return f.Count(status, "")
}

func (f *fakeOrders) RevenueSince(since time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, o := range f.rows {
		if o.Status == domain.OrderPaid && o.PaidAt != nil && !o.PaidAt.Before(since) {
			total += o.TotalAmount
		}
	}
	return total, nil
}

func (f *fakeOrders) Recent(limit int) ([]domain.Order, error) {
	return f.List("", "", limit, 0)
}

func (f *fakeOrders) RequeueFailed(id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.rows[id]
	if !ok || o.Status != domain.OrderFailed {
		return false, nil
	}
	o.Status = domain.OrderPending
	o.FailedReason = ""
	f.rows[id] = o
	return true, nil
}

// fakeVouchers membaca voucher dari fakeOrders.
type fakeVouchers struct{ orders *fakeOrders }

func (f *fakeVouchers) GetByOrderID(orderID string) (domain.Voucher, error) {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	v, ok := f.orders.vouchers[orderID]
	if !ok {
		return domain.Voucher{}, domain.ErrNotFound
	}
	return v, nil
}

// fakeWebhooks adalah WebhookRepository memori.
type fakeWebhooks struct {
	mu   sync.Mutex
	rows map[string]domain.WebhookDelivery
}

func (f *fakeWebhooks) Exists(id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.rows[id]
	return ok, nil
}

func (f *fakeWebhooks) Record(d domain.WebhookDelivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]domain.WebhookDelivery{}
	}
	f.rows[d.DeliveryID] = d
	return nil
}

// fakeAudits menampung log audit.
type fakeAudits struct {
	mu   sync.Mutex
	logs []string
}

func (f *fakeAudits) Log(actor, action, entity, entityID, meta string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, action+":"+entityID)
	return nil
}

// fakeInvoiceClient meniru gateway invoice dengan skrip per metode.
type fakeInvoiceClient struct {
	mu          sync.Mutex
	createCalls int
	createErrs  []error
	createOut   InvoiceInfo
	batchOut    []InvoiceStatusInfo
	batchErr    error
	batchIDs    [][]string
	getOut      map[string]InvoiceInfo
	getErr      error
	cancelled   []string
	lastLabel   string
}

func (f *fakeInvoiceClient) CreateInvoice(_ context.Context, _ int, label string) (InvoiceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	f.lastLabel = label
	if len(f.createErrs) > 0 {
		err := f.createErrs[0]
		f.createErrs = f.createErrs[1:]
		if err != nil {
			return InvoiceInfo{}, err
		}
	}
	return f.createOut, nil
}

func (f *fakeInvoiceClient) GetInvoice(_ context.Context, id string) (InvoiceInfo, error) {
	if f.getErr != nil {
		return InvoiceInfo{}, f.getErr
	}
	if inv, ok := f.getOut[id]; ok {
		return inv, nil
	}
	return InvoiceInfo{ID: id, Status: "pending"}, nil
}

func (f *fakeInvoiceClient) BatchStatus(_ context.Context, ids []string) ([]InvoiceStatusInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batchIDs = append(f.batchIDs, append([]string{}, ids...))
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	return f.batchOut, nil
}

func (f *fakeInvoiceClient) CancelInvoice(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = append(f.cancelled, id)
	return nil
}

func payStack() (*fakeOrders, *fakePackages, *fakeSettings, *fakeWebhooks, *fakeAudits, *ConnectionUsecase, *fakeSession) {
	orders := newFakeOrders()
	pkgs := newFakePackages()
	set := newFakeSettings()
	hooks := &fakeWebhooks{}
	audits := &fakeAudits{}
	conns := newFakeConns()
	conns.insert(domain.Connection{Host: "r", Port: 8728, Username: "a", EncPassword: "enc:pw", IsActive: true})
	sess := &fakeSession{}
	gw := &fakeGateway{sess: sess}
	connUC := NewConnectionUsecase(conns, fakeBox{}, gw)
	return orders, pkgs, set, hooks, audits, connUC, sess
}

func seedPackage(t *testing.T, pkgs *fakePackages, active bool) domain.Package {
	t.Helper()
	p, err := pkgs.Create(domain.Package{
		Name: "Harian", PriceIDR: 15000,
		ValidityLabel: "12 Jam", ProfileName: "default", IsActive: active,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func verifyOK(string, []byte, string) bool { return true }

func TestOrderCreateSukses(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	p := seedPackage(t, pkgs, true)
	_ = set
	exp := time.Now().Add(15 * time.Minute)
	inv := &fakeInvoiceClient{createOut: InvoiceInfo{
		ID: "inv_1", Status: "pending", Amount: 15042,
		BaseAmount: 15000, UniqueCode: 42, QRPayload: "QRISI",
		ExpiresAt: &exp,
	}}
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return inv, nil
	})

	res, err := ordUC.Create(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Order.BaseAmount != 15000 || res.Order.UniqueCode != 42 || res.Order.TotalAmount != 15042 {
		t.Fatalf("order = %+v", res.Order)
	}
	if res.Order.QRPayload != "QRISI" || res.Order.Status != domain.OrderPending {
		t.Fatalf("order = %+v", res.Order)
	}
	if len(inv.lastLabel) > 64 {
		t.Fatalf("label %d char melebihi 64", len(inv.lastLabel))
	}
	_ = hooks
	_ = audits
	_ = connUC
}

func TestOrderCreatePaketNonaktif(t *testing.T) {
	orders, pkgs, set, _, _, _, _ := payStack()
	p := seedPackage(t, pkgs, false)
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return &fakeInvoiceClient{}, nil
	})
	if _, err := ordUC.Create(context.Background(), p.ID); err == nil {
		t.Fatal("paket nonaktif lolos")
	}
	if _, err := ordUC.Create(context.Background(), 999); err == nil {
		t.Fatal("paket asing lolos")
	}
}

func TestOrderCreateRetryHormatiRetryAfter(t *testing.T) {
	orders, pkgs, set, _, _, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	inv := &fakeInvoiceClient{
		createErrs: []error{
			InvoiceError{Msg: "sibuk", Status: 429, RetryAfter: 3},
			InvoiceError{Msg: "habis", Code: "duplicate_pending"},
		},
		createOut: InvoiceInfo{ID: "inv_9", Status: "pending", Amount: 15011,
			BaseAmount: 15000, UniqueCode: 11, QRPayload: "Q"},
	}
	var waits []time.Duration
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return inv, nil
	})
	ordUC.Sleep = func(d time.Duration) { waits = append(waits, d) }

	res, err := ordUC.Create(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Order.KrisKrisInvoice != "inv_9" {
		t.Fatalf("invoice = %s", res.Order.KrisKrisInvoice)
	}
	if len(waits) != 2 || waits[0] != 3*time.Second {
		t.Fatalf("jeda = %v, want [3s ...]", waits)
	}
	if inv.createCalls != 3 {
		t.Fatalf("create dipanggil %d kali, want 3", inv.createCalls)
	}
}

func TestOrderCreateRetryHabis(t *testing.T) {
	orders, pkgs, set, _, _, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	inv := &fakeInvoiceClient{createErrs: []error{
		InvoiceError{Msg: "sibuk", Status: 429},
		InvoiceError{Msg: "sibuk", Status: 429},
		InvoiceError{Msg: "sibuk", Status: 429},
		InvoiceError{Msg: "sibuk", Status: 429},
		InvoiceError{Msg: "sibuk", Status: 429},
	}}
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return inv, nil
	})
	ordUC.Sleep = func(time.Duration) {}
	if _, err := ordUC.Create(context.Background(), p.ID); err == nil {
		t.Fatal("retry habis tapi lolos")
	}
	if inv.createCalls != 4 { // 1 awal + 3 retry
		t.Fatalf("create dipanggil %d kali, want 4", inv.createCalls)
	}
}

func TestOrderCreateGagalPermanenTanpaRetry(t *testing.T) {
	orders, pkgs, set, _, _, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	inv := &fakeInvoiceClient{createErrs: []error{
		InvoiceError{Msg: "nominal salah", Code: "invalid_amount"},
	}}
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return inv, nil
	})
	if _, err := ordUC.Create(context.Background(), p.ID); err == nil {
		t.Fatal("invalid_amount lolos")
	}
	if inv.createCalls != 1 {
		t.Fatalf("create dipanggil %d kali, want 1", inv.createCalls)
	}
}

func TestOrderCancel(t *testing.T) {
	orders, pkgs, set, _, _, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	inv := &fakeInvoiceClient{createOut: InvoiceInfo{
		ID: "inv_c", Status: "pending", Amount: 15020,
		BaseAmount: 15000, UniqueCode: 20, QRPayload: "Q",
	}}
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return inv, nil
	})
	res, err := ordUC.Create(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	o, err := ordUC.Cancel(context.Background(), res.Order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != domain.OrderCancelled {
		t.Fatalf("status = %s", o.Status)
	}
	if len(inv.cancelled) != 1 || inv.cancelled[0] != "inv_c" {
		t.Fatalf("cancel invoice = %v", inv.cancelled)
	}
	if _, err := ordUC.Cancel(context.Background(), res.Order.ID); err == nil {
		t.Fatal("cancel ganda lolos")
	}
}

func newPayUC(orders *fakeOrders, pkgs *fakePackages, set *fakeSettings,
	hooks *fakeWebhooks, audits *fakeAudits, connUC *ConnectionUsecase,
	inv InvoiceClient) *PaymentUsecase {
	if err := set.Set(domain.SettingKrisKrisCallbackSec, "enc:cb-secret"); err != nil {
		panic(err)
	}
	pay := NewPaymentUsecase(orders, &fakeVouchers{orders}, hooks, audits,
		pkgs, set, fakeBox{}, connUC,
		func() (InvoiceClient, error) { return inv, nil }, verifyOK)
	pay.RandString = func(n int) (string, error) { return "VCHR1234"[:n], nil }
	return pay
}

func TestProcessPaidOrderIdempoten(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, sess := payStack()
	p := seedPackage(t, pkgs, true)
	exp := time.Now().Add(15 * time.Minute)
	orders.Create(domain.Order{ID: "ord_test1", PackageID: p.ID,
		BaseAmount: 15000, UniqueCode: 1, TotalAmount: 15001,
		KrisKrisInvoice: "inv_1", Status: domain.OrderPending, ExpiresAt: &exp})
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})

	if err := pay.ProcessPaidOrder("ord_test1"); err != nil {
		t.Fatal(err)
	}
	if err := pay.ProcessPaidOrder("ord_test1"); err != nil {
		t.Fatal(err)
	}
	o, _ := orders.GetByID("ord_test1")
	if o.Status != domain.OrderPaid || o.PaidAt == nil {
		t.Fatalf("order = %+v", o)
	}
	if len(orders.vouchers) != 1 {
		t.Fatalf("voucher = %d, want 1", len(orders.vouchers))
	}
	if len(sess.users) != 1 {
		t.Fatalf("user router = %d, want 1", len(sess.users))
	}
	u := sess.users[0]
	if u.Name != "VCHR1234" || u.Password != "VCHR1234" {
		t.Fatalf("kredensial = %+v", u)
	}
	if u.Profile != "default" || u.LimitUptime != "" || u.Comment != "mikbil:ord_test1" {
		t.Fatalf("user = %+v", u)
	}
}

func TestProcessPaidOrderRouterGagal(t *testing.T) {
	orders, pkgs, set, hooks, audits, _, _ := payStack()
	p := seedPackage(t, pkgs, true)
	exp := time.Now().Add(15 * time.Minute)
	orders.Create(domain.Order{ID: "ord_fail1", PackageID: p.ID,
		BaseAmount: 15000, UniqueCode: 2, TotalAmount: 15002,
		KrisKrisInvoice: "inv_2", Status: domain.OrderPending, ExpiresAt: &exp})
	conns := newFakeConns() // tanpa koneksi aktif
	connUC := NewConnectionUsecase(conns, fakeBox{}, &fakeGateway{sess: &fakeSession{}})
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})

	if err := pay.ProcessPaidOrder("ord_fail1"); err == nil {
		t.Fatal("router mati tapi sukses")
	}
	o, _ := orders.GetByID("ord_fail1")
	if o.Status != domain.OrderFailed || o.FailedReason == "" {
		t.Fatalf("order = %+v", o)
	}
	if len(orders.vouchers) != 0 {
		t.Fatal("voucher tercipta walau router gagal")
	}
	if len(audits.logs) != 1 {
		t.Fatalf("audit = %v", audits.logs)
	}
}

func TestHandleWebhookDedup(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	p := seedPackage(t, pkgs, true)
	exp := time.Now().Add(15 * time.Minute)
	orders.Create(domain.Order{ID: "ord_web1", PackageID: p.ID,
		BaseAmount: 15000, UniqueCode: 3, TotalAmount: 15003,
		KrisKrisInvoice: "inv_3", Status: domain.OrderPending, ExpiresAt: &exp})
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})

	body := []byte(`{"event":"invoice.paid","delivery_id":"del_1","invoice_id":"inv_3"}`)
	first, err := pay.HandleWebhook(body, "invoice.paid", "del_1", "sig")
	if err != nil || first != "ord_web1" {
		t.Fatalf("pertama = %q, err = %v", first, err)
	}
	second, err := pay.HandleWebhook(body, "invoice.paid", "del_1", "sig")
	if err != nil || second != "" {
		t.Fatalf("kedua = %q, err = %v (duplikat harus kosong)", second, err)
	}
	ping, err := pay.HandleWebhook(
		[]byte(`{"event":"test.ping","delivery_id":"del_ping"}`), "test.ping", "del_ping", "sig")
	if err != nil || ping != "" {
		t.Fatalf("ping = %q, err = %v", ping, err)
	}
	if len(hooks.rows) != 2 {
		t.Fatalf("delivery tercatat = %d, want 2", len(hooks.rows))
	}
}

func TestHandleWebhookSignatureSalah(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	_ = set.Set(domain.SettingKrisKrisCallbackSec, "enc:cb-secret")
	pay := NewPaymentUsecase(orders, &fakeVouchers{orders}, hooks, audits,
		pkgs, set, fakeBox{}, connUC,
		func() (InvoiceClient, error) { return &fakeInvoiceClient{}, nil },
		func(string, []byte, string) bool { return false })
	if _, err := pay.HandleWebhook([]byte(`{}`), "invoice.paid", "d", "salah"); err == nil {
		t.Fatal("signature salah lolos")
	} else {
		var unauth UnauthorizedError
		if !errors.As(err, &unauth) {
			t.Fatalf("err = %v, want UnauthorizedError", err)
		}
	}
}

func TestPollerBatchDanStale(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	p := seedPackage(t, pkgs, true)
	fresh := time.Now().Add(10 * time.Minute)
	stale := time.Now().Add(-time.Minute)
	orders.Create(domain.Order{ID: "ord_pol1", PackageID: p.ID, TotalAmount: 15001,
		KrisKrisInvoice: "inv_pol1", Status: domain.OrderPending, ExpiresAt: &fresh})
	orders.Create(domain.Order{ID: "ord_pol2", PackageID: p.ID, TotalAmount: 15002,
		KrisKrisInvoice: "inv_pol2", Status: domain.OrderPending, ExpiresAt: &fresh})
	orders.Create(domain.Order{ID: "ord_pol3", PackageID: p.ID, TotalAmount: 15003,
		KrisKrisInvoice: "inv_pol3", Status: domain.OrderPending, ExpiresAt: &stale})
	inv := &fakeInvoiceClient{
		batchOut: []InvoiceStatusInfo{
			{ID: "inv_pol1", Found: true, Status: "paid"},
			{ID: "inv_pol2", Found: true, Status: "expired"},
		},
		getOut: map[string]InvoiceInfo{
			"inv_pol3": {ID: "inv_pol3", Status: "expired"},
		},
	}
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, inv)

	if err := pay.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(inv.batchIDs) != 1 || len(inv.batchIDs[0]) != 2 {
		t.Fatalf("batch = %v, want 1 request 2 id", inv.batchIDs)
	}
	o1, _ := orders.GetByID("ord_pol1")
	if o1.Status != domain.OrderPaid {
		t.Fatalf("pol1 = %s, want paid", o1.Status)
	}
	if len(orders.vouchers) != 1 {
		t.Fatalf("voucher = %d, want 1", len(orders.vouchers))
	}
	o2, _ := orders.GetByID("ord_pol2")
	if o2.Status != domain.OrderExpired {
		t.Fatalf("pol2 = %s, want expired", o2.Status)
	}
	o3, _ := orders.GetByID("ord_pol3")
	if o3.Status != domain.OrderExpired {
		t.Fatalf("pol3 = %s, want expired", o3.Status)
	}
}

func TestDetailVoucherHanyaBilaPaid(t *testing.T) {
	orders, pkgs, set, hooks, audits, connUC, _ := payStack()
	p := seedPackage(t, pkgs, true)
	_ = set.Set(domain.SettingHotspotLoginBase, "http://192.168.88.1/login")
	exp := time.Now().Add(15 * time.Minute)
	orders.Create(domain.Order{ID: "ord_det1", PackageID: p.ID, TotalAmount: 15001,
		KrisKrisInvoice: "inv_d", Status: domain.OrderPending, ExpiresAt: &exp})
	ordUC := NewOrderUsecase(orders, &fakeVouchers{orders}, pkgs, set, fakeBox{}, func() (InvoiceClient, error) {
		return &fakeInvoiceClient{}, nil
	})
	d, err := ordUC.Detail("ord_det1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Voucher != nil || d.LoginURL != "" {
		t.Fatal("pending tidak boleh memuat voucher/login_url")
	}
	pay := newPayUC(orders, pkgs, set, hooks, audits, connUC, &fakeInvoiceClient{})
	if err := pay.ProcessPaidOrder("ord_det1"); err != nil {
		t.Fatal(err)
	}
	d, err = ordUC.Detail("ord_det1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Voucher == nil || d.Voucher.Username != "VCHR1234" || d.Voucher.Password != "VCHR1234" {
		t.Fatalf("voucher = %+v", d.Voucher)
	}
	if d.LoginURL != "http://192.168.88.1/login?username=VCHR1234&password=VCHR1234" {
		t.Fatalf("login_url = %s", d.LoginURL)
	}
}
