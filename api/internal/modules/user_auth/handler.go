package user_auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/internal/subscriptions"
	"github.com/marketkit/api/pkg/mask"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var avatarMIMETypes = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

const maxAvatarBytes = 5 * 1024 * 1024 // 5 MB, matches community/market image limits

const userRefreshCookieName = "user_refresh_token"

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	// Mode is the app mode chosen before registration ("learning" or
	// "market") — the mode chooser now runs before the register form.
	Mode string `json:"mode"`
}

type sendOTPRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

// HandleRegister godoc
// @Summary     Register a new user
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  user_auth.registerRequest  true  "User registration payload"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Router      /auth/register [post]
func HandleRegister(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if req.Name == "" || req.Email == "" || req.Phone == "" || req.Password == "" {
		return response.BadRequest(c, "name, email, phone and password are required")
	}
	if len(req.Password) < 6 {
		return response.BadRequest(c, "password must be at least 6 characters")
	}
	if req.Mode != models.AppModeLearning && req.Mode != models.AppModeMarket {
		return response.BadRequest(c, "mode must be 'learning' or 'market'")
	}

	err := Register(req.Name, req.Email, req.Phone, req.Password, req.Mode)
	if err == ErrEmailTaken {
		return response.BadRequest(c, "an account with this email already exists")
	}
	if err != nil {
		return response.InternalError(c, "registration failed")
	}

	return response.OK(c, fiber.Map{
		"message": "Account created. Check your email for the OTP to verify.",
	})
}

// HandleSendOTP godoc
// @Summary     Send OTP for verification
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  user_auth.sendOTPRequest  true  "Email and password payload"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/send-otp [post]
func HandleSendOTP(c *fiber.Ctx) error {
	var req sendOTPRequest
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "email and password are required")
	}

	if err := SendOTP(req.Email, req.Password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return response.Unauthorized(c, "Invalid email or password.")
		}
		slog.Error("user/send-otp: error", "email", mask.Email(req.Email), "error", err)
		return response.InternalError(c, "Failed to send OTP. Please try again.")
	}
	return response.OK(c, fiber.Map{"message": "OTP sent to your email."})
}

// HandleVerifyOTP godoc
// @Summary     Verify OTP and login
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  user_auth.verifyOTPRequest  true  "Email and OTP payload"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/verify-otp [post]
func HandleVerifyOTP(c *fiber.Ctx) error {
	var req verifyOTPRequest
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.OTP == "" {
		return response.BadRequest(c, "email and otp are required")
	}

	accessToken, refreshToken, user, err := VerifyOTP(req.Email, req.OTP, c.Get("User-Agent"), c.IP())
	if err != nil {
		if errors.Is(err, ErrOTPInvalid) {
			return response.Unauthorized(c, "invalid or expired OTP")
		}
		slog.Error("user/verify-otp: error", "email", mask.Email(req.Email), "error", err)
		return response.InternalError(c, "login failed after OTP verification")
	}

	sameSite := "Lax"
	if config.App.SecureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     userRefreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: sameSite,
		MaxAge:   7 * 24 * 60 * 60,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})

	return response.OK(c, fiber.Map{
		"token":         accessToken,
		"refresh_token": refreshToken,
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"current_app_mode": user.CurrentAppMode,
		},
	})
}

