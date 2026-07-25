package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
)

type UserClaims struct {
	UserID    string `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// UserAuthenticate validates a user JWT (role: "user"), attaches claims to
// ctx.Locals, and enforces single-session by checking the token's session ID
// against the current session ID stored on the user row.
func UserAuthenticate(c *fiber.Ctx) error {
	var tokenStr string
	if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if tokenStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid authorization header"})
	}

	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.ErrUnauthorized
		}
		return []byte(config.App.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	if claims.Role != "user" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "access denied"})
	}

	// Single-session check: confirm the session ID in the token matches the
	// current session ID on the user row. A mismatch means the account was
	// opened on another device and this session has been superseded.
	if claims.SessionID != "" {
		var user models.User
		if err := database.DB.Select("current_session_id").
			Where("id = ?", claims.UserID).First(&user).Error; err == nil {
			if user.CurrentSessionID != claims.SessionID {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "session_replaced",
				})
			}
		}
	}

	c.Locals("userID", claims.UserID)
	c.Locals("userEmail", claims.Email)
	return c.Next()
}
