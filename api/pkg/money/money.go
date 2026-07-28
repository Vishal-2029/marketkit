// Package money formats amounts held in a currency's minor unit.
//
// Every monetary column in the kit stores an integer in the *minor unit* of the
// configured currency — paise for INR, cents for USD, yen for JPY. Integers
// avoid the rounding errors floats introduce once you start splitting platform
// fees, and the minor unit is what every payment gateway expects on the wire.
//
// The kit runs ONE currency per deployment, set by PAYMENT_CURRENCY. Supporting
// several at once would mean storing a currency alongside every amount and
// deciding how to convert when a seller's wallet mixes them — deliberately out
// of scope. Pick your currency before you take the first payment.
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// exponents holds currencies whose minor unit is not 1/100. Anything absent
// uses 2 decimal places, which covers the large majority of ISO-4217.
var exponents = map[string]int{
	// Zero-decimal: the minor unit *is* the major unit.
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "JPY": 0, "KMF": 0, "KRW": 0,
	"MGA": 0, "PYG": 0, "RWF": 0, "UGX": 0, "VND": 0, "VUV": 0, "XAF": 0,
	"XOF": 0, "XPF": 0,
	// Three-decimal.
	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "LYD": 3, "OMR": 3, "TND": 3,
}

// symbols are display prefixes for the currencies a kit buyer is most likely to
// use. Anything absent falls back to the ISO code, which is always correct if
// less pretty ("CHF 1,200.00").
var symbols = map[string]string{
	"AED": "د.إ", "AUD": "A$", "BRL": "R$", "CAD": "C$", "CNY": "¥",
	"EUR": "€", "GBP": "£", "HKD": "HK$", "IDR": "Rp", "ILS": "₪",
	"INR": "₹", "JPY": "¥", "KRW": "₩", "MXN": "MX$", "MYR": "RM",
	"NGN": "₦", "NZD": "NZ$", "PHP": "₱", "PLN": "zł", "RUB": "₽",
	"SAR": "﷼", "SEK": "kr", "SGD": "S$", "THB": "฿", "TRY": "₺",
	"USD": "$", "VND": "₫", "ZAR": "R",
}

// Decimals returns how many decimal places currency uses.
func Decimals(currency string) int {
	if e, ok := exponents[strings.ToUpper(currency)]; ok {
		return e
	}
	return 2
}

// Symbol returns the display symbol for currency, falling back to the ISO code.
func Symbol(currency string) string {
	code := strings.ToUpper(currency)
	if s, ok := symbols[code]; ok {
		return s
	}
	return code
}

// Format renders a minor-unit amount for display: Format(499900, "INR") is
// "₹4,999" and Format(499900, "USD") is "$4,999". Whole amounts drop the
// decimals; fractional amounts show them in full, so Format(499950, "USD") is
// "$4,999.50" and Format(499990, "USD") is "$4,999.90".
//
// For a zero-decimal currency the amount is the major unit already:
// Format(4999, "JPY") is "¥4,999".
func Format(minor int64, currency string) string {
	return Symbol(currency) + FormatPlain(minor, currency)
}

// FormatPlain is Format without the currency symbol, for places that render the
// symbol separately (tables with a currency column, CSV exports).
func FormatPlain(minor int64, currency string) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}

	dec := Decimals(currency)
	div := int64(1)
	for i := 0; i < dec; i++ {
		div *= 10
	}

	major := minor / div
	out := group(major)

	// A fractional part is shown in full — money reads as "99.90", never
	// "99.9". A whole amount drops the decimals entirely: "4,999".
	if dec > 0 {
		if frac := minor % div; frac != 0 {
			out += "." + fmt.Sprintf("%0*d", dec, frac)
		}
	}
	if neg {
		out = "-" + out
	}
	return out
}

// FormatASCII renders an amount using the ISO code instead of the symbol:
// "INR 4,999", "USD 4,999.50". Use it anywhere the output must stay Latin-1 —
// notably the generated PDFs, whose core fonts cannot draw ₹, ₩ or ₺ and would
// otherwise emit mojibake.
func FormatASCII(minor int64, currency string) string {
	return strings.ToUpper(currency) + " " + FormatPlain(minor, currency)
}

// ToMajor converts a minor-unit amount to its major-unit value. Use it only at
// the edges (display, third-party APIs that want decimals) — never for
// arithmetic, which must stay in integer minor units.
func ToMajor(minor int64, currency string) float64 {
	div := 1.0
	for i := 0; i < Decimals(currency); i++ {
		div *= 10
	}
	return float64(minor) / div
}

// group inserts thousands separators. Indian-numbering (1,00,000) is
// deliberately not special-cased: the kit ships one grouping style so output is
// predictable across every currency a buyer might pick.
func group(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
