package user_auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
	"golang.org/x/crypto/bcrypt"
)

const otpTTL = 10 * time.Minute

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrOTPInvalid         = errors.New("invalid or expired OTP")
	ErrEmailTaken         = errors.New("email already registered")
	ErrRefreshInvalid     = errors.New("invalid or expired refresh token")
	ErrAccountInactive    = errors.New("account is not active")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidAppMode     = errors.New("mode must be 'learning' or 'market'")
)

// parseDeviceInfo converts a raw User-Agent string into a human-readable device name.
// e.g. "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)..." → "iPhone · iOS 17"
func parseDeviceInfo(ua string) string {
	if ua == "" {
		return "Unknown Device"
	}

	ua = strings.TrimSpace(ua)
	uaLower := strings.ToLower(ua)

	// ── Detect OS ─────────────────────────────────────────────────────────────
	os := ""
	switch {
	case strings.Contains(uaLower, "iphone"):
		os = "iPhone"
		if idx := strings.Index(ua, "CPU iPhone OS "); idx != -1 {
			rest := ua[idx+14:]
			if end := strings.IndexByte(rest, ' '); end != -1 {
				ver := strings.ReplaceAll(rest[:end], "_", ".")
				// Keep only major.minor
				parts := strings.SplitN(ver, ".", 3)
				if len(parts) >= 2 {
					os = "iPhone · iOS " + parts[0] + "." + parts[1]
				} else {
					os = "iPhone · iOS " + parts[0]
				}
			}
		}
	case strings.Contains(uaLower, "ipad"):
		os = "iPad"
		if idx := strings.Index(ua, "CPU OS "); idx != -1 {
			rest := ua[idx+7:]
			if end := strings.IndexByte(rest, ' '); end != -1 {
				ver := strings.ReplaceAll(rest[:end], "_", ".")
				parts := strings.SplitN(ver, ".", 3)
				if len(parts) >= 1 {
					os = "iPad · iPadOS " + parts[0]
				}
			}
		}
	case strings.Contains(uaLower, "android"):
		os = "Android"
		if idx := strings.Index(ua, "Android "); idx != -1 {
			rest := ua[idx+8:]
			end := strings.IndexAny(rest, ";) ")
			if end == -1 {
				end = len(rest)
			}
			ver := rest[:end]
			parts := strings.SplitN(ver, ".", 3)
			os = "Android " + parts[0]
		}
	case strings.Contains(uaLower, "windows nt"):
		ntVer := ""
		if idx := strings.Index(uaLower, "windows nt "); idx != -1 {
			rest := ua[idx+11:]
			if end := strings.IndexAny(rest, ";) "); end != -1 {
				ntVer = rest[:end]
			}
		}
		switch ntVer {
		case "10.0":
			os = "Windows 10/11"
		case "6.3":
			os = "Windows 8.1"
		case "6.2":
			os = "Windows 8"
		case "6.1":
			os = "Windows 7"
		default:
			os = "Windows"
		}
	case strings.Contains(uaLower, "macintosh"), strings.Contains(uaLower, "mac os x"):
		os = "macOS"
		if idx := strings.Index(ua, "Mac OS X "); idx != -1 {
			rest := ua[idx+9:]
			if end := strings.IndexAny(rest, ";) "); end != -1 {
				ver := strings.ReplaceAll(rest[:end], "_", ".")
				parts := strings.SplitN(ver, ".", 3)
				if len(parts) >= 2 {
					os = "macOS " + parts[0] + "." + parts[1]
				}
			}
		}
	case strings.Contains(uaLower, "linux"):
		os = "Linux"
	default:
		os = "Unknown OS"
	}

	// ── Detect Browser / App ──────────────────────────────────────────────────
	app := ""
	switch {
	case strings.Contains(uaLower, "dart"):
		app = "Stitch Craft App"
	case strings.Contains(uaLower, "flutter"):
		app = "Stitch Craft App"
	case strings.Contains(uaLower, "edg/"):
		app = "Edge"
	case strings.Contains(uaLower, "opr/") || strings.Contains(uaLower, "opera"):
		app = "Opera"
	case strings.Contains(uaLower, "chrome/") && !strings.Contains(uaLower, "chromium"):
		app = "Chrome"
	case strings.Contains(uaLower, "firefox/"):
		app = "Firefox"
	case strings.Contains(uaLower, "safari/") && !strings.Contains(uaLower, "chrome"):
		app = "Safari"
	case strings.Contains(uaLower, "chromium"):
		app = "Chromium"
	}

	if app != "" {
		if os == "Unknown OS" {
			return app
		}
		return os + " · " + app
	}
	return os
}

