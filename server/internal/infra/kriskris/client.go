package kriskris

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// BaseURL adalah alamat API KrisKris.id.
const BaseURL = "https://kriskris.id"

// Path API invoice eksternal.
const (
	pathInvoices = "/api/v1/external/invoices"
	pathStatus   = "/api/v1/external/invoices/status"
)

// Invoice adalah data invoice dari KrisKris.
type Invoice struct {
	ID         string
	Status     string
	Amount     int
	BaseAmount int
	UniqueCode int
	Label      string
	QRPayload  string
	ExpiresAt  *time.Time
	PaidAt     *time.Time
}

// InvoiceStatus adalah satu butir hasil batch status.
type InvoiceStatus struct {
	ID     string
	Found  bool
	Status string
	PaidAt *time.Time
}

// APIError adalah kegagalan API KrisKris berenvelope.
type APIError struct {
	StatusCode int
	Code       string // invalid_amount, duplicate_pending, ...
	Message    string
	RetryAfter int // detik, dari respons 429
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("kriskris %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("kriskris http %d: %s", e.StatusCode, e.Message)
}

// Client memanggil API KrisKris dengan signature HMAC per request.
type Client struct {
	BaseURL   string
	APIKey    string
	APISecret string
	HTTP      *http.Client
	Now       func() time.Time

	mu        sync.Mutex
	clockSkew time.Duration
	skewOK    bool

	// Shared menerima tiap sampel skew agar health-check bisa membacanya.
	Shared *SkewRecorder
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// ClockSkew mengembalikan selisih jam server KrisKris terhadap jam lokal
// dari header Date respons terakhir; ok=false bila belum ada sampel.
func (c *Client) ClockSkew() (skew time.Duration, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clockSkew, c.skewOK
}

func (c *Client) observeDate(h http.Header) {
	ds := h.Get("Date")
	if ds == "" {
		return
	}
	serverTime, err := time.Parse(time.RFC1123, ds)
	if err != nil {
		serverTime, err = time.Parse(time.RFC1123Z, ds)
		if err != nil {
			return
		}
	}
	skew := serverTime.Sub(c.now())
	c.mu.Lock()
	c.clockSkew = skew
	c.skewOK = true
	shared := c.Shared
	c.mu.Unlock()
	if shared != nil {
		shared.Observe(skew)
	}
}

type envelope struct {
	OK         bool            `json:"ok"`
	Data       json.RawMessage `json:"data"`
	Error      string          `json:"error"`
	Code       string          `json:"code"`
	RetryAfter int             `json:"retry_after"`
}

// debugAktif menyalakan log request/response KrisKris bila env
// MIKBIL_DEBUG_KRISKRIS=1. Secret dan signature tidak pernah dicatat.
func debugAktif() bool { return os.Getenv("MIKBIL_DEBUG_KRISKRIS") == "1" }

// ringkas memangkas teks log agar tidak membanjiri stderr.
func ringkas(s string, maks int) string {
	if len(s) > maks {
		return s[:maks] + "..."
	}
	return s
}

func debugReq(method, path, rawQuery, stamp, keyPrefix string) {
	if !debugAktif() {
		return
	}
	target := path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	log.Printf("kriskris req %s %s ts=%s key=%s***", method, target, stamp, keyPrefix)
}

func debugResp(status int, raw []byte) {
	if !debugAktif() {
		return
	}
	log.Printf("kriskris resp http=%d body=%s", status, ringkas(string(raw), 2048))
}

func debugErr(err error) {
	if !debugAktif() {
		return
	}
	log.Printf("kriskris transport gagal: %s", ringkas(err.Error(), 512))
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path, rawQuery string, payload any) (json.RawMessage, http.Header, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
	}
	base := c.BaseURL
	if base == "" {
		base = BaseURL
	}
	target := base + path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	stamp := c.now().UTC().Format("2006-01-02T15:04:05Z")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("X-Kriskris-Timestamp", stamp)
	req.Header.Set("X-Kriskris-Signature", SignRequest(c.APISecret, stamp, method, path, rawQuery, body))
	debugReq(method, path, rawQuery, stamp, ringkas(c.APIKey, 10))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		debugErr(err)
		return nil, nil, err
	}
	defer resp.Body.Close()
	c.observeDate(resp.Header)
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	debugResp(resp.StatusCode, raw)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, &APIError{StatusCode: resp.StatusCode, Message: "respons tidak valid"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !env.OK {
		msg := env.Error
		if msg == "" {
			msg = "request gagal"
		}
		code, retryAfter := env.Code, env.RetryAfter
		if (code == "" || retryAfter == 0) && len(env.Data) > 0 {
			var detail struct {
				Code       string `json:"code"`
				RetryAfter int    `json:"retry_after"`
			}
			if json.Unmarshal(env.Data, &detail) == nil {
				if code == "" {
					code = detail.Code
				}
				if retryAfter == 0 {
					retryAfter = detail.RetryAfter
				}
			}
		}
		return nil, nil, &APIError{
			StatusCode: resp.StatusCode, Code: code,
			Message: msg, RetryAfter: retryAfter,
		}
	}
	return env.Data, resp.Header, nil
}

type invoiceJSON struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Amount     int    `json:"amount"`
	BaseAmount int    `json:"base_amount"`
	UniqueCode int    `json:"unique_code"`
	Label      string `json:"label"`
	QRPayload  string `json:"qr_payload"`
	ExpiresAt  string `json:"expires_at"`
	PaidAt     string `json:"paid_at"`
}

