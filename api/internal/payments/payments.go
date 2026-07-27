// Package payments wires the configured gateway into the rest of the app.
//
// Domain code (plans, marketplace purchases, wallet top-ups, refunds) calls the
// helpers here rather than a gateway directly, so swapping or adding a gateway
// touches only Init and the provider implementations.
package payments

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/payments/provider"
	"github.com/marketkit/api/internal/payments/provider/razorpay"
)

// Init registers every compiled-in gateway and validates that the configured
// one exists. Call once at startup, after config.Load.
//
// Registration is deliberately unconditional: a gateway with empty credentials
// still registers and fails at call time with a clear error, which is easier to
// diagnose than a provider that silently isn't there.
func Init() error {
	provider.Reset() // makes Init safe to call twice (tests, hot reload)

	provider.Register(razorpay.New(razorpay.Config{
		KeyID:         config.App.RazorpayKeyID,
		KeySecret:     config.App.RazorpayKeySecret,
		WebhookSecret: config.App.RazorpayWebhookSecret,
	}))

	name := config.App.PaymentProvider
	if _, err := provider.Get(name); err != nil {
		return fmt.Errorf("PAYMENT_PROVIDER=%q is not a known gateway (available: %v)", name, provider.Names())
	}

	slog.Info("payments: initialized",
		"active", name, "currency", config.App.PaymentCurrency, "available", provider.Names())
	return nil
}

// Active returns the gateway selected by PAYMENT_PROVIDER.
func Active() (provider.Provider, error) {
	return provider.Get(config.App.PaymentProvider)
}

// Currency is the ISO-4217 code all amounts are charged in.
func Currency() string { return config.App.PaymentCurrency }

// CreateOrder creates an order on the active gateway. This is the single entry
// point every module uses to start a payment — previously each module had its
// own copy of this logic.
//
// receipt is a short merchant-side reference shown in gateway dashboards; notes
// are arbitrary key/values echoed back on the payment.
func CreateOrder(ctx context.Context, amountMinor int64, receipt string, notes map[string]string) (provider.Order, error) {
	p, err := Active()
	if err != nil {
		return provider.Order{}, err
	}
	return p.CreateOrder(ctx, provider.OrderRequest{
		AmountMinor: amountMinor,
		Currency:    Currency(),
		Receipt:     receipt,
		Notes:       notes,
	})
}

// VerifyCheckout validates the signature the client SDK returned.
func VerifyCheckout(res provider.CheckoutResult) error {
	p, err := Active()
	if err != nil {
		return err
	}
	return p.VerifyCheckout(res)
}

// Refund refunds a captured payment on the active gateway.
func Refund(ctx context.Context, paymentID string, amountMinor int64, reason string) (provider.Refund, error) {
	p, err := Active()
	if err != nil {
		return provider.Refund{}, err
	}
	return p.Refund(ctx, provider.RefundRequest{
		PaymentID:   paymentID,
		AmountMinor: amountMinor,
		Reason:      reason,
	})
}

// Receipt builds a gateway receipt reference from up to two ids. Gateways cap
// receipt length (Razorpay at 40 chars), so ids are truncated to 8 characters —
// enough to correlate while staying inside every gateway's limit.
func Receipt(prefix string, ids ...string) string {
	out := prefix
	for _, id := range ids {
		out += "_" + short(id)
	}
	return out
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
