// Command mikbil: serve | migrate | genkey | version.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	deliveryhttp "github.com/mikbil/mikbil/internal/delivery/http"
	"github.com/mikbil/mikbil/internal/domain"
	infra "github.com/mikbil/mikbil/internal/infra/crypto"
	"github.com/mikbil/mikbil/internal/infra/kriskris"
	"github.com/mikbil/mikbil/internal/infra/mikrotik"
	"github.com/mikbil/mikbil/internal/repository/sqlite"
	"github.com/mikbil/mikbil/internal/usecase"
)

// Version adalah versi binary; diisi ulang saat rilis via -X main.Version=<tag>.
var Version = "0.1.0"

type hasher struct{}

func (hasher) Hash(pw string) (string, error) { return infra.HashPassword(pw) }
func (hasher) Check(hash, pw string) bool     { return infra.CheckPassword(hash, pw) }

// gwAdapter menjembatani port usecase.RouterGateway ke infra mikrotik.
type gwAdapter struct{ gw mikrotik.Gateway }

func (a gwAdapter) Dial(p usecase.RouterDialParams) (usecase.RouterSession, error) {
	return a.gw.Dial(mikrotik.DialParams{
		Host: p.Host, Port: p.Port, Username: p.Username,
		Password: p.Password, UseTLS: p.UseTLS,
	})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func openDB() (*sql.DB, error) {
	db, err := sqlite.Open(envOr("MIKBIL_DB", "./mikbil.db"))
	if err != nil {
		return nil, err
	}
	if err := sqlite.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func mustProtector() *infra.Protector {
	p, err := infra.NewProtector(os.Getenv("MIKBIL_ENC_KEY"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "MIKBIL_ENC_KEY tidak valid:", err)
		fmt.Fprintln(os.Stderr, "Buat kunci baru dengan: mikbil genkey")
		os.Exit(1)
	}
	return p
}

// invoiceAdapter menjembatani usecase.InvoiceClient ke infra kriskris.
type invoiceAdapter struct{ c *kriskris.Client }

// skewAdapter menjembatani usecase.SkewReader ke perekam skew bersama.
type skewAdapter struct{ rec *kriskris.SkewRecorder }

func (a skewAdapter) ClockSkewSec() (float64, bool) {
	d, ok := a.rec.Skew()
	if !ok {
		return 0, false
	}
	return d.Seconds(), true
}

func adaptInvoice(inv kriskris.Invoice) usecase.InvoiceInfo {
	return usecase.InvoiceInfo{
		ID: inv.ID, Status: inv.Status, Amount: inv.Amount,
		BaseAmount: inv.BaseAmount, UniqueCode: inv.UniqueCode,
		Label: inv.Label, QRPayload: inv.QRPayload,
		ExpiresAt: inv.ExpiresAt, PaidAt: inv.PaidAt,
	}
}

func adaptErr(err error) error {
	var apiErr *kriskris.APIError
	if errors.As(err, &apiErr) {
		return usecase.InvoiceError{
			Msg: apiErr.Message, Code: apiErr.Code,
			Status: apiErr.StatusCode, RetryAfter: apiErr.RetryAfter,
		}
	}
	return err
}

func (a invoiceAdapter) CreateInvoice(ctx context.Context, amount int, label string) (usecase.InvoiceInfo, error) {
	inv, err := a.c.CreateInvoice(ctx, amount, label)
	if err != nil {
		return usecase.InvoiceInfo{}, adaptErr(err)
	}
	return adaptInvoice(inv), nil
}

func (a invoiceAdapter) GetInvoice(ctx context.Context, id string) (usecase.InvoiceInfo, error) {
	inv, err := a.c.GetInvoice(ctx, id)
	if err != nil {
		return usecase.InvoiceInfo{}, adaptErr(err)
	}
	return adaptInvoice(inv), nil
}

func (a invoiceAdapter) BatchStatus(ctx context.Context, ids []string) ([]usecase.InvoiceStatusInfo, error) {
	states, err := a.c.BatchStatus(ctx, ids)
	if err != nil {
		return nil, adaptErr(err)
	}
	out := make([]usecase.InvoiceStatusInfo, 0, len(states))
	for _, s := range states {
		out = append(out, usecase.InvoiceStatusInfo{
			ID: s.ID, Found: s.Found, Status: s.Status, PaidAt: s.PaidAt,
		})
	}
	return out, nil
}

func (a invoiceAdapter) CancelInvoice(ctx context.Context, id string) error {
	return adaptErr(a.c.CancelInvoice(ctx, id))
}

func cmdServe() {
	box := mustProtector()
	db, err := openDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membuka database:", err)
		os.Exit(1)
	}
	defer db.Close()

	admins := sqlite.NewAdminRepo(db)
	conns := sqlite.NewConnectionRepo(db)
	set := sqlite.NewSettingsRepo(db)
	sessions := sqlite.NewSessionRepo(db)
	orders := sqlite.NewOrderRepo(db)
	vouchers := sqlite.NewVoucherRepo(db)
	webhooks := sqlite.NewWebhookRepo(db)
	audits := sqlite.NewAuditRepo(db)
	pkgRepo := sqlite.NewPackageRepo(db)
	h := hasher{}
	gw := mikrotik.Gateway{}
	connUC := usecase.NewConnectionUsecase(conns, box, gwAdapter{gw})
	profUC := usecase.NewProfileUsecase(connUC)
	allowRepo := sqlite.NewAllowlistRepo(db)
	allowUC := usecase.NewAllowlistUsecase(allowRepo, set, connUC)
	provUC := usecase.NewProvisioningUsecase(set, connUC, allowRepo)
	hotUC := usecase.NewHotspotUsecase(set)
	setup := usecase.NewSetupUsecase(admins, conns, set, h, box, connUC, provUC)
	auth := usecase.NewAuthUsecase(admins, sessions, set, h, infra.RandomBytes)
	pkgUC := usecase.NewPackageUsecase(pkgRepo, profUC)

	skewRec := &kriskris.SkewRecorder{}
	buildInvoice := func(key, secret string) (usecase.InvoiceClient, error) {
		return invoiceAdapter{c: &kriskris.Client{
			APIKey: key, APISecret: secret,
			HTTP:   &http.Client{Timeout: 10 * time.Second},
			Shared: skewRec,
		}}, nil
	}
	factory := func() (usecase.InvoiceClient, error) {
		key, found, err := set.Get(domain.SettingKrisKrisAPIKey)
		if err != nil || !found || key == "" {
			return nil, usecase.InputError{Msg: "kredensial KrisKris belum dikonfigurasi"}
		}
		enc, found, err := set.Get(domain.SettingKrisKrisAPISecret)
		if err != nil || !found {
			return nil, usecase.InputError{Msg: "kredensial KrisKris belum dikonfigurasi"}
		}
		secret, err := box.Open(enc)
		if err != nil {
			return nil, err
		}
		return buildInvoice(key, secret)
	}
	orderUC := usecase.NewOrderUsecase(orders, vouchers, pkgRepo, set, box, factory)
	payUC := usecase.NewPaymentUsecase(orders, vouchers, webhooks, audits,
		pkgRepo, set, box, connUC, factory, kriskris.VerifyWebhook)
	payUC.RandString = infra.RandomString
	adminUC := usecase.NewAdminOrderUsecase(orders, vouchers, pkgRepo, set, box, payUC)
	healthUC := usecase.NewHealthUsecase(db, conns, skewAdapter{skewRec})
	setUC := usecase.NewSettingsUsecase(set, box)
	themeUC := usecase.NewThemeUsecase(set, sqlite.NewThemeRepo(db))

	go runPoller(payUC)

	addr := envOr("MIKBIL_ADDR", ":8080")
	fmt.Println("Mikbil", Version, "mendengarkan di", addr)
	if err := http.ListenAndServe(addr, deliveryhttp.NewRouter(deliveryhttp.Deps{
		Setup: setup, Auth: auth, Conn: connUC, Prof: profUC, Prov: provUC,
		Hot: hotUC, Pkgs: pkgUC, Ord: orderUC, Pay: payUC, Adm: adminUC,
		Health: healthUC, Set: setUC, Allow: allowUC, Theme: themeUC,
	})); err != nil {
		fmt.Fprintln(os.Stderr, "server berhenti:", err)
		os.Exit(1)
	}
}

// runPoller menjalankan siklus status invoice tiap 5 detik (fallback webhook).
func runPoller(pay *usecase.PaymentUsecase) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = pay.PollOnce(ctx)
		cancel()
	}
}

func cmdMigrate() {
	db, err := sqlite.Open(envOr("MIKBIL_DB", "./mikbil.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membuka database:", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := sqlite.Migrate(db); err != nil {
		fmt.Fprintln(os.Stderr, "migrasi gagal:", err)
		os.Exit(1)
	}
	fmt.Println("migrasi selesai")
}

func cmdGenkey() {
	b, err := infra.RandomBytes(32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membuat kunci:", err)
		os.Exit(1)
	}
	for _, c := range b {
		fmt.Printf("%02x", c)
	}
	fmt.Println()
}

func usage() {
	fmt.Fprintln(os.Stderr, "Pakai: mikbil <serve|migrate|genkey|version>")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		cmdServe()
	case "migrate":
		cmdMigrate()
	case "genkey":
		cmdGenkey()
	case "version":
		fmt.Println("mikbil", Version)
	default:
		usage()
		os.Exit(2)
	}
}
