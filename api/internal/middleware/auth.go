package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
)

// PublishSecretAuth allows requests that carry the correct X-Publish-Secret header.
// Used by scripts/publish-apk.sh so it never needs an expiring JWT.
func PublishSecretAuth(c *fiber.Ctx) error {
	secret := config.App.PublishSecret
	if secret == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "publish secret not configured"})
	}
	given := c.Get("X-Publish-Secret")
	// Constant-time compare — a plain != leaks timing information proportional
	// to the matching prefix length, letting an attacker brute-force the
	// secret byte-by-byte.
	if len(given) != len(secret) || subtle.ConstantTimeCompare([]byte(given), []byte(secret)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid publish secret"})
	}
	return c.Next()
}

type AdminClaims struct {
	AdminID string `json:"sub"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// Authenticate validates the Bearer JWT and attaches claims to ctx.Locals.
func Authenticate(c *fiber.Ctx) error {
	var tokenStr string
	if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if tokenStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid authorization header"})
	}
	claims := &AdminClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.ErrUnauthorized
		}
		return []byte(config.App.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	// Reject user JWTs (role:"user") from admin endpoints.
	if claims.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
	}

	// A signature-valid, unexpired token from a deactivated or deleted admin
	// would otherwise keep working for its remaining ~15 min lifetime — there
	// is no per-token revocation, so check the account is still active on
	// every request (mirrors the DB check UserAuthenticate already does,
	// just against is_active rather than a rotating session ID — admins
	// intentionally support concurrent multi-device sessions, so a
	// single-session/session-ID model isn't a fit here).
	var admin models.Admin
	if err := database.DB.Select("is_active").
		Where("id = ?", claims.AdminID).First(&admin).Error; err != nil || !admin.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "account inactive"})
	}

	c.Locals("adminID", claims.AdminID)
	c.Locals("adminEmail", claims.Email)
	return c.Next()
}