func toInvoice(j invoiceJSON) Invoice {
	return Invoice{
		ID: j.ID, Status: j.Status, Amount: j.Amount,
		BaseAmount: j.BaseAmount, UniqueCode: j.UniqueCode,
		Label: j.Label, QRPayload: j.QRPayload,
		ExpiresAt: parseTimePtr(j.ExpiresAt), PaidAt: parseTimePtr(j.PaidAt),
	}
}

// CreateInvoice membuat invoice baru sebesar amount dengan label opsional.
func (c *Client) CreateInvoice(ctx context.Context, amount int, label string) (Invoice, error) {
	data, _, err := c.do(ctx, http.MethodPost, pathInvoices, "", map[string]any{
		"amount": amount, "label": label,
	})
	if err != nil {
		return Invoice{}, err
	}
	var j invoiceJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Invoice{}, err
	}
	return toInvoice(j), nil
}

// GetInvoice mengambil detail satu invoice.
func (c *Client) GetInvoice(ctx context.Context, id string) (Invoice, error) {
	data, _, err := c.do(ctx, http.MethodGet, pathInvoices+"/"+id, "", nil)
	if err != nil {
		return Invoice{}, err
	}
	var j invoiceJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Invoice{}, err
	}
	return toInvoice(j), nil
}

// BatchStatus menanyakan status banyak invoice sekaligus (maks 50 id).
func (c *Client) BatchStatus(ctx context.Context, ids []string) ([]InvoiceStatus, error) {
	q := url.Values{"ids": {strings.Join(ids, ",")}}.Encode()
	data, _, err := c.do(ctx, http.MethodGet, pathStatus, q, nil)
	if err != nil {
		return nil, err
	}
	var arr []struct {
		ID     string `json:"id"`
		Found  *bool  `json:"found"`
		Status string `json:"status"`
		PaidAt string `json:"paid_at"`
	}
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	out := make([]InvoiceStatus, 0, len(arr))
	for _, a := range arr {
		found := true
		if a.Found != nil {
			found = *a.Found
		}
		out = append(out, InvoiceStatus{
			ID: a.ID, Found: found, Status: a.Status, PaidAt: parseTimePtr(a.PaidAt),
		})
	}
	return out, nil
}

// CancelInvoice membatalkan invoice yang masih pending.
func (c *Client) CancelInvoice(ctx context.Context, id string) error {
	_, _, err := c.do(ctx, http.MethodPost, pathInvoices+"/"+id+"/cancel", "", map[string]any{})
	return err
}
