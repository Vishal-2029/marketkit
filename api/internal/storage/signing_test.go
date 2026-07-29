package storage

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	config.App = &config.Config{JWTSecret: "test-signing-secret"}
	m.Run()
}

func TestIsPrivateKey(t *testing.T) {
	assert.True(t, IsPrivateKey("market/files/abc.zip"), "product files are paid content")
	assert.True(t, IsPrivateKey("/market/files/abc.zip"))
	assert.False(t, IsPrivateKey("market/previews/abc.jpg"), "previews are meant to be public")
	assert.False(t, IsPrivateKey("photos/x.jpg"))
	assert.False(t, IsPrivateKey("avatars/x.jpg"))
}

// The whole point of the guard: a product file must not be fetchable without a
// valid, unexpired signature, or paid content is free to anyone with the URL.
func TestProtectUploads(t *testing.T) {
	app := fiber.New()
	app.Use("/uploads", ProtectUploads)
	app.Get("/uploads/*", func(c *fiber.Ctx) error { return c.SendString("FILE") })

	get := func(path string) *httptest.ResponseRecorder {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		rec.Code = resp.StatusCode
		for k, v := range resp.Header {
			rec.Header()[k] = v
		}
		return rec
	}

	const key = "market/files/secret.zip"

	t.Run("private key without a signature is refused", func(t *testing.T) {
		assert.Equal(t, fiber.StatusForbidden, get("/uploads/"+key).Code)
	})

	t.Run("forged signature is refused", func(t *testing.T) {
		exp := time.Now().Add(time.Hour).Unix()
		got := get("/uploads/" + key + "?exp=" + strconv.FormatInt(exp, 10) + "&sig=forged")
		assert.Equal(t, fiber.StatusForbidden, got.Code)
	})

	t.Run("signature for a different key is refused", func(t *testing.T) {
		other := "market/files/other.zip"
		got := get("/uploads/" + key + "?" + SignQuery(other, time.Hour))
		assert.Equal(t, fiber.StatusForbidden, got.Code, "a link for one file must not open another")
	})

	t.Run("expired signature is refused", func(t *testing.T) {
		got := get("/uploads/" + key + "?" + SignQuery(key, -time.Minute))
		assert.Equal(t, fiber.StatusForbidden, got.Code)
	})

	t.Run("valid signature passes and forces download", func(t *testing.T) {
		got := get("/uploads/" + key + "?" + SignQuery(key, time.Hour))
		assert.Equal(t, fiber.StatusOK, got.Code)
		assert.Equal(t, "attachment", got.Header().Get("Content-Disposition"),
			"an uploaded SVG served inline would execute script on the API origin")
	})

	t.Run("public keys are untouched", func(t *testing.T) {
		got := get("/uploads/market/previews/pic.jpg")
		assert.Equal(t, fiber.StatusOK, got.Code)
		assert.Empty(t, got.Header().Get("Content-Disposition"))
	})
}

func TestLocalSignedURL(t *testing.T) {
	l := newLocal("/tmp", "http://api.test")

	pub, err := l.SignedURL(t.Context(), "market/previews/a.jpg", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://api.test/uploads/market/previews/a.jpg", pub, "public keys need no signature")

	priv, err := l.SignedURL(t.Context(), "market/files/a.zip", time.Hour)
	require.NoError(t, err)
	assert.True(t, strings.Contains(priv, "sig="), "private keys must be signed: %s", priv)
	assert.True(t, strings.Contains(priv, "exp="), "private keys must expire: %s", priv)
}
