package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/marketkit/api/internal/payments/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedNow = time.Unix(1_700_000_000, 0)

func sign(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func sigHeader(secret string, ts time.Time, body string) string {
	t := fmt.Sprintf("%d", ts.Unix())
	return "t=" + t + ",v1=" + sign(secret, t, body)
}

func TestCreateOrder(t *testing.T) {
	var gotAuth, gotPath, gotContentType string
	var gotForm url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		r.ParseForm()
		gotForm = r.PostForm
		json.NewEncoder(w).Encode(map[string]any{
			"id": "pi_abc123", "client_secret": "pi_abc123_secret_xyz", "currency": "usd",
		})
	}))
	defer srv.Close()

	c := New(Config{SecretKey: "sk_test_123", BaseURL: srv.URL})
	order, err := c.CreateOrder(context.Background(), provider.OrderRequest{
		AmountMinor: 499900, Currency: "USD", Receipt: "rcpt_1",
		Notes: map[string]string{"plan_id": "p1"},
	})

	require.NoError(t, err)
	assert.Equal(t, "pi_abc123", order.ID)
	assert.Equal(t, "pi_abc123_secret_xyz", order.ClientSecret, "client SDK needs this to show the sheet")
	assert.Equal(t, "USD", order.Currency)

	assert.Equal(t, "Bearer sk_test_123", gotAuth)
	assert.Equal(t, "/payment_intents", gotPath)
	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType, "Stripe takes form encoding, not JSON")
	assert.Equal(t, "499900", gotForm.Get("amount"))
	assert.Equal(t, "usd", gotForm.Get("currency"), "Stripe requires a lowercase ISO code")
	assert.Equal(t, "true", gotForm.Get("automatic_payment_methods[enabled]"))
	assert.Equal(t, "p1", gotForm.Get("metadata[plan_id]"))
}

func TestCreateOrder_Rejects(t *testing.T) {
	t.Run("unconfigured key", func(t *testing.T) {
		_, err := New(Config{}).CreateOrder(context.Background(),
			provider.OrderRequest{AmountMinor: 100, Currency: "USD"})
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})

	t.Run("non-positive amount", func(t *testing.T) {
		_, err := New(Config{SecretKey: "sk"}).CreateOrder(context.Background(),
			provider.OrderRequest{AmountMinor: 0, Currency: "USD"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "amount must be > 0")
	})

	t.Run("gateway error message is surfaced", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "Amount must be at least $0.50 usd"},
			})
		}))
		defer srv.Close()
		_, err := New(Config{SecretKey: "sk", BaseURL: srv.URL}).CreateOrder(context.Background(),
			provider.OrderRequest{AmountMinor: 1, Currency: "USD"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least $0.50")
	})
}

// Stripe returns no client-side signature, so VerifyCheckout must ask Stripe
// whether the intent really succeeded. Trusting the client here would let any
// caller mark an arbitrary payment as paid.
func TestVerifyCheckout_AsksStripe(t *testing.T) {
	t.Run("succeeded intent passes", func(t *testing.T) {
		var gotPath, gotMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			json.NewEncoder(w).Encode(map[string]any{"id": "pi_1", "status": "succeeded"})
		}))
		defer srv.Close()

		c := New(Config{SecretKey: "sk", BaseURL: srv.URL})
		require.NoError(t, c.VerifyCheckout(provider.CheckoutResult{OrderID: "pi_1"}))
		assert.Equal(t, "/payment_intents/pi_1", gotPath)
		assert.Equal(t, http.MethodGet, gotMethod)
	})

	t.Run("unpaid intent is rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "pi_1", "status": "requires_payment_method"})
		}))
		defer srv.Close()

		err := New(Config{SecretKey: "sk", BaseURL: srv.URL}).
			VerifyCheckout(provider.CheckoutResult{OrderID: "pi_1"})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
		assert.Contains(t, err.Error(), "requires_payment_method")
	})

	t.Run("missing id", func(t *testing.T) {
		err := New(Config{SecretKey: "sk"}).VerifyCheckout(provider.CheckoutResult{})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("unconfigured key", func(t *testing.T) {
		err := New(Config{}).VerifyCheckout(provider.CheckoutResult{OrderID: "pi_1"})
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})
}

func intentBody(t *testing.T, evType, id string, amount float64) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": evType,
		"data": map[string]any{"object": map[string]any{
			"id": id, "amount": amount, "amount_received": amount, "currency": "usd",
		}},
	})
	require.NoError(t, err)
	return b
}