// HandleGoogleLogin godoc
// @Summary     Sign in (or register) with a Google ID token
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  object  true  "id_token from Google Sign-In"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/auth/google [post]
func HandleGoogleLogin(c *fiber.Ctx) error {
	var req struct {
		IDToken string `json:"id_token"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.IDToken) == "" {
		return response.BadRequest(c, "id_token is required")
	}

	accessToken, refreshToken, user, err := LoginWithGoogle(req.IDToken, c.Get("User-Agent"), c.IP())
	if err != nil {
		switch {
		case errors.Is(err, ErrGoogleNotConfigured):
			return response.InternalError(c, "Google Sign-In is not configured on this server")
		case errors.Is(err, ErrGoogleTokenInvalid):
			return response.Unauthorized(c, "Invalid Google sign-in. Please try again.")
		case errors.Is(err, ErrGoogleEmailUnverified):
			return response.Unauthorized(c, "Your Google email is not verified.")
		case errors.Is(err, ErrAccountInactive):
			return response.Unauthorized(c, "This account is not active.")
		case errors.Is(err, ErrGoogleAccountBlocked):
			return response.Unauthorized(c, "This email cannot be used for app sign-in.")
		default:
			slog.Error("user/google: error", "error", err)
			return response.InternalError(c, "Google sign-in failed. Please try again.")
		}
	}

	sameSite := "Lax"
	if config.App.SecureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     userRefreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: sameSite,
		MaxAge:   7 * 24 * 60 * 60,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})

	return response.OK(c, fiber.Map{
		"token":         accessToken,
		"refresh_token": refreshToken,
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"current_app_mode": user.CurrentAppMode,
		},
	})
}

// HandleRefresh godoc
// @Summary     Refresh access token
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/refresh [post]
func HandleRefresh(c *fiber.Ctx) error {
	raw := c.Cookies(userRefreshCookieName)
	if raw == "" {
		// Mobile clients cannot send httpOnly cookies — accept token from body.
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.BodyParser(&body)
		raw = body.RefreshToken
	}
	if raw == "" {
		return response.Unauthorized(c, "no refresh token")
	}

	accessToken, user, err := RefreshAccess(raw)
	if err != nil {
		clearSameSite := "Lax"
		if config.App.SecureCookie {
			clearSameSite = "None"
		}
		c.Cookie(&fiber.Cookie{
			Name:     userRefreshCookieName,
			Value:    "",
			Path:     "/",
			HTTPOnly: true,
			Secure:   config.App.SecureCookie,
			SameSite: clearSameSite,
			MaxAge:   -1,
			Expires:  time.Now().Add(-1 * time.Hour),
		})
		return response.Unauthorized(c, "invalid or expired session — please log in again")
	}

	return response.OK(c, fiber.Map{
		"token": accessToken,
		"user": fiber.Map{
			"id":               user.ID,
			"name":             user.Name,
			"email":            user.Email,
			"phone":            user.Phone,
			"current_app_mode": user.CurrentAppMode,
		},
	})
}

// HandleLogout godoc
// @Summary     Logout user
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Success     200  {object}  map[string]string
// @Router      /auth/logout [post]
func HandleLogout(c *fiber.Ctx) error {
	raw := c.Cookies(userRefreshCookieName)
	if raw == "" {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.BodyParser(&body)
		raw = body.RefreshToken
	}
	if raw != "" {
		RevokeRefreshToken(raw)
	}

	clearSameSite := "Lax"
	if config.App.SecureCookie {
		clearSameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     userRefreshCookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: clearSameSite,
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
	})

	return response.OK(c, fiber.Map{"message": "logged out"})
}

// HandleMe godoc
// @Summary     Get current user profile
// @Tags        User Auth
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /auth/me [get]
func HandleMe(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	now := time.Now()

	var user models.User
	if err := database.DB.
		Preload("Subscriptions", "status = ? AND expiry_date > ?", models.SubscriptionActive, now).
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	subscriptionsData := make([]map[string]interface{}, 0, len(user.Subscriptions))
	for i := range user.Subscriptions {
		s := &user.Subscriptions[i]
		var plan models.Plan
		if err := database.DB.First(&plan, "id = ?", s.PlanID).Error; err != nil {
			continue
		}
		subscriptionsData = append(subscriptionsData, subscriptions.SubscriptionData(s, &plan))
	}

	// Find the most recent active subscription for backward compatibility
	var activeSub *models.Subscription
	for i := range user.Subscriptions {
		s := &user.Subscriptions[i]
		if activeSub == nil || s.CreatedAt.After(activeSub.CreatedAt) {
			activeSub = s
		}
	}

	var subData interface{} = nil
	if activeSub != nil {
		var plan models.Plan
		database.DB.First(&plan, "id = ?", activeSub.PlanID)
		subData = subscriptions.SubscriptionData(activeSub, &plan)
	}

	return response.OK(c, fiber.Map{
		"id":                 user.ID,
		"name":               user.Name,
		"email":              user.Email,
		"phone":              user.Phone,
		"status":             string(user.Status),
		"avatar_url":         user.AvatarURL,
		"subscriptions":      subscriptionsData,
		"subscription":       subData,
		"current_app_mode":   user.CurrentAppMode,
		"market_joined_at":   user.MarketJoinedAt,
		"learning_joined_at": user.LearningJoinedAt,
	})
}

// HandleForgotPassword godoc
// @Summary     Request password reset
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  map[string]interface{}  true  "Email address"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Router      /auth/forgot-password [post]
func HandleForgotPassword(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return response.BadRequest(c, "email is required")
	}
	ForgotPassword(body.Email)
	return response.OK(c, fiber.Map{"message": "If your email is registered, you will receive an OTP shortly."})
}

// HandleResetPassword godoc
// @Summary     Reset user password
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Param       body  body  map[string]interface{}  true  "Email, OTP, and new password"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/reset-password [post]
func HandleResetPassword(c *fiber.Ctx) error {
	var body struct {
		Email       string `json:"email"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" || body.OTP == "" || body.NewPassword == "" {
		return response.BadRequest(c, "email, otp and new_password are required")
	}

	if err := ResetPassword(body.Email, body.OTP, body.NewPassword); err != nil {
		if errors.Is(err, ErrOTPInvalid) {
			return response.Unauthorized(c, "invalid or expired OTP")
		}
		if err.Error() == "password too short" {
			return response.BadRequest(c, "password must be at least 6 characters")
		}
		return response.InternalError(c, "failed to reset password")
	}

	return response.OK(c, fiber.Map{"message": "Password reset successfully. Please log in."})
}

