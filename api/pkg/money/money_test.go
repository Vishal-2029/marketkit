package money

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecimals(t *testing.T) {
	assert.Equal(t, 2, Decimals("INR"))
	assert.Equal(t, 2, Decimals("USD"))
	assert.Equal(t, 0, Decimals("JPY"), "yen has no minor unit")
	assert.Equal(t, 3, Decimals("KWD"), "dinar uses fils, 1/1000")
	assert.Equal(t, 2, Decimals("ZZZ"), "unknown currencies fall back to 2")
	assert.Equal(t, 0, Decimals("jpy"), "lookup is case-insensitive")
}

func TestSymbol(t *testing.T) {
	assert.Equal(t, "₹", Symbol("INR"))
	assert.Equal(t, "$", Symbol("USD"))
	assert.Equal(t, "€", Symbol("EUR"))
	assert.Equal(t, "CHF", Symbol("CHF"), "unmapped currencies show the ISO code")
	assert.Equal(t, "$", Symbol("usd"))
}

func TestFormat(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{499900, "INR", "₹4,999"},
		{499900, "USD", "$4,999"},
		{499950, "USD", "$4,999.50"},
		{499999, "USD", "$4,999.99"},
		{100, "USD", "$1"},
		{99, "USD", "$0.99"},
		{5, "USD", "$0.05"},
		{0, "USD", "$0"},
		{-250000, "USD", "$-2,500"},

		// Zero-decimal: the stored integer is already the major unit.
		{4999, "JPY", "¥4,999"},
		{100, "JPY", "¥100"},

		// Three-decimal.
		{1500, "KWD", "KWD1.500"},
		{1000, "KWD", "KWD1"},

		// Grouping boundaries.
		{100000, "USD", "$1,000"},
		{99999, "USD", "$999.99"},
		{100000000, "USD", "$1,000,000"},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, Format(c.minor, c.currency),
			"Format(%d, %q)", c.minor, c.currency)
	}
}

func TestFormatPlain(t *testing.T) {
	assert.Equal(t, "4,999", FormatPlain(499900, "INR"))
	assert.Equal(t, "4,999.50", FormatPlain(499950, "USD"))
	assert.Equal(t, "4,999", FormatPlain(4999, "JPY"))
}

func TestToMajor(t *testing.T) {
	assert.InDelta(t, 4999.0, ToMajor(499900, "INR"), 0.0001)
	assert.InDelta(t, 4999.5, ToMajor(499950, "USD"), 0.0001)
	assert.InDelta(t, 4999.0, ToMajor(4999, "JPY"), 0.0001, "no division for zero-decimal")
	assert.InDelta(t, 1.5, ToMajor(1500, "KWD"), 0.0001)
}

// The whole reason amounts are integers: a fee split must not drift.
func TestIntegerArithmeticIsExact(t *testing.T) {
	const price = int64(99900) // ₹999.00
	fee := price * 10 / 100    // 10% platform fee
	net := price - fee

	assert.Equal(t, int64(9990), fee)
	assert.Equal(t, int64(89910), net)
	assert.Equal(t, price, fee+net, "fee and net must sum back to the price exactly")
	assert.Equal(t, "₹99.90", Format(fee, "INR"))
	assert.Equal(t, "₹899.10", Format(net, "INR"))
}