// Register creates a new user account and sends a verification OTP. mode is
// chosen at the mode chooser before the registration form is even shown, so
// account creation is itself the account's first entry into that mode.
func Register(name, userEmail, phone, password, mode string) error {
	userEmail = strings.TrimSpace(strings.ToLower(userEmail))
	if mode != models.AppModeLearning && mode != models.AppModeMarket {
		return ErrInvalidAppMode
	}
	var count int64
	database.DB.Model(&models.User{}).Where("LOWER(email) = ?", userEmail).Count(&count)
	if count > 0 {
		return ErrEmailTaken
	}
	var adminCount int64
	database.DB.Model(&models.Admin{}).Where("LOWER(email) = ?", userEmail).Count(&adminCount)
	if adminCount > 0 {
		return ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	user := models.User{
		Name:           name,
		Email:          userEmail,
		Phone:          phone,
		PasswordHash:   string(hash),
		Status:         models.UserStatusActive,
		CurrentAppMode: mode,
	}
	if mode == models.AppModeMarket {
		user.MarketJoinedAt = &now
	} else {
		user.LearningJoinedAt = &now
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return sendOTPForUser(&user)
}

// SetAppMode switches an already-registered user's app mode. Used only for
// post-registration switches (e.g. from the profile tab) — registration
// itself sets the initial mode directly via Register. firstTimeInMode tells
// the caller whether this is the first time the account has ever entered the
// target mode, so the client knows whether to show the animated welcome
// popup; no email is sent from here.
func SetAppMode(userID, mode, ipAddress string) (user models.User, firstTimeInMode bool, err error) {
	if mode != models.AppModeLearning && mode != models.AppModeMarket {
		return models.User{}, false, ErrInvalidAppMode
	}
	if err = database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return models.User{}, false, err
	}
	if user.CurrentAppMode == mode {
		return user, false, nil
	}

	oldMode := user.CurrentAppMode
	now := time.Now()
	updates := map[string]interface{}{"current_app_mode": mode}
	if mode == models.AppModeMarket && user.MarketJoinedAt == nil {
		firstTimeInMode = true
		updates["market_joined_at"] = now
	} else if mode == models.AppModeLearning && user.LearningJoinedAt == nil {
		firstTimeInMode = true
		updates["learning_joined_at"] = now
	}

	if err = database.DB.Model(&user).Updates(updates).Error; err != nil {
		return models.User{}, false, fmt.Errorf("update app mode: %w", err)
	}
	user.CurrentAppMode = mode
	if firstTimeInMode {
		if mode == models.AppModeMarket {
			user.MarketJoinedAt = &now
		} else {
			user.LearningJoinedAt = &now
		}
	}

	database.DB.Create(&models.AuditLog{
		EventType:   models.EventAppModeSwitched,
		ActorUserID: &userID,
		Details:     models.JSONMap{"from": oldMode, "to": mode},
		IPAddress:   ipAddress,
	})

	return user, firstTimeInMode, nil
}

// SendOTP verifies the password, then emails an OTP.
// Always returns nil to the HTTP layer to prevent email enumeration.
func SendOTP(userEmail, password string) error {
	userEmail = strings.TrimSpace(strings.ToLower(userEmail))
	var user models.User
	if err := database.DB.Where("LOWER(email) = ? AND status = ?", userEmail, models.UserStatusActive).First(&user).Error; err != nil {
		return ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return sendOTPForUser(&user)
}

// sendOTPForUser generates, stores, and emails a 6-digit OTP.
func sendOTPForUser(user *models.User) error {
	database.DB.Where("user_id = ? AND used = false", user.ID).Delete(&models.UserOtpCode{})

	otp, err := generateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash otp: %w", err)
	}

	record := models.UserOtpCode{
		UserID:    user.ID,
		CodeHash:  string(hash),
		ExpiresAt: time.Now().Add(otpTTL),
	}
	if err := database.DB.Create(&record).Error; err != nil {
		return fmt.Errorf("store otp: %w", err)
	}

	if err := email.SendOTPEmail(user.Email, user.Name, otp); err != nil {
		log.Printf("⚠️  USER OTP EMAIL FAILED for %s | Error: %v", mask.Email(user.Email), err)
	}

	return nil
}

type geoResponse struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country_name"`
	Error   bool   `json:"error"`
}

