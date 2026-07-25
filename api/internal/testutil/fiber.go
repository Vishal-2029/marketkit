package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// JSONBody marshals v for use as an httptest.NewRequest body.
func JSONBody(t *testing.T, v interface{}) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

// FiberApp returns a minimal Fiber app that injects the given Locals (e.g.
// "userID" or "adminID", matching what middleware.UserAuthenticate /
// middleware.Authenticate set from a real JWT) before dispatching to the
// handler under test — enough to exercise handlers end-to-end without a
// real token.
func FiberApp(locals map[string]string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		for k, v := range locals {
			c.Locals(k, v)
		}
		return c.Next()
	})
	return app
}
