package user_auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"gorm.io/gorm"
)

var (
	ErrGoogleNotConfigured   = errors.New("google sign-in is not configured")
	ErrGoogleTokenInvalid    = errors.New("invalid google id token")
	ErrGoogleEmailUnverified = errors.New("google email is not verified")
	ErrGoogleAccountBlocked  = errors.New("account is not available for google sign-in")
)

// googleIDTokenInfo is the subset returned by Google's tokeninfo endpoint.
type googleIDTokenInfo struct {
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
}

// LoginWithGoogle verifies a Google ID token and logs the user in (creating
// an account on first use). Returns the same token pair as VerifyOTP.
func LoginWithGoogle(idToken, deviceInfo, ipAddress string) (accessToken, refreshToken string, user *models.User, err error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return "", "", nil, ErrGoogleTokenInvalid
	}

	info, err := verifyGoogleIDToken(idToken)
	if err != nil {
		return "", "", nil, err
	}

	email := strings.TrimSpace(strings.ToLower(info.Email))
	if email == "" {
		return "", "", nil, ErrGoogleTokenInvalid
	}
	if !isTruthy(info.EmailVerified) {
		return "", "", nil, ErrGoogleEmailUnverified
	}

	var u models.User
	err = database.DB.Where("LOWER(email) = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Block Google login for emails already used by an admin account.
		var adminCount int64
		database.DB.Model(&models.Admin{}).Where("LOWER(email) = ?", email).Count(&adminCount)
		if adminCount > 0 {
			return "", "", nil, ErrGoogleAccountBlocked
		}

		name := strings.TrimSpace(info.Name)
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		u = models.User{
			Name:         name,
			Email:        email,
			Phone:        "",
			PasswordHash: "", // Google-only accounts have no password until set later
			Status:       models.UserStatusActive,
		}
		if err = database.DB.Create(&u).Error; err != nil {
			return "", "", nil, fmt.Errorf("create google user: %w", err)
		}
	} else if err != nil {
		return "", "", nil, fmt.Errorf("lookup user: %w", err)
	} else if u.Status != models.UserStatusActive {
		return "", "", nil, ErrAccountInactive
	}

	accessToken, refreshToken, err = completeLogin(&u, deviceInfo, ipAddress)
	if err != nil {
		return "", "", nil, err
	}
	return accessToken, refreshToken, &u, nil
}

func verifyGoogleIDToken(idToken string) (*googleIDTokenInfo, error) {
	audiences := allowedGoogleAudiences()
	if len(audiences) == 0 {
		return nil, ErrGoogleNotConfigured
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken))
	if err != nil {
		return nil, fmt.Errorf("google tokeninfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read tokeninfo: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrGoogleTokenInvalid
	}

	var info googleIDTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, ErrGoogleTokenInvalid
	}
	if info.Aud == "" || info.Email == "" {
		return nil, ErrGoogleTokenInvalid
	}

	ok := false
	for _, aud := range audiences {
		if info.Aud == aud {
			ok = true
			break
		}
	}
	if !ok {
		return nil, ErrGoogleTokenInvalid
	}
	return &info, nil
}

func allowedGoogleAudiences() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range []string{
		config.App.GoogleWebClientID,
		config.App.GoogleClientID,
		config.App.GoogleAndroidClientID,
	} {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