func TestParseWebhook(t *testing.T) {
	const secret = "whsec_test"
	c := New(Config{WebhookSecret: secret, Now: func() time.Time { return fixedNow }})

	body := intentBody(t, "payment_intent.succeeded", "pi_9", 50000)
	headers := map[string]string{"Stripe-Signature": sigHeader(secret, fixedNow, string(body))}

	ev, err := c.ParseWebhook(body, headers)
	require.NoError(t, err)
	assert.Equal(t, provider.EventPaymentCaptured, ev.Kind)
	assert.Equal(t, "pi_9", ev.OrderID)
	assert.Equal(t, "pi_9", ev.PaymentID, "the intent id is both the order ref and the refund handle")
	assert.EqualValues(t, 50000, ev.AmountMinor)
	assert.Equal(t, "USD", ev.Currency)

	t.Run("header lookup is case-insensitive", func(t *testing.T) {
		lower := map[string]string{"stripe-signature": sigHeader(secret, fixedNow, string(body))}
		ev, err := c.ParseWebhook(body, lower)
		require.NoError(t, err)
		assert.Equal(t, provider.EventPaymentCaptured, ev.Kind)
	})

	t.Run("other events are ignored, not errors", func(t *testing.T) {
		b := intentBody(t, "payment_intent.created", "pi_x", 1)
		ev, err := c.ParseWebhook(b, map[string]string{"Stripe-Signature": sigHeader(secret, fixedNow, string(b))})
		require.NoError(t, err)
		assert.Equal(t, provider.EventIgnored, ev.Kind)
	})

	// Stripe sends several v1 signatures while a signing secret is rotated;
	// any one matching is enough.
	t.Run("accepts one of several v1 signatures", func(t *testing.T) {
		ts := fmt.Sprintf("%d", fixedNow.Unix())
		header := "t=" + ts +
			",v1=" + sign("old_secret", ts, string(body)) +
			",v1=" + sign(secret, ts, string(body))
		ev, err := c.ParseWebhook(body, map[string]string{"Stripe-Signature": header})
		require.NoError(t, err)
		assert.Equal(t, provider.EventPaymentCaptured, ev.Kind)
	})
}

func TestParseWebhook_Rejects(t *testing.T) {
	const secret = "whsec_test"
	c := New(Config{WebhookSecret: secret, Now: func() time.Time { return fixedNow }})
	body := intentBody(t, "payment_intent.succeeded", "pi_1", 100)

	t.Run("unconfigured secret fails closed", func(t *testing.T) {
		unset := New(Config{Now: func() time.Time { return fixedNow }})
		_, err := unset.ParseWebhook(body, map[string]string{
			"Stripe-Signature": sigHeader("", fixedNow, string(body)),
		})
		assert.ErrorIs(t, err, provider.ErrNotConfigured)
	})

	t.Run("wrong secret", func(t *testing.T) {
		_, err := c.ParseWebhook(body, map[string]string{
			"Stripe-Signature": sigHeader("attacker", fixedNow, string(body)),
		})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("missing header", func(t *testing.T) {
		_, err := c.ParseWebhook(body, map[string]string{})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("malformed header", func(t *testing.T) {
		_, err := c.ParseWebhook(body, map[string]string{"Stripe-Signature": "garbage"})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("body tampered after signing", func(t *testing.T) {
		header := sigHeader(secret, fixedNow, string(body))
		tampered := intentBody(t, "payment_intent.succeeded", "pi_1", 999999)
		_, err := c.ParseWebhook(tampered, map[string]string{"Stripe-Signature": header})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	// A correctly-signed request captured off the wire must stop working once
	// it falls outside the tolerance window.
	t.Run("stale timestamp is a replay", func(t *testing.T) {
		old := fixedNow.Add(-10 * time.Minute)
		_, err := c.ParseWebhook(body, map[string]string{
			"Stripe-Signature": sigHeader(secret, old, string(body)),
		})
		require.ErrorIs(t, err, provider.ErrInvalidSignature)
		assert.Contains(t, err.Error(), "replay")
	})

	t.Run("far-future timestamp is rejected", func(t *testing.T) {
		future := fixedNow.Add(10 * time.Minute)
		_, err := c.ParseWebhook(body, map[string]string{
			"Stripe-Signature": sigHeader(secret, future, string(body)),
		})
		assert.ErrorIs(t, err, provider.ErrInvalidSignature)
	})

	t.Run("timestamp inside tolerance is accepted", func(t *testing.T) {
		recent := fixedNow.Add(-2 * time.Minute)
		_, err := c.ParseWebhook(body, map[string]string{
			"Stripe-Signature": sigHeader(secret, recent, string(body)),
		})
		assert.NoError(t, err)
	})

	t.Run("succeeded event missing id", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"type": "payment_intent.succeeded",
			"data": map[string]any{"object": map[string]any{"amount": 100.0}},
		})
		_, err := c.ParseWebhook(b, map[string]string{"Stripe-Signature": sigHeader(secret, fixedNow, string(b))})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing id")
	})
}

func TestRefund(t *testing.T) {
	var gotPath string
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		r.ParseForm()
		gotForm = r.PostForm
		json.NewEncoder(w).Encode(map[string]any{"id": "re_1"})
	}))
	defer srv.Close()

	c := New(Config{SecretKey: "sk", BaseURL: srv.URL})
	r, err := c.Refund(context.Background(), provider.RefundRequest{
		PaymentID: "pi_1", AmountMinor: 25000, Reason: "duplicate charge",
	})

	require.NoError(t, err)
	assert.Equal(t, "re_1", r.ID)
	assert.Equal(t, "/refunds", gotPath)
	assert.Equal(t, "pi_1", gotForm.Get("payment_intent"), "Stripe refunds key off the intent")
	assert.Equal(t, "25000", gotForm.Get("amount"))
	assert.Equal(t, "duplicate charge", gotForm.Get("metadata[reason]"),
		"free text goes to metadata; Stripe's reason enum only takes three values")

	t.Run("requires payment id", func(t *testing.T) {
		_, err := c.Refund(context.Background(), provider.RefundRequest{AmountMinor: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payment intent id is required")
	})
}

func TestSatisfiesProviderInterface(t *testing.T) {
	var p provider.Provider = New(Config{})
	assert.Equal(t, "stripe", p.Name())
}
