// Package stripe implements provider.Provider against Stripe's REST API.
//
// Like the Razorpay implementation it talks to the API directly rather than
// through the official SDK: the kit needs four calls, and a hand-rolled client
// keeps the dependency footprint small for something people vendor into their
// own products.
//
// Two Stripe-specific behaviours are worth knowing:
//
//   - A PaymentIntent is both the "order" and the thing that gets paid, so its
//     id (pi_…) is stored as ProviderOrderID *and* ProviderPaymentID. Refunds
//     accept a payment_intent, so this keeps the refund path working.
//   - Stripe has no client-side signature to verify. VerifyCheckout therefore
//     asks Stripe directly whether the intent actually succeeded rather than
//     trusting the client — see the comment on that method.
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/marketkit/api/internal/payments/provider"
)

const (
	// Name is the provider's stable identifier.
	Name = "stripe"

	defaultBaseURL = "https://api.stripe.com/v1"
	requestTimeout = 30 * time.Second

	// webhookTolerance is how far a webhook's timestamp may drift from now
	// before it is rejected as a replay. Stripe's own libraries default to
	// five minutes.
	webhookTolerance = 5 * time.Minute
)

// Config holds the credentials Stripe needs.
type Config struct {
	// SecretKey is the sk_test_… / sk_live_… key. Never expose it to clients.
	SecretKey string
	// WebhookSecret is the whsec_… signing secret from the endpoint settings.
	WebhookSecret string

	// BaseURL overrides the API root. Tests set this; leave empty in production.
	BaseURL string
	// HTTPClient overrides the client. Tests set this; leave nil in production.
	HTTPClient *http.Client
	// Now overrides the clock, for testing the webhook replay window.
	Now func() time.Time
}

// Client implements provider.Provider.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

// New builds a Stripe client. It does not verify the credentials — an
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
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Client{cfg: cfg, http: hc, now: now}
}

func (c *Client) Name() string { return Name }

// CreateOrder creates a PaymentIntent. The returned ClientSecret is what the
// client SDK (Stripe.js, flutter_stripe) needs to present the payment sheet.
func (c *Client) CreateOrder(ctx context.Context, req provider.OrderRequest) (provider.Order, error) {
	if c.cfg.SecretKey == "" {
		return provider.Order{}, fmt.Errorf("%w: STRIPE_SECRET_KEY is empty", provider.ErrNotConfigured)
	}
	if req.AmountMinor <= 0 {
		return provider.Order{}, fmt.Errorf("stripe: amount must be > 0, got %d", req.AmountMinor)
	}

	form := url.Values{}
	form.Set("amount", strconv.FormatInt(req.AmountMinor, 10))
	// Stripe requires a lowercase ISO code.
	form.Set("currency", strings.ToLower(req.Currency))
	// Let Stripe pick the payment methods enabled on the account rather than
	// hardcoding "card" — this is what makes wallets and local methods work.
	form.Set("automatic_payment_methods[enabled]", "true")
	if req.Receipt != "" {
		form.Set("description", req.Receipt)
		form.Set("metadata[receipt]", req.Receipt)
	}
	for k, v := range req.Notes {
		form.Set("metadata["+k+"]", v)
	}

	result, err := c.do(ctx, http.MethodPost, "/payment_intents", form)
	if err != nil {
		return provider.Order{}, err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return provider.Order{}, fmt.Errorf("stripe: response contained no payment intent id")
	}
	secret, _ := result["client_secret"].(string)
	currency, _ := result["currency"].(string)

	return provider.Order{
		ID:           id,
		Currency:     strings.ToUpper(currency),
		ClientSecret: secret,
		Raw:          result,
	}, nil
}