func geolocateIP(ipAddress string) string {
	if ipAddress == "" {
		return ""
	}
	ipAddress = strings.TrimSpace(ipAddress)
	if strings.HasPrefix(ipAddress, "::ffff:") {
		ipAddress = strings.TrimPrefix(ipAddress, "::ffff:")
	}
	if ipAddress == "127.0.0.1" || ipAddress == "::1" || strings.HasPrefix(ipAddress, "127.") {
		return ""
	}

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://ipapi.co/%s/json/", ipAddress))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var geo geoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return ""
	}
	if geo.Error {
		return ""
	}

	parts := []string{}
	if geo.City != "" {
		parts = append(parts, geo.City)
	}
	if geo.Region != "" {
		parts = append(parts, geo.Region)
	}
	return strings.Join(parts, ", ")
}

// VerifyOTP checks the OTP, marks it used, and returns a JWT + refresh token.
// It also creates a new user session record for the current login.
func VerifyOTP(userEmail, inputOTP, deviceInfo, ipAddress string) (accessToken string, refreshToken string, user *models.User, err error) {
	userEmail = strings.TrimSpace(strings.ToLower(userEmail))
	inputOTP = strings.TrimSpace(inputOTP)
	if len(inputOTP) != 6 {
		return "", "", nil, ErrOTPInvalid
	}

	var u models.User
	if err = database.DB.Where("LOWER(email) = ? AND status = ?", userEmail, models.UserStatusActive).First(&u).Error; err != nil {
		return "", "", nil, ErrOTPInvalid
	}

	var otpRecord models.UserOtpCode
	err = database.DB.
		Where("user_id = ? AND used = false AND expires_at > ?", u.ID, time.Now()).
		Order("created_at DESC").
		First(&otpRecord).Error
	if err != nil {
		return "", "", nil, ErrOTPInvalid
	}

	if err = bcrypt.CompareHashAndPassword([]byte(otpRecord.CodeHash), []byte(inputOTP)); err != nil {
		return "", "", nil, ErrOTPInvalid
	}

	if err = database.DB.Model(&otpRecord).Update("used", true).Error; err != nil {
		return "", "", nil, fmt.Errorf("mark otp used: %w", err)
	}

	accessToken, refreshToken, err = completeLogin(&u, deviceInfo, ipAddress)
	if err != nil {
		return "", "", nil, err
	}
	return accessToken, refreshToken, &u, nil
}

