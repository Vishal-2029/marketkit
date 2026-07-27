package capture

import (
	"testing"

	"github.com/marketkit/api/internal/payments/provider"
	"github.com/stretchr/testify/assert"
)

func ev(orderID string) provider.Event {
	return provider.Event{Kind: provider.EventPaymentCaptured, OrderID: orderID, PaymentID: "pay_1"}
}

func TestDispatch_FirstClaimantWins(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var called []string
	Register("first", func(provider.Event) bool { called = append(called, "first"); return false })
	Register("second", func(provider.Event) bool { called = append(called, "second"); return true })
	Register("third", func(provider.Event) bool { called = append(called, "third"); return true })

	name, handled := Dispatch(ev("order_1"))

	assert.True(t, handled)
	assert.Equal(t, "second", name)
	assert.Equal(t, []string{"first", "second"}, called,
		"dispatch must stop at the first claimant — a later handler must not also process the payment")
}

func TestDispatch_NoClaimant(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register("a", func(provider.Event) bool { return false })
	Register("b", func(provider.Event) bool { return false })

	name, handled := Dispatch(ev("order_unknown"))

	// Not an error: this is also what a duplicate webhook for an
	// already-captured order looks like. The caller still returns 200.
	assert.False(t, handled)
	assert.Empty(t, name)
}

func TestDispatch_EmptyRegistry(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	name, handled := Dispatch(ev("order_1"))
	assert.False(t, handled)
	assert.Empty(t, name)
}

func TestDispatch_PassesEventThrough(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var got provider.Event
	Register("capture", func(e provider.Event) bool { got = e; return true })

	want := provider.Event{
		Kind: provider.EventPaymentCaptured, OrderID: "o1", PaymentID: "p1",
		AmountMinor: 12345, Currency: "INR", Raw: map[string]any{"k": "v"},
	}
	Dispatch(want)

	assert.Equal(t, want, got, "handlers receive the event unmodified")
}

func TestNamesAreInDispatchOrder(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register("learning_plan", func(provider.Event) bool { return false })
	Register("market_purchase", func(provider.Event) bool { return false })
	Register("wallet_topup", func(provider.Event) bool { return false })

	assert.Equal(t, []string{"learning_plan", "market_purchase", "wallet_topup"}, Names())
}

func TestReset(t *testing.T) {
	Reset()
	Register("x", func(provider.Event) bool { return true })
	assert.Len(t, Names(), 1)

	Reset()
	assert.Empty(t, Names())
}