// HandleUpdateProfile godoc
// @Summary     Update user profile
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]interface{}  true  "Name and phone number"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/profile [put]
func HandleUpdateProfile(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var body struct {
		Name  *string `json:"name"`
		Phone *string `json:"phone"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	updates := map[string]interface{}{}
	if body.Name != nil {
		if *body.Name == "" {
			return response.BadRequest(c, "name cannot be empty")
		}
		updates["name"] = *body.Name
	}
	if body.Phone != nil {
		updates["phone"] = *body.Phone
	}
	if len(updates) == 0 {
		return response.BadRequest(c, "no fields to update")
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return response.InternalError(c, "failed to update profile")
	}

	var user models.User
	database.DB.Where("id = ?", userID).First(&user)

	return response.OK(c, fiber.Map{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"phone": user.Phone,
	})
}

// HandleSetAppMode godoc
// @Summary     Switch the current user's app mode (post-registration)
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]interface{}  true  "New mode: learning or market"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/auth/me/app-mode [patch]
func HandleSetAppMode(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var body struct {
		Mode string `json:"mode"`
	}
	if err := c.BodyParser(&body); err != nil || body.Mode == "" {
		return response.BadRequest(c, "mode is required")
	}

	user, firstTime, err := SetAppMode(userID, body.Mode, c.IP())
	if err != nil {
		if errors.Is(err, ErrInvalidAppMode) {
			return response.BadRequest(c, "mode must be 'learning' or 'market'")
		}
		return response.InternalError(c, "failed to switch app mode")
	}

	return response.OK(c, fiber.Map{
		"current_app_mode":   user.CurrentAppMode,
		"market_joined_at":   user.MarketJoinedAt,
		"learning_joined_at": user.LearningJoinedAt,
		"first_time":         firstTime,
	})
}

// HandleChangePassword godoc
// @Summary     Change user password
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]interface{}  true  "Current and new password"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/change-password [put]
func HandleChangePassword(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&body); err != nil || body.CurrentPassword == "" || body.NewPassword == "" {
		return response.BadRequest(c, "current_password and new_password are required")
	}
	if len(body.NewPassword) < 6 {
		return response.BadRequest(c, "new password must be at least 6 characters")
	}

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.CurrentPassword)); err != nil {
		return response.BadRequest(c, "incorrect current password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return response.InternalError(c, "failed to update password")
	}

	database.DB.Model(&user).Update("password_hash", string(hash))

	// Revoke every outstanding session — a device holding a stolen refresh
	// token (or the current one, if this change was prompted by suspected
	// compromise) must not survive a password change. Caller will need to
	// log in again, including on this device, matching login's own
	// single-session enforcement.
	database.DB.Where("user_id = ?", userID).Delete(&models.UserRefreshToken{})
	database.DB.Model(&user).Update("current_session_id", uuid.New().String())

	return response.OK(c, fiber.Map{"message": "password updated successfully"})
}

// HandleDeleteAccount deletes the current user's profile and associated auth artifacts.
// @Summary     Delete current user account
// @Tags        User Auth
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/auth/me [delete]
func HandleDeleteAccount(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	now := time.Now()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.UserRefreshToken{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.DeviceToken{}).Error; err != nil {
			return err
		}

		if user.AvatarURL != nil && *user.AvatarURL != "" {
			if key := extractKey(*user.AvatarURL); key != "" {
				_ = storage.Store.Delete(context.Background(), key)
			}
		}

		// Scrub PII before the soft-delete so the row (and anything that joins
		// against it later) no longer carries the user's name/email/phone —
		// the email is replaced with a unique placeholder to satisfy the
		// column's uniqueIndex.
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"name":          "Deleted User",
			"email":         fmt.Sprintf("deleted-%s@deleted.invalid", userID),
			"phone":         "",
			"password_hash": "",
			"avatar_url":    nil,
		}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&user).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return response.InternalError(c, "failed to delete account")
	}

	database.DB.Create(&models.AuditLog{
		EventType:   models.EventUserDeleted,
		ActorUserID: &userID,
		TargetID:    &userID,
		IPAddress:   c.IP(),
	})

	return response.OK(c, fiber.Map{"message": "account deleted"})
}

// resolveAvatarUpload picks extension/MIME from the filename or file content (mobile
// clients often send cache paths without an extension).
func resolveAvatarUpload(filename string, src io.Reader, reportedSize int64) (ext, mimeType string, body io.Reader, size int64, err error) {
	ext = strings.ToLower(filepath.Ext(filename))
	if avatarMIMETypes[ext] {
		return ext, avatarMIMEForExt(ext), src, reportedSize, nil
	}

	head := make([]byte, 512)
	n, readErr := io.ReadFull(src, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return "", "", nil, 0, fmt.Errorf("failed to read uploaded file")
	}
	head = head[:n]
	detected := http.DetectContentType(head)
	switch detected {
	case "image/jpeg":
		ext, mimeType = ".jpg", "image/jpeg"
	case "image/png":
		ext, mimeType = ".png", "image/png"
	case "image/webp":
		ext, mimeType = ".webp", "image/webp"
	default:
		return "", "", nil, 0, fmt.Errorf("unsupported image format (jpg, png, webp only)")
	}
	body = io.MultiReader(bytes.NewReader(head), src)
	size = reportedSize
	if size <= 0 {
		size = int64(len(head))
	}
	return ext, mimeType, body, size, nil
}

func avatarMIMEForExt(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// HandleUploadAvatar saves a new profile picture for the current user.
// @Summary     Upload profile picture
// @Tags        User Auth
// @Accept      multipart/form-data
// @Produce     json
// @Security    UserAuth
// @Param       file  formData  file  true  "Avatar file (jpg, png, webp only)"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/avatar [post]
func HandleUploadAvatar(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "image file is required")
	}
	if file.Size > maxAvatarBytes {
		return response.BadRequest(c, "image must be under 5 MB")
	}

	src, err := file.Open()
	if err != nil {
		return response.InternalError(c, "failed to read uploaded file")
	}
	defer src.Close()

	ext, mimeType, body, size, err := resolveAvatarUpload(file.Filename, src, file.Size)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	fileKey := fmt.Sprintf("avatars/%s%s", uuid.New().String(), ext)

	if err := storage.Store.Upload(context.Background(), fileKey, mimeType, body, size); err != nil {
		slog.Error("[upload] failed to store avatar", "key", fileKey, "error", err)
		return response.InternalError(c, "failed to save avatar")
	}

	avatarURL := storage.Store.PublicURL(fileKey)

	// Read-old-URL-then-update was two unsynchronized DB calls — concurrent
	// requests could race on the stale read and each overwrite the other's
	// URL, orphaning every upload but the last writer. Lock the row for the
	// duration of the swap so concurrent calls serialize instead of racing.
	var oldAvatarURL string
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.AvatarURL != nil {
			oldAvatarURL = *user.AvatarURL
		}
		return tx.Model(&models.User{}).Where("id = ?", userID).Update("avatar_url", avatarURL).Error
	})
	if err != nil {
		slog.Error("[upload] failed to update avatar_url", "error", err)
		return response.InternalError(c, "failed to save avatar")
	}

	if oldAvatarURL != "" {
		if oldKey := extractKey(oldAvatarURL); oldKey != "" {
			_ = storage.Store.Delete(context.Background(), oldKey)
		}
	}

	return response.OK(c, fiber.Map{"avatar_url": avatarURL})
}

// HandleDeleteAvatar removes the current user's profile picture.
func HandleDeleteAvatar(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	var user models.User
	database.DB.Where("id = ?", userID).First(&user)
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		if key := extractKey(*user.AvatarURL); key != "" {
			_ = storage.Store.Delete(context.Background(), key)
		}
	}
	database.DB.Model(&models.User{}).Where("id = ?", userID).Update("avatar_url", nil)
	return response.OK(c, fiber.Map{"message": "avatar removed"})
}

// extractKey returns the storage key from either a full URL or a legacy /uploads/ path.
func extractKey(avatarURL string) string {
	if strings.HasPrefix(avatarURL, "/uploads/") {
		return strings.TrimPrefix(avatarURL, "/uploads/")
	}
	// Full URL — extract key after known CDN or presigned host prefixes
	prefixes := []string{".r2.cloudflarestorage.com/"}
	if base := strings.TrimRight(config.App.R2PublicBase, "/"); base != "" {
		prefixes = append(prefixes, base+"/")
	}
	if base := strings.TrimRight(config.App.ServerBaseURL, "/"); base != "" {
		prefixes = append(prefixes, base+"/uploads/")
	}
	for _, prefix := range prefixes {
		if idx := strings.Index(avatarURL, prefix); idx >= 0 {
			return avatarURL[idx+len(prefix):]
		}
	}
	return ""
}

// HandleRegisterDeviceToken upserts an FCM device token for the current user.
// @Summary     Register device token
// @Tags        User Auth
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]interface{}  true  "Token and platform"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/device-token [post]
func maskToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}

func HandleRegisterDeviceToken(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var body struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return response.BadRequest(c, "token is required")
	}
	platform := body.Platform
	if platform == "" {
		platform = "android"
	}

	slog.Info("device token registration", "user_id", userID, "platform", platform, "token_prefix", maskToken(body.Token))

	token := models.DeviceToken{
		UserID:   userID,
		Token:    body.Token,
		Platform: platform,
	}
	// Upsert by token value — reassign to this user if token already exists
	database.DB.Where(models.DeviceToken{Token: body.Token}).
		Assign(models.DeviceToken{UserID: userID, Platform: platform}).
		FirstOrCreate(&token)

	return response.OK(c, fiber.Map{"message": "device token registered"})
}

func HandleListDeviceTokens(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	var tokens []models.DeviceToken
	database.DB.Where("user_id = ?", userID).Find(&tokens)

	// Mask the token and drop the internal row ID — callers only need to know
	// which platforms/devices are registered, not the raw FCM credential.
	out := make([]fiber.Map, len(tokens))
	for i, t := range tokens {
		out[i] = fiber.Map{
			"platform":     t.Platform,
			"token_prefix": maskToken(t.Token),
			"updated_at":   t.UpdatedAt,
		}
	}
	return response.OK(c, fiber.Map{
		"count":  len(tokens),
		"tokens": out,
	})
}
