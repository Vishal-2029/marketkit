package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleOAuthStateCookie = "google_oauth_state"

// generateOAuthState returns a random, unguessable CSRF token for the OAuth
// state parameter.
func generateOAuthState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// googleOAuthConfig builds the OAuth2 config from app config at runtime so
// the handler still works even if GOOGLE_CLIENT_ID is set after startup.
func googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.App.GoogleClientID,
		ClientSecret: config.App.GoogleClientSecret,
		RedirectURL:  config.App.GoogleRedirectURL,
		// Only the email scope is requested — the admin lookup below matches on
		// email alone, so there's no reason to ask Google for the full profile.
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}
}

// googleUserInfo is the subset of fields we care about from Google's userinfo API.
type googleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

// HandleGoogleLogin godoc
// @Summary     Redirect to Google OAuth consent screen
// @Tags        Admin Auth
// @Produce     json
// @Success     302
// @Router      /auth/google [get]
func HandleGoogleLogin(c *fiber.Ctx) error {
	cfg := googleOAuthConfig()
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return response.InternalError(c, "Google OAuth is not configured on this server")
	}

	state, err := generateOAuthState()
	if err != nil {
		return response.InternalError(c, "failed to start Google login")
	}
	sameSite := "Lax"
	if config.App.SecureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    state,
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: sameSite,
		MaxAge:   5 * 60,
	})

	// Use "online" access type so no refresh token is issued by Google (we
	// manage our own sessions).
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// HandleGoogleCallback godoc
// @Summary     Handle Google OAuth callback and log the admin in
// @Tags        Admin Auth
// @Produce     json
// @Success     302
// @Failure     401  {object}  map[string]string
// @Router      /auth/google/callback [get]
func HandleGoogleCallback(c *fiber.Ctx) error {
	cfg := googleOAuthConfig()

	// Verify the CSRF state before doing anything else — this must hold even
	// if a subsequent step below fails, so a stolen/replayed callback URL
	// from someone else's OAuth flow can't be used to log in as them.
	expectedState := c.Cookies(googleOAuthStateCookie)
	c.Cookie(&fiber.Cookie{Name: googleOAuthStateCookie, Value: "", MaxAge: -1})
	state := c.Query("state")
	if expectedState == "" || state == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		return redirectWithError(c, "Google login session expired or invalid. Please try again.")
	}

	// Exchange the authorisation code for a token.
	code := c.Query("code")
	if code == "" {
		return response.BadRequest(c, "missing OAuth code")
	}

	oauthToken, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("google oauth: code exchange failed", "error", err)
		return redirectWithError(c, "Google authentication failed. Please try again.")
	}

	// Fetch the authenticated user's profile from Google.
	info, err := fetchGoogleUserInfo(oauthToken.AccessToken)
	if err != nil {
		slog.Error("google oauth: fetch user info failed", "error", err)
		return redirectWithError(c, "Could not retrieve your Google profile.")
	}

	if !info.VerifiedEmail {
		return redirectWithError(c, "Your Google email address is not verified.")
	}

	// Look up the admin account — the Google email must already exist.
	var admin models.Admin
	if err := database.DB.
		Where("email = ? AND is_active = true", info.Email).
		First(&admin).Error; err != nil {
		slog.Warn("google oauth: no matching admin", "email", mask.Email(info.Email))
		return redirectWithError(c, "No admin account found for "+info.Email+". Access denied.")
	}

	// Issue our own JWT + refresh token (same as the OTP path).
	accessToken, refreshToken, err := issueTokensForAdmin(&admin)
	if err != nil {
		slog.Error("google oauth: token issuance failed", "error", err)
		return redirectWithError(c, "Login succeeded but session creation failed.")
	}

	// Audit log.
	adminID := admin.ID
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventLogin,
		ActorAdminID: &adminID,
		IPAddress:    c.IP(),
		Device:       c.Get("User-Agent"),
		Details:      models.JSONMap{"email": admin.Email, "method": "google"},
	})

	// Set the httpOnly refresh token cookie (same settings as HandleVerifyOTP).
	sameSite := "Lax"
	if config.App.SecureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken,
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: sameSite,
		MaxAge:   7 * 24 * 60 * 60,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})

	// Redirect the browser back to the frontend with the short-lived access token
	// in the query string. The frontend reads it, stores it in memory, and clears
	// the URL param — it never persists in localStorage.
	frontendURL := frontendRedirectURL()
	return c.Redirect(
		fmt.Sprintf("%s/?token=%s", frontendURL, accessToken),
		fiber.StatusTemporaryRedirect,
	)
}

// issueTokensForAdmin is a shared helper used by both the OTP and Google paths.
func issueTokensForAdmin(admin *models.Admin) (accessToken, refreshToken string, err error) {
	accessToken, err = issueAccessToken(admin)
	if err != nil {
		return "", "", fmt.Errorf("issue access token: %w", err)
	}
	refreshToken, err = issueRefreshToken(admin.ID)
	if err != nil {
		return "", "", fmt.Errorf("issue refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

// fetchGoogleUserInfo calls Google's userinfo endpoint with the given access token.
func fetchGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// frontendRedirectURL returns the public admin-panel origin for OAuth redirects.
// FRONTEND_URL is preferred; otherwise the first non-localhost CORS origin is used.
func frontendRedirectURL() string {
	if url := strings.TrimSpace(config.App.FrontendURL); url != "" {
		return strings.TrimRight(url, "/")
	}
	return deriveFrontendURL(config.App.CORSOrigin)
}

// deriveFrontendURL picks a redirect origin from CORS_ORIGIN for local dev.
// It never prefers localhost when a public (non-localhost) origin is available.
func deriveFrontendURL(corsOrigin string) string {
	origins := splitOrigins(corsOrigin)
	var localhostFallback string
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if isLocalhostOrigin(o) {
			if localhostFallback == "" {
				localhostFallback = o
			}
			continue
		}
		return strings.TrimRight(o, "/")
	}
	if localhostFallback != "" {
		return strings.TrimRight(localhostFallback, "/")
	}
	return "http://localhost:8081"
}

func isLocalhostOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	return strings.Contains(lower, "://localhost") ||
		strings.Contains(lower, "://127.0.0.1") ||
		strings.Contains(lower, "://[::1]")
}

func splitOrigins(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

// redirectWithError sends the browser back to the frontend login page with an
// error message in the query string so it can display a toast notification.
func redirectWithError(c *fiber.Ctx, msg string) error {
	frontendURL := frontendRedirectURL()
	return c.Redirect(
		fmt.Sprintf("%s/login?google_error=%s", frontendURL, msg),
		fiber.StatusTemporaryRedirect,
	)
}
