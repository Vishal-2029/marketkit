// Package mask provides helpers for redacting PII before it reaches logs.
package mask

import "strings"

// Email masks all but the first character of the local part, e.g.
// "youruser@example.com" -> "v***@example.com". Used so logs can still be
// correlated to "which user" without persisting the full address in plaintext.
func Email(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return "[REDACTED]"
	}
	return email[:1] + "***" + email[at:]
}
