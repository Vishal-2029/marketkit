package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
)

// privatePrefixes are upload paths that must never be fetched without a valid
// signature. Everything else under /uploads (preview images, photos, video
// thumbnails, avatars) is public by design.
//
// Product files live here: they are the thing buyers pay for, so a plain
// public URL would hand out paid content to anyone holding the link.
var privatePrefixes = []string{"market/files/"}

// IsPrivateKey reports whether key requires a signature to fetch.
func IsPrivateKey(key string) bool {
	k := strings.TrimPrefix(key, "/")
	for _, p := range privatePrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// sign returns the HMAC for a key/expiry pair. The JWT secret is reused as the
// signing key — it is already required, already secret, and rotating it
// correctly invalidates outstanding download links too.
func sign(key string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(config.App.JWTSecret))
	mac.Write([]byte(key + "\n" + strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignQuery builds the "exp=…&sig=…" query string for a key.
func SignQuery(key string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return "exp=" + strconv.FormatInt(exp, 10) + "&sig=" + sign(key, exp)
}

// ProtectUploads guards the /uploads static mount.
//
// Public keys pass through untouched. Private keys require a signature that is
// both valid and unexpired, and are always sent as an attachment: an uploaded
// SVG served inline would otherwise execute script on the API's own origin,
// and Content-Type is genuinely image/svg+xml so nosniff does not help.
func ProtectUploads(c *fiber.Ctx) error {
	key := strings.TrimPrefix(c.Path(), "/uploads/")
	if !IsPrivateKey(key) {
		return c.Next()
	}

	exp, err := strconv.ParseInt(c.Query("exp"), 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "this download link is missing or has expired",
		})
	}
	if !hmac.Equal([]byte(c.Query("sig")), []byte(sign(key, exp))) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error":   "this download link is not valid",
		})
	}

	c.Set(fiber.HeaderContentDisposition, "attachment")
	return c.Next()
}
