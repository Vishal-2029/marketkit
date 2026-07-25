package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
	"golang.org/x/crypto/bcrypt"
)

const otpTTL = 10 * time.Minute

var (
	ErrAdminNotFound      = errors.New("admin not found")
	ErrOTPInvalid         = errors.New("invalid or expired OTP")
	ErrEmailTaken         = errors.New("email already registered")
	ErrRefreshInvalid     = errors.New("invalid or expired refresh token")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// Register creates a new admin account and sends an OTP to verify the email.
func Register(firstName, lastName, phone, adminEmail, password string) error {
	adminEmail = strings.TrimSpace(strings.ToLower(adminEmail))
	var count int64
	database.DB.Model(&models.Admin{}).Where("LOWER(email) = ?", adminEmail).Count(&count)
	if count > 0 {
		return ErrEmailTaken
	}
	var userCount int64
	database.DB.Model(&models.User{}).Where("LOWER(email) = ?", adminEmail).Count(&userCount)
	if userCount > 0 {
		return ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	admin := models.Admin{
		FirstName:    firstName,
		LastName:     lastName,
		Phone:        phone,
		Email:        adminEmail,
		PasswordHash: string(hash),
		IsActive:     true,
	}
	if err := database.DB.Create(&admin).Error; err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	return sendOTPForAdmin(&admin)
}

// SendOTP verifies the password first, then sends OTP.
func SendOTP(adminEmail, password string) error {
	adminEmail = strings.TrimSpace(strings.ToLower(adminEmail))
	var admin models.Admin
	if err := database.DB.Where("LOWER(email) = ? AND is_active = true", adminEmail).First(&admin).Error; err != nil {
		return ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return sendOTPForAdmin(&admin)
}

// sendOTPForAdmin generates, stores, and emails a 6-digit OTP.
func sendOTPForAdmin(admin *models.Admin) error {
	database.DB.Where("admin_id = ? AND used = false", admin.ID).Delete(&models.OtpCode{})

	otp, err := generateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash otp: %w", err)
	}

	record := models.OtpCode{
		AdminID:   admin.ID,
		CodeHash:  string(hash),
		ExpiresAt: time.Now().Add(otpTTL),
	}
	if err := database.DB.Create(&record).Error; err != nil {
		return fmt.Errorf("store otp: %w", err)
	}

	if err := email.SendOTPEmail(admin.Email, admin.FullName(), otp); err != nil {
		log.Printf("⚠️  ADMIN OTP EMAIL FAILED for %s | Error: %v", mask.Email(admin.Email), err)
	}

	return nil
}

// VerifyOTP checks the OTP, marks it used, and returns a signed JWT + refresh token.
func VerifyOTP(adminEmail, inputOTP string) (accessToken string, refreshToken string, admin *models.Admin, err error) {
	adminEmail = strings.TrimSpace(strings.ToLower(adminEmail))
	var a models.Admin
	if err = database.DB.Where("LOWER(email) = ? AND is_active = true", adminEmail).First(&a).Error; err != nil {
		return "", "", nil, ErrOTPInvalid
	}

	var otpRecord models.OtpCode
	err = database.DB.
		Where("admin_id = ? AND used = false AND expires_at > ?", a.ID, time.Now()).
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

	// Detect first-ever login: count existing refresh tokens before issuing new one
	var priorTokenCount int64
	database.DB.Model(&models.RefreshToken{}).Where("admin_id = ?", a.ID).Count(&priorTokenCount)

	accessToken, err = issueAccessToken(&a)
	if err != nil {
		return "", "", nil, fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, err = issueRefreshToken(a.ID)
	if err != nil {
		return "", "", nil, fmt.Errorf("issue refresh token: %w", err)
	}

	if priorTokenCount == 0 {
		go email.SendWelcomeEmail(a.Email, a.FullName(), true)
	}

	return accessToken, refreshToken, &a, nil
}

// RefreshAccess validates the raw refresh token cookie value and returns a new
// access token. The cookie value format is "<record-uuid>.<random-secret>".
func RefreshAccess(rawToken string) (string, *models.Admin, error) {
	parts := strings.SplitN(rawToken, ".", 2)
	if len(parts) != 2 {
		return "", nil, ErrRefreshInvalid
	}
	id, secret := parts[0], parts[1]

	var rt models.RefreshToken
	if err := database.DB.First(&rt,
		"id = ? AND revoked = false AND expires_at > ?", id, time.Now(),
	).Error; err != nil {
		return "", nil, ErrRefreshInvalid
	}

	if bcrypt.CompareHashAndPassword([]byte(rt.TokenHash), []byte(secret)) != nil {
		return "", nil, ErrRefreshInvalid
	}

	var admin models.Admin
	if err := database.DB.First(&admin, "id = ? AND is_active = true", rt.AdminID).Error; err != nil {
		return "", nil, ErrRefreshInvalid
	}

	accessToken, err := issueAccessToken(&admin)
	if err != nil {
		return "", nil, fmt.Errorf("issue access token: %w", err)
	}

	return accessToken, &admin, nil
}

// RevokeRefreshToken marks the refresh token identified by the raw cookie value
// as revoked so it can no longer be used.
func RevokeRefreshToken(rawToken string) {
	parts := strings.SplitN(rawToken, ".", 2)
	if len(parts) != 2 {
		return
	}
	database.DB.Model(&models.RefreshToken{}).
		Where("id = ?", parts[0]).
		Update("revoked", true)
}

// issueAccessToken signs a short-lived (15 min) JWT for the given admin.
func issueAccessToken(admin *models.Admin) (string, error) {
	claims := jwt.MapClaims{
		"sub":   admin.ID,
		"email": admin.Email,
		"role":  "admin",
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.App.JWTSecret))
}

// issueRefreshToken generates a random token, stores its bcrypt hash, and
// returns the raw value in "<id>.<secret>" format for use as a cookie value.
func issueRefreshToken(adminID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(b)

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash refresh token: %w", err)
	}

	rt := models.RefreshToken{
		AdminID:   adminID,
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