// completeLogin enforces single-session login and returns JWT + refresh token.
func completeLogin(u *models.User, deviceInfo, ipAddress string) (accessToken, refreshToken string, err error) {
	var priorTokenCount int64
	database.DB.Model(&models.UserRefreshToken{}).Where("user_id = ?", u.ID).Count(&priorTokenCount)

	// Single-session enforcement: revoke all previous refresh tokens and rotate session ID.
	database.DB.Where("user_id = ?", u.ID).Delete(&models.UserRefreshToken{})
	newSessionID := uuid.New().String()
	if err = database.DB.Model(u).Update("current_session_id", newSessionID).Error; err != nil {
		return "", "", fmt.Errorf("rotate session: %w", err)
	}
	u.CurrentSessionID = newSessionID

	accessToken, err = issueAccessToken(u)
	if err != nil {
		return "", "", fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, err = issueRefreshToken(u.ID)
	if err != nil {
		return "", "", fmt.Errorf("issue refresh token: %w", err)
	}

	now := time.Now()
	database.DB.Model(&models.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", u.ID).
		Update("revoked_at", now)

	location := geolocateIP(ipAddress)
	session := models.UserSession{
		UserID:       u.ID,
		DeviceInfo:   parseDeviceInfo(deviceInfo),
		IPAddress:    ipAddress,
		Location:     location,
		JwtJti:       uuid.New().String(),
		LastActiveAt: time.Now(),
	}
	if err = database.DB.Create(&session).Error; err != nil {
		return "", "", fmt.Errorf("create user session: %w", err)
	}

	if priorTokenCount == 0 {
		switch u.CurrentAppMode {
		case models.AppModeMarket:
			go email.SendWelcomeMarketEmail(u.Email, u.Name)
		case models.AppModeLearning:
			go email.SendWelcomeLearningEmail(u.Email, u.Name)
		default:
			// Legacy accounts predating app-mode selection.
			go email.SendWelcomeEmail(u.Email, u.Name, false)
		}
	}

	return accessToken, refreshToken, nil
}

// ForgotPassword sends a reset OTP to the email if it exists (always silent).
func ForgotPassword(userEmail string) error {
	var user models.User
	if err := database.DB.Where("email = ? AND status = ?", userEmail, models.UserStatusActive).First(&user).Error; err != nil {
		return nil
	}
	return sendOTPForUser(&user)
}

// ResetPassword verifies the OTP then sets a new password.
func ResetPassword(userEmail, inputOTP, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password too short")
	}

	var u models.User
	if err := database.DB.Where("email = ? AND status = ?", userEmail, models.UserStatusActive).First(&u).Error; err != nil {
		return ErrOTPInvalid
	}

	var otpRecord models.UserOtpCode
	if err := database.DB.
		Where("user_id = ? AND used = false AND expires_at > ?", u.ID, time.Now()).
		Order("created_at DESC").
		First(&otpRecord).Error; err != nil {
		return ErrOTPInvalid
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otpRecord.CodeHash), []byte(inputOTP)); err != nil {
		return ErrOTPInvalid
	}

	if err := database.DB.Model(&otpRecord).Update("used", true).Error; err != nil {
		return fmt.Errorf("mark otp used: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := database.DB.Model(&u).Update("password_hash", string(hash)).Error; err != nil {
		return err
	}

	// A password reset means any session — including one an attacker who had
	// the old password may be holding — must not survive it. Mirror login's
	// single-session enforcement: revoke every outstanding refresh token and
	// rotate current_session_id so already-issued access tokens fail their
	// next sid check too.
	database.DB.Where("user_id = ?", u.ID).Delete(&models.UserRefreshToken{})
	database.DB.Model(&u).Update("current_session_id", uuid.New().String())

	return nil
}

// RefreshAccess validates a raw refresh token cookie value and returns a new
// access token. Cookie format: "<record-uuid>.<random-secret>".
func RefreshAccess(rawToken string) (string, *models.User, error) {
	parts := strings.SplitN(rawToken, ".", 2)
	if len(parts) != 2 {
		return "", nil, ErrRefreshInvalid
	}
	id, secret := parts[0], parts[1]

	var rt models.UserRefreshToken
	if err := database.DB.First(&rt,
		"id = ? AND revoked = false AND expires_at > ?", id, time.Now(),
	).Error; err != nil {
		return "", nil, ErrRefreshInvalid
	}

	if bcrypt.CompareHashAndPassword([]byte(rt.TokenHash), []byte(secret)) != nil {
		return "", nil, ErrRefreshInvalid
	}

	var user models.User
	if err := database.DB.First(&user, "id = ? AND status = ?", rt.UserID, models.UserStatusActive).Error; err != nil {
		return "", nil, ErrRefreshInvalid
	}

	accessToken, err := issueAccessToken(&user)
	if err != nil {
		return "", nil, fmt.Errorf("issue access token: %w", err)
	}

	return accessToken, &user, nil
}

// RevokeRefreshToken marks the refresh token as revoked.
func RevokeRefreshToken(rawToken string) {
	parts := strings.SplitN(rawToken, ".", 2)
	if len(parts) != 2 {
		return
	}

	var rt models.UserRefreshToken
	if err := database.DB.First(&rt, "id = ?", parts[0]).Error; err == nil {
		now := time.Now()
		database.DB.Model(&models.UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", rt.UserID).
			Update("revoked_at", now)

		// Rotate the session ID so any access token already issued for this
		// session (which embeds the old "sid") fails UserAuthenticate's
		// session check on its very next use, instead of staying valid for
		// its remaining ~15 min lifetime after logout.
		database.DB.Model(&models.User{}).
			Where("id = ?", rt.UserID).
			Update("current_session_id", uuid.New().String())
	}

	database.DB.Model(&models.UserRefreshToken{}).
		Where("id = ?", parts[0]).
		Update("revoked", true)
}

// issueAccessToken signs a 15-minute JWT with role: "user" and the current session ID.
func issueAccessToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"role":  "user",
		"sid":   user.CurrentSessionID,
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.App.JWTSecret))
}

// issueRefreshToken generates a random secret, stores its bcrypt hash, and
// returns "<id>.<secret>" as the cookie value.
func issueRefreshToken(userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(b)

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash refresh token: %w", err)
	}

	rt := models.UserRefreshToken{
		UserID:    userID,
		TokenHash: string(hash),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := database.DB.Create(&rt).Error; err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	return rt.ID + "." + secret, nil
}

func generateOTP() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}
