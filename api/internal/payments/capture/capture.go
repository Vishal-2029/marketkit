// Package capture routes a gateway's "payment captured" event to whichever
// module owns that order.
//
// Gateway order ids are account-unique but carry no hint of what was bought, so
// the webhook has to ask each module in turn: "is this yours?". This used to be
// a hardcoded if-chain inside the webhook handler, which meant the payments
// module imported every module that could take money — and made those modules
// impossible to remove.
//
// Now each module registers a handler at startup and the webhook just
// dispatches. Removing a module removes its registration and nothing else.
package capture

import (
	"log/slog"
	"sync"

	"github.com/marketkit/api/internal/payments/provider"
)

// Handler decides whether an event belongs to its module and, if so, captures
// it. Return true only when the order was found and processed — returning true
// stops dispatch, so a handler that claims an order it doesn't own will
// swallow another module's payment.
//
// A handler must be idempotent: gateways retry webhooks, and the same event can
// arrive more than once.
type Handler func(ev provider.Event) bool

type entry struct {
	name string
	fn   Handler
}

var (
	mu       sync.RWMutex
	handlers []entry
)

// Register adds a capture handler. Dispatch tries handlers in registration
// order, so register the most specific/likely first. Call during startup.
func Register(name string, fn Handler) {
	mu.Lock()
	defer mu.Unlock()
	handlers = append(handlers, entry{name: name, fn: fn})
}

// Dispatch offers ev to each handler until one claims it. The returned name is
// the claiming handler, for logging.
//
// handled == false is not necessarily an error: it also covers a duplicate
// webhook for an order that was already captured. Callers should still return
// 200 so the gateway stops retrying.
func Dispatch(ev provider.Event) (name string, handled bool) {
	mu.RLock()
	snapshot := make([]entry, len(handlers))
	copy(snapshot, handlers)
	mu.RUnlock()

	for _, h := range snapshot {
		if h.fn(ev) {
			slog.Info("payment captured", "handler", h.name, "order_id", ev.OrderID, "payment_id", ev.PaymentID)
			return h.name, true
		}
	}
	slog.Warn("payment captured but no handler claimed it",
		"order_id", ev.OrderID, "payment_id", ev.PaymentID, "handlers", namesLocked())
	return "", false
}

// Names lists registered handlers in dispatch order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return namesLocked()
}

func namesLocked() []string {
	out := make([]string, 0, len(handlers))
	for _, h := range handlers {
		out = append(out, h.name)
	}
	return out
}

// Reset clears all handlers. Tests and re-initialization only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	handlers = nil
}
