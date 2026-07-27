package razorpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marketkit/api/internal/payments/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestCreateOrder(t *testing.T) {
	var gotAuthUser, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthUser, _, _ = r.BasicAuth()
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "order_abc123", "currency": "INR"})
	}))
	defer srv.Close()

	c := New(Config{KeyID: "key", KeySecret: "secret", BaseURL: srv.URL})
	order, err := c.CreateOrder(context.Background(), provider.OrderRequest{
		AmountMinor: 99900, Currency: "INR", Receipt: "rcpt_1",
		Notes: map[string]string{"plan": "pro"},
	})

	require.NoError(t, err)
	assert.Equal(t, "order_abc123", order.ID)
	assert.Equal(t, "INR", order.Currency)
	assert.Empty(t, order.ClientSecret, "razorpay checkout keys off the order id alone")

	assert.Equal(t, "key", gotAuthUser, "must authenticate with the key id")
	assert.Equal(t, "/orders", gotPath)
	assert.EqualValues(t, 99900, gotBody["amount"])
	assert.Equal(t, "INR", gotBody["currency"])
	assert.Equal(t, "rcpt_1", gotBody["receipt"])
	assert.NotNil(t, gotBody["notes"])
}

func TestCreateOrder_Rejects(t *testing.T) {
	t.Run("unconfigured credentials", func(t *testing.T) {
		c := New(Config{})
		_, err := c.CreateOrder(context.Background(), provider.OrderRequest{AmountMinor: 100, Currency: "INR"})
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})

	t.Run("non-positive amount", func(t *testing.T) {
		c := New(Config{KeyID: "k", KeySecret: "s"})
		_, err := c.CreateOrder(context.Background(), provider.OrderRequest{AmountMinor: 0, Currency: "INR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "amount must be > 0")
	})

	t.Run("gateway error description is surfaced", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"description": "amount exceeds maximum"},
			})
		}))
		defer srv.Close()

		c := New(Config{KeyID: "k", KeySecret: "s", BaseURL: srv.URL})
		_, err := c.CreateOrder(context.Background(), provider.OrderRequest{AmountMinor: 1, Currency: "INR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "amount exceeds maximum")
	})

	t.Run("missing order id in response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"currency": "INR"})
		}))
		defer srv.Close()

		c := New(Config{KeyID: "k", KeySecret: "s", BaseURL: srv.URL})
		_, err := c.CreateOrder(context.Background(), provider.OrderRequest{AmountMinor: 1, Currency: "INR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no order id")
	})
}

func TestVerifyCheckout(t *testing.T) {
	const secret = "test_secret"
	c := New(Config{KeyID: "k", KeySecret: secret})

	valid := provider.CheckoutResult{
		OrderID:   "order_1",
		PaymentID: "pay_1",
		Signature: sign(secret, "order_1|pay_1"),
	}
	assert.NoError(t, c.VerifyCheckout(valid))

	t.Run("tampered signature", func(t *testing.T) {
		bad := valid
		bad.Signature = sign("wrong_secret", "order_1|pay_1")
		assert.ErrorIs(t, c.VerifyCheckout(bad), provider.ErrInvalidSignature)
	})

	t.Run("swapped ids do not verify", func(t *testing.T) {
		bad := valid
		bad.OrderID, bad.PaymentID = valid.PaymentID, valid.OrderID
		assert.ErrorIs(t, c.VerifyCheckout(bad), provider.ErrInvalidSignature)
	})

	t.Run("missing fields", func(t *testing.T) {
		assert.ErrorIs(t, c.VerifyCheckout(provider.CheckoutResult{}), provider.ErrInvalidSignature)
	})

	t.Run("unconfigured secret never passes", func(t *testing.T) {
		empty := New(Config{})
		err := empty.VerifyCheckout(valid)
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})
}

func webhookBody(t *testing.T, event string, entity map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"event":   event,
		"payload": map[string]any{"payment": map[string]any{"entity": entity}},
	})
	require.NoError(t, err)
	return b
}