// VerifyCheckout confirms a payment with Stripe rather than trusting the
// client.
//
// Stripe returns no client-side HMAC, so there is nothing to check locally. A
// no-op here would let any authenticated caller claim an arbitrary intent had
// been paid, so instead this retrieves the PaymentIntent and requires that it
// exists and has actually succeeded. The webhook remains the authoritative
// path; this only lets the app confirm without waiting for it.
func (c *Client) VerifyCheckout(res provider.CheckoutResult) error {
	if c.cfg.SecretKey == "" {
		return fmt.Errorf("%w: STRIPE_SECRET_KEY is empty", provider.ErrNotConfigured)
	}
	if res.OrderID == "" {
		return fmt.Errorf("%w: payment intent id is required", provider.ErrInvalidSignature)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	result, err := c.do(ctx, http.MethodGet, "/payment_intents/"+url.PathEscape(res.OrderID), nil)
	if err != nil {
		return err
	}

	status, _ := result["status"].(string)
	if status != "succeeded" {
		return fmt.Errorf("%w: payment intent %s has status %q, not succeeded",
			provider.ErrInvalidSignature, res.OrderID, status)
	}
	return nil
}

// ParseWebhook verifies the Stripe-Signature header and normalizes
// payment_intent.succeeded.
//
// The header looks like "t=1700000000,v1=abc...,v1=def...". The signed payload
// is "<t>.<raw body>", and more than one v1 may be present while a signing
// secret is being rotated — any match is accepted. The timestamp is checked
// against a tolerance so a captured request cannot be replayed indefinitely.
func (c *Client) ParseWebhook(body []byte, headers map[string]string) (provider.Event, error) {
	if c.cfg.WebhookSecret == "" {
		return provider.Event{}, fmt.Errorf("%w: STRIPE_WEBHOOK_SECRET is empty", provider.ErrNotConfigured)
	}

	header := headerLookup(headers, "Stripe-Signature")
	if header == "" {
		return provider.Event{}, fmt.Errorf("%w: missing Stripe-Signature header", provider.ErrInvalidSignature)
	}

	timestamp, signatures := parseSignatureHeader(header)
	if timestamp == "" || len(signatures) == 0 {
		return provider.Event{}, fmt.Errorf("%w: malformed Stripe-Signature header", provider.ErrInvalidSignature)
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return provider.Event{}, fmt.Errorf("%w: unparsable timestamp", provider.ErrInvalidSignature)
	}
	if drift := c.now().Sub(time.Unix(ts, 0)); drift > webhookTolerance || drift < -webhookTolerance {
		return provider.Event{}, fmt.Errorf("%w: timestamp outside the %s tolerance (possible replay)",
			provider.ErrInvalidSignature, webhookTolerance)
	}

	mac := hmac.New(sha256.New, []byte(c.cfg.WebhookSecret))
	mac.Write([]byte(timestamp + "." + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))

	matched := false
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			matched = true
			break
		}
	}
	if !matched {
		return provider.Event{}, provider.ErrInvalidSignature
	}

	var envelope struct {
		Type string `json:"type"`
		Data struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return provider.Event{}, fmt.Errorf("stripe: malformed webhook body: %w", err)
	}

	if envelope.Type != "payment_intent.succeeded" {
		return provider.Event{Kind: provider.EventIgnored}, nil
	}

	obj := envelope.Data.Object
	id := stringField(obj, "id")
	if id == "" {
		return provider.Event{}, fmt.Errorf("stripe: payment_intent.succeeded missing id")
	}

	ev := provider.Event{
		Kind: provider.EventPaymentCaptured,
		// The PaymentIntent id is both the order reference we stored and the
		// handle refunds take, so it fills both fields.
		OrderID:   id,
		PaymentID: id,
		Currency:  strings.ToUpper(stringField(obj, "currency")),
		Raw:       obj,
	}
	if amt, ok := obj["amount_received"].(float64); ok && amt > 0 {
		ev.AmountMinor = int64(amt)
	} else if amt, ok := obj["amount"].(float64); ok {
		ev.AmountMinor = int64(amt)
	}
	return ev, nil
}

// Refund refunds a captured PaymentIntent.
func (c *Client) Refund(ctx context.Context, req provider.RefundRequest) (provider.Refund, error) {
	if c.cfg.SecretKey == "" {
		return provider.Refund{}, fmt.Errorf("%w: STRIPE_SECRET_KEY is empty", provider.ErrNotConfigured)
	}
	if req.PaymentID == "" {
		return provider.Refund{}, fmt.Errorf("stripe: payment intent id is required")
	}

	form := url.Values{}
	form.Set("payment_intent", req.PaymentID)
	if req.AmountMinor > 0 {
		form.Set("amount", strconv.FormatInt(req.AmountMinor, 10))
	}
	if req.Reason != "" {
		// Stripe's `reason` enum only accepts three values, so free text goes
		// to metadata where it stays visible in the dashboard.
		form.Set("metadata[reason]", req.Reason)
	}

	result, err := c.do(ctx, http.MethodPost, "/refunds", form)
	if err != nil {
		return provider.Refund{}, err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return provider.Refund{}, fmt.Errorf("stripe: response contained no refund id")
	}
	return provider.Refund{ID: id, Raw: result}, nil
}

// do performs an authenticated form-encoded call and unwraps Stripe's error
// shape. Stripe takes form encoding on input and returns JSON.
func (c *Client) do(ctx context.Context, method, path string, form url.Values) (map[string]any, error) {
	var bodyReader io.Reader
	if form != nil {
		bodyReader = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.SecretKey)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: reading response: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("stripe: malformed response (status %d)", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Stripe wraps failures as {"error": {"message": "..."}}.
		if errObj, ok := result["error"].(map[string]any); ok {
			if msg, _ := errObj["message"].(string); msg != "" {
				return nil, fmt.Errorf("stripe: %s", msg)
			}
		}
		return nil, fmt.Errorf("stripe: returned status %d", resp.StatusCode)
	}
	return result, nil
}

// parseSignatureHeader splits "t=123,v1=abc,v1=def" into its timestamp and the
// set of v1 signatures. Multiple v1 entries appear during secret rotation.
func parseSignatureHeader(header string) (timestamp string, signatures []string) {
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	return timestamp, signatures
}

func headerLookup(headers map[string]string, key string) string {
	if v, ok := headers[key]; ok {
		return v
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
