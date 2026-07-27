// Package razorpay implements provider.Provider against Razorpay's REST API.
//
// It talks to the API directly rather than through an SDK — the surface the
// kit needs is four calls, and a hand-rolled client keeps the dependency list
// short and the behaviour easy to read.
package razorpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/marketkit/api/internal/payments/provider"
)

const (
	// Name is the provider's stable identifier.
	Name = "razorpay"

	defaultBaseURL = "https://api.razorpay.com/v1"
	requestTimeout = 30 * time.Second
)

// Config holds the credentials Razorpay needs.
type Config struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string

	// BaseURL overrides the API root. Tests set this; leave empty in production.
	BaseURL string
	// HTTPClient overrides the client. Tests set this; leave nil in production.
	HTTPClient *http.Client
}

// Client implements provider.Provider.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Razorpay client. It does not verify the credentials — an
// unconfigured key surfaces as an error on the first API call, and an
// unconfigured webhook secret rejects every webhook.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: requestTimeout}
	}
	return &Client{cfg: cfg, http: hc}
}

func (c *Client) Name() string { return Name }

// CreateOrder creates a Razorpay order. Razorpay's checkout keys off the order
// id alone, so no client secret is returned.
func (c *Client) CreateOrder(ctx context.Context, req provider.OrderRequest) (provider.Order, error) {
	if c.cfg.KeyID == "" || c.cfg.KeySecret == "" {
		return provider.Order{}, fmt.Errorf("%w: RAZORPAY_KEY_ID / RAZORPAY_KEY_SECRET are empty", provider.ErrNotConfigured)
	}
	if req.AmountMinor <= 0 {
		return provider.Order{}, fmt.Errorf("razorpay: amount must be > 0, got %d", req.AmountMinor)
	}

	payload := map[string]any{
		"amount":   req.AmountMinor,
		"currency": req.Currency,
		"receipt":  req.Receipt,
	}
	if len(req.Notes) > 0 {
		payload["notes"] = req.Notes
	}

	result, err := c.do(ctx, http.MethodPost, "/orders", payload)
	if err != nil {
		return provider.Order{}, err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return provider.Order{}, fmt.Errorf("razorpay: response contained no order id")
	}
	currency, _ := result["currency"].(string)
	return provider.Order{ID: id, Currency: currency, Raw: result}, nil
}

// VerifyCheckout validates HMAC_SHA256("<order_id>|<payment_id>", key_secret),
// which is what Razorpay's client SDK returns after a successful payment.
func (c *Client) VerifyCheckout(res provider.CheckoutResult) error {
	if c.cfg.KeySecret == "" {
		return fmt.Errorf("%w: RAZORPAY_KEY_SECRET is empty", provider.ErrNotConfigured)
	}
	if res.OrderID == "" || res.PaymentID == "" || res.Signature == "" {
		return fmt.Errorf("%w: order id, payment id and signature are all required", provider.ErrInvalidSignature)
	}

	mac := hmac.New(sha256.New, []byte(c.cfg.KeySecret))
	mac.Write([]byte(res.OrderID + "|" + res.PaymentID))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(res.Signature), []byte(expected)) {
		return provider.ErrInvalidSignature
	}
	return nil
}

// ParseWebhook verifies X-Razorpay-Signature over the raw body and normalizes
// payment.captured. It fails closed when the webhook secret is unset: with an
// empty key the HMAC is trivially forgeable by anyone.
func (c *Client) ParseWebhook(body []byte, headers map[string]string) (provider.Event, error) {
	if c.cfg.WebhookSecret == "" {
		return provider.Event{}, fmt.Errorf("%w: RAZORPAY_WEBHOOK_SECRET is empty", provider.ErrNotConfigured)
	}

	signature := headerLookup(headers, "X-Razorpay-Signature")
	mac := hmac.New(sha256.New, []byte(c.cfg.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return provider.Event{}, provider.ErrInvalidSignature
	}

	var envelope struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity map[string]any `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return provider.Event{}, fmt.Errorf("razorpay: malformed webhook body: %w", err)
	}

	if envelope.Event != "payment.captured" {
		return provider.Event{Kind: provider.EventIgnored}, nil
	}

	entity := envelope.Payload.Payment.Entity
	ev := provider.Event{
		Kind:      provider.EventPaymentCaptured,
		OrderID:   stringField(entity, "order_id"),
		PaymentID: stringField(entity, "id"),
		Currency:  stringField(entity, "currency"),
		Raw:       entity,
	}
	if amt, ok := entity["amount"].(float64); ok {
		ev.AmountMinor = int64(amt)
	}
	if ev.OrderID == "" || ev.PaymentID == "" {
		return provider.Event{}, fmt.Errorf("razorpay: payment.captured missing order_id or id")
	}
	return ev, nil
}

// Refund refunds a captured payment.
func (c *Client) Refund(ctx context.Context, req provider.RefundRequest) (provider.Refund, error) {
	if c.cfg.KeyID == "" || c.cfg.KeySecret == "" {
		return provider.Refund{}, fmt.Errorf("%w: RAZORPAY_KEY_ID / RAZORPAY_KEY_SECRET are empty", provider.ErrNotConfigured)
	}
	if req.PaymentID == "" {
		return provider.Refund{}, fmt.Errorf("razorpay: payment id is required")
	}

	payload := map[string]any{
		"amount": req.AmountMinor,
		"speed":  "normal",
		"notes":  map[string]string{"reason": req.Reason},
	}
	result, err := c.do(ctx, http.MethodPost, "/payments/"+req.PaymentID+"/refund", payload)
	if err != nil {
		return provider.Refund{}, err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return provider.Refund{}, fmt.Errorf("razorpay: response contained no refund id")
	}
	return provider.Refund{ID: id, Raw: result}, nil
}

// do performs an authenticated JSON call and unwraps Razorpay's error shape.
func (c *Client) do(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.KeyID, c.cfg.KeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("razorpay: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("razorpay: reading response: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("razorpay: malformed response (status %d)", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Razorpay wraps failures as {"error": {"description": "..."}}.
		if errObj, ok := result["error"].(map[string]any); ok {
			if desc, _ := errObj["description"].(string); desc != "" {
				return nil, fmt.Errorf("razorpay: %s", desc)
			}
		}
		return nil, fmt.Errorf("razorpay: returned status %d", resp.StatusCode)
	}
	return result, nil
}

// headerLookup finds a header case-insensitively; Fiber and net/http differ in
// how they canonicalize keys.
func headerLookup(headers map[string]string, key string) string {
	if v, ok := headers[key]; ok {
		return v
	}
	for k, v := range headers {
		if equalFold(k, key) {
			return v
		}
	}
	return ""
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
