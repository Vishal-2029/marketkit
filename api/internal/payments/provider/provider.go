// Package provider defines the payment-gateway abstraction.
//
// Every gateway the kit supports implements [Provider]. Domain code (plans,
// marketplace purchases, wallet top-ups, refunds) talks only to this interface,
// so adding a gateway means writing one implementation and registering it —
// no changes in the modules that take money.
//
// Amounts are always in the currency's *minor unit* (paise for INR, cents for
// USD). The kit runs one currency per deployment, set by PAYMENT_CURRENCY.
package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Errors returned by implementations and the registry.
var (
	ErrUnknownProvider  = errors.New("payments: unknown provider")
	ErrNotConfigured    = errors.New("payments: provider is not configured")
	ErrInvalidSignature = errors.New("payments: invalid signature")
)

// OrderRequest asks a gateway to create a payable order.
type OrderRequest struct {
	AmountMinor int64  // amount in the currency's minor unit; must be > 0
	Currency    string // ISO-4217, e.g. "INR", "USD"
	Receipt     string // short merchant-side reference, shown in gateway dashboards
	Notes       map[string]string
}

// Order is a gateway-created order the client SDK can pay against.
type Order struct {
	ID       string         // gateway order/intent id, stored as Payment.ProviderOrderID
	Currency string         //
	Raw      map[string]any // full gateway response, for debugging
	// ClientSecret is set by gateways whose client SDK needs it (Stripe).
	// Gateways that key checkout off the order id alone (Razorpay) leave it empty.
	ClientSecret string
}

// CheckoutResult is what the client SDK hands back after the user pays, for
// gateways that support confirming a payment without waiting for the webhook.
type CheckoutResult struct {
	OrderID   string
	PaymentID string
	Signature string
}

// EventKind is the normalized set of webhook events the kit reacts to.
// Gateway-specific event names map onto these.
type EventKind string

const (
	// EventPaymentCaptured means money has been captured for an order.
	EventPaymentCaptured EventKind = "payment.captured"
	// EventIgnored is returned for events the kit does not act on. Handlers
	// must still respond 200 so the gateway stops retrying.
	EventIgnored EventKind = "ignored"
)

// Event is a gateway webhook normalized to the shape domain code needs.
type Event struct {
	Kind        EventKind
	OrderID     string
	PaymentID   string
	AmountMinor int64
	Currency    string
	Raw         map[string]any
}

// RefundRequest asks a gateway to refund a captured payment.
type RefundRequest struct {
	PaymentID   string
	AmountMinor int64
	Reason      string
}

// Refund is a gateway-created refund.
type Refund struct {
	ID  string
	Raw map[string]any
}

// Provider is a payment gateway. Implementations must be safe for concurrent
// use — one instance is shared across all requests.
type Provider interface {
	// Name is the stable identifier stored on Payment.Provider and used in
	// API routes (e.g. "razorpay", "stripe"). Lowercase, no spaces.
	Name() string

	// CreateOrder creates a payable order at the gateway.
	CreateOrder(ctx context.Context, req OrderRequest) (Order, error)

	// VerifyCheckout validates the signature the client SDK returns after a
	// successful payment. Returns ErrInvalidSignature when it does not match.
	// Gateways with no client-side signature return nil without checking —
	// for those the webhook is the only source of truth.
	VerifyCheckout(res CheckoutResult) error

	// ParseWebhook verifies the request's authenticity and normalizes it.
	// It must fail closed: an unconfigured signing secret is an error, never
	// a pass. Events the gateway sends but the kit ignores come back with
	// Kind == EventIgnored and no error.
	ParseWebhook(body []byte, headers map[string]string) (Event, error)

	// Refund refunds a captured payment, fully or partially.
	Refund(ctx context.Context, req RefundRequest) (Refund, error)
}

var (
	mu        sync.RWMutex
	providers = map[string]Provider{}
)

// Register makes p available under p.Name(). Called once per gateway during
// startup. Registering the same name twice panics — it is always a bug.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	name := p.Name()
	if _, dup := providers[name]; dup {
		panic(fmt.Sprintf("payments: provider %q registered twice", name))
	}
	providers[name] = p
}

// Get returns the provider registered under name.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q (registered: %v)", ErrUnknownProvider, name, namesLocked())
	}
	return p, nil
}

// Names lists registered providers, sorted, for diagnostics and health output.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return namesLocked()
}

func namesLocked() []string {
	out := make([]string, 0, len(providers))
	for n := range providers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Reset clears the registry. Tests only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	providers = map[string]Provider{}
}