func TestParseWebhook(t *testing.T) {
	const secret = "wh_secret"
	c := New(Config{WebhookSecret: secret})

	body := webhookBody(t, "payment.captured", map[string]any{
		"id": "pay_9", "order_id": "order_9", "amount": float64(50000), "currency": "INR",
	})
	headers := map[string]string{"X-Razorpay-Signature": sign(secret, string(body))}

	ev, err := c.ParseWebhook(body, headers)
	require.NoError(t, err)
	assert.Equal(t, provider.EventPaymentCaptured, ev.Kind)
	assert.Equal(t, "order_9", ev.OrderID)
	assert.Equal(t, "pay_9", ev.PaymentID)
	assert.EqualValues(t, 50000, ev.AmountMinor)
	assert.Equal(t, "INR", ev.Currency)
	assert.NotNil(t, ev.Raw)

	t.Run("header lookup is case-insensitive", func(t *testing.T) {
		lower := map[string]string{"x-razorpay-signature": sign(secret, string(body))}
		ev, err := c.ParseWebhook(body, lower)
		require.NoError(t, err)
		assert.Equal(t, provider.EventPaymentCaptured, ev.Kind)
	})

	t.Run("other events are ignored, not errors", func(t *testing.T) {
		b := webhookBody(t, "payment.failed", map[string]any{"id": "pay_x"})
		ev, err := c.ParseWebhook(b, map[string]string{"X-Razorpay-Signature": sign(secret, string(b))})
		require.NoError(t, err)
		assert.Equal(t, provider.EventIgnored, ev.Kind)
	})
}

// The webhook is the only unauthenticated write path into the money layer, so
// every rejection route matters.
func TestParseWebhook_Rejects(t *testing.T) {
	const secret = "wh_secret"
	c := New(Config{WebhookSecret: secret})
	body := webhookBody(t, "payment.captured", map[string]any{"id": "p", "order_id": "o"})

	t.Run("unconfigured secret fails closed", func(t *testing.T) {
		unset := New(Config{})
		// Signed with an empty key — trivially forgeable if this were allowed.
		_, err := unset.ParseWebhook(body, map[string]string{"X-Razorpay-Signature": sign("", string(body))})
		require.Error(t, err)
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})

	t.Run("wrong signature", func(t *testing.T) {
		_, err := c.ParseWebhook(body, map[string]string{"X-Razorpay-Signature": sign("attacker", string(body))})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("missing signature header", func(t *testing.T) {
		_, err := c.ParseWebhook(body, map[string]string{})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("body tampered after signing", func(t *testing.T) {
		sig := sign(secret, string(body))
		tampered := webhookBody(t, "payment.captured", map[string]any{"id": "p", "order_id": "o", "amount": float64(1)})
		_, err := c.ParseWebhook(tampered, map[string]string{"X-Razorpay-Signature": sig})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("captured event missing ids", func(t *testing.T) {
		b := webhookBody(t, "payment.captured", map[string]any{"amount": float64(1)})
		_, err := c.ParseWebhook(b, map[string]string{"X-Razorpay-Signature": sign(secret, string(b))})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing order_id or id")
	})

	t.Run("malformed json", func(t *testing.T) {
		bad := []byte("{not json")
		_, err := c.ParseWebhook(bad, map[string]string{"X-Razorpay-Signature": sign(secret, string(bad))})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed webhook body")
	})
}

func TestRefund(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{"id": "rfnd_1"})
	}))
	defer srv.Close()

	c := New(Config{KeyID: "k", KeySecret: "s", BaseURL: srv.URL})
	r, err := c.Refund(context.Background(), provider.RefundRequest{
		PaymentID: "pay_1", AmountMinor: 25000, Reason: "duplicate",
	})

	require.NoError(t, err)
	assert.Equal(t, "rfnd_1", r.ID)
	assert.Equal(t, "/payments/pay_1/refund", gotPath)
	assert.EqualValues(t, 25000, gotBody["amount"])

	t.Run("requires payment id", func(t *testing.T) {
		_, err := c.Refund(context.Background(), provider.RefundRequest{AmountMinor: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payment id is required")
	})
}

func TestSatisfiesProviderInterface(t *testing.T) {
	var p provider.Provider = New(Config{})
	assert.Equal(t, "razorpay", p.Name())
}

func TestRegistry(t *testing.T) {
	provider.Reset()
	t.Cleanup(provider.Reset)

	provider.Register(New(Config{}))

	got, err := provider.Get("razorpay")
	require.NoError(t, err)
	assert.Equal(t, "razorpay", got.Name())

	_, err = provider.Get("stripe")
	assert.ErrorIs(t, err, provider.ErrUnknownProvider)
	assert.Equal(t, []string{"razorpay"}, provider.Names())

	assert.Panics(t, func() { provider.Register(New(Config{})) }, "duplicate registration is a bug")
}

var _ = errors.Is // keep errors imported for readability of assertions above
