package auth

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

const refreshCookieName = "refresh_token"

type registerRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Password  string `json:"password"`
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
// @Summary     Register a new admin account
// @Tags        Admin Auth
// @Accept      json
// @Produce     json
// @Param       body  body  auth.registerRequest  true  "Registration payload"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Router      /auth/register [post]
func HandleRegister(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if req.FirstName == "" || req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "first name, email and password are required")
	}
	if len(req.Password) < 6 {
		return response.BadRequest(c, "password must be at least 6 characters")
	}

	return response.Forbidden(c, "registration is disabled")
}

// HandleSendOTP godoc
// @Summary     Send OTP to admin email
// @Tags        Admin Auth
// @Accept      json
// @Produce     json
// @Param       body  body  auth.sendOTPRequest  true  "Email and password"
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
		slog.Error("admin/send-otp: SMTP error", "email", mask.Email(req.Email), "error", err)
		return response.InternalError(c, "Failed to send OTP. Please try again.")
	}
	return response.OK(c, fiber.Map{"message": "OTP sent to your email."})
}

// HandleVerifyOTP godoc
// @Summary     Verify OTP and login as admin
// @Tags        Admin Auth
// @Accept      json
// @Produce     json
// @Param       body  body  auth.verifyOTPRequest  true  "Email and OTP"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /auth/verify-otp [post]
func HandleVerifyOTP(c *fiber.Ctx) error {
	var req verifyOTPRequest
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.OTP == "" {
		return response.BadRequest(c, "email and otp are required")
	}

	accessToken, refreshToken, admin, err := VerifyOTP(req.Email, req.OTP)
	if err != nil {
		return response.Unauthorized(c, "invalid or expired OTP")
	}

	adminID := admin.ID
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventLogin,
		ActorAdminID: &adminID,
		IPAddress:    c.IP(),
		Device:       c.Get("User-Agent"),
		Details:      models.JSONMap{"email": admin.Email},
	})

	sameSite := "Lax"
	if config.App.SecureCookie {
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: sameSite,
		MaxAge:   7 * 24 * 60 * 60,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})

	return response.OK(c, fiber.Map{
		"token": accessToken,
		"admin": fiber.Map{
			"id":         admin.ID,
			"email":      admin.Email,
			"first_name": admin.FirstName,
			"last_name":  admin.LastName,
			"phone":      admin.Phone,
			"is_super":   admin.IsSuper,
			"avatar_url": admin.AvatarURL,
		},
	})
}

// HandleRefresh godoc
// @Summary     Refresh admin access token
// @Tags        Admin Auth
// @Produce     json
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /auth/refresh [post]
func HandleRefresh(c *fiber.Ctx) error {
	raw := c.Cookies(refreshCookieName)
	if raw == "" {
		return response.Unauthorized(c, "no refresh token")
	}

	accessToken, admin, err := RefreshAccess(raw)
	if err != nil {
		// Clear the invalid cookie
		clearSameSite := "Lax"
		if config.App.SecureCookie {
			clearSameSite = "None"
		}
		c.Cookie(&fiber.Cookie{
			Name:     refreshCookieName,
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
		"admin": fiber.Map{
			"id":         admin.ID,
			"email":      admin.Email,
			"first_name": admin.FirstName,
			"last_name":  admin.LastName,
			"phone":      admin.Phone,
			"is_super":   admin.IsSuper,
			"avatar_url": admin.AvatarURL,
		},
	})
}

// HandleLogout godoc
// @Summary     Logout admin
// @Tags        Admin Auth
// @Produce     json
// @Success     200  {object}  map[string]string
// @Router      /auth/logout [post]
func HandleLogout(c *fiber.Ctx) error {
	raw := c.Cookies(refreshCookieName)
	if raw != "" {
		RevokeRefreshToken(raw)
	}

	clearSameSite := "Lax"
	if config.App.SecureCookie {
		clearSameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   config.App.SecureCookie,
		SameSite: clearSameSite,
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
	})

	adminID, _ := c.Locals("adminID").(string)
	if adminID != "" {
		database.DB.Create(&models.AuditLog{
			EventType:    models.EventLogout,
			ActorAdminID: &adminID,
			IPAddress:    c.IP(),
		})
	}

	return response.OK(c, fiber.Map{"message": "logged out"})
}

// HandleMe godoc
// @Summary     Get current admin profile
// @Tags        Admin Auth
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /auth/me [get]
func HandleMe(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)

	var admin models.Admin
	if err := database.DB.Where("id = ?", adminID).First(&admin).Error; err != nil {
		return response.NotFound(c, "admin not found")
	}

	return response.OK(c, fiber.Map{
		"id":         admin.ID,
		"email":      admin.Email,
		"first_name": admin.FirstName,
		"last_name":  admin.LastName,
		"phone":      admin.Phone,
		"is_super":   admin.IsSuper,
		"avatar_url": admin.AvatarURL,
		"created_at": admin.CreatedAt,
	})
}

// HandleUpdateProfile godoc
// @Summary     Update current admin profile
// @Tags        Admin Auth
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]interface{}  true  "Profile fields"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/me [patch]
func HandleUpdateProfile(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)

	var body struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		Phone     *string `json:"phone"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	updates := map[string]interface{}{}
	if body.FirstName != nil {
		if *body.FirstName == "" {
			return response.BadRequest(c, "first name cannot be empty")
		}
		updates["first_name"] = *body.FirstName
	}
	if body.LastName != nil {
		updates["last_name"] = *body.LastName
	}
	if body.Phone != nil {
		updates["phone"] = *body.Phone
	}
	if len(updates) == 0 {
		return response.BadRequest(c, "no fields to update")
	}

	if err := database.DB.Model(&models.Admin{}).Where("id = ?", adminID).Updates(updates).Error; err != nil {
		return response.InternalError(c, "failed to update profile")
	}

	var admin models.Admin
	if err := database.DB.Where("id = ?", adminID).First(&admin).Error; err != nil {
		return response.NotFound(c, "admin not found")
	}

	return response.OK(c, fiber.Map{
		"id":         admin.ID,
		"email":      admin.Email,
		"first_name": admin.FirstName,
		"last_name":  admin.LastName,
		"phone":      admin.Phone,
		"is_super":   admin.IsSuper,
		"avatar_url": admin.AvatarURL,
		"created_at": admin.CreatedAt,
	})
}

// HandleChangePassword godoc
// @Summary     Change current admin password
// @Tags        Admin Auth
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]interface{}  true  "Current and new password"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/change-password [post]
func HandleChangePassword(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)

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

	var admin models.Admin
	if err := database.DB.Where("id = ?", adminID).First(&admin).Error; err != nil {
		return response.NotFound(c, "admin not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(body.CurrentPassword)); err != nil {
		return response.BadRequest(c, "incorrect current password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return response.InternalError(c, "failed to update password")
	}

	if err := database.DB.Model(&admin).Update("password_hash", string(hash)).Error; err != nil {
		return response.InternalError(c, "failed to update password")
	}

	// Revoke every outstanding refresh token so a device holding a stolen
	// one (or this admin's own other sessions) can't keep minting fresh
	// access tokens after the password that was supposed to lock it out changes.
	database.DB.Where("admin_id = ?", admin.ID).Delete(&models.RefreshToken{})

	return response.OK(c, fiber.Map{"message": "password updated successfully"})
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

// HandleUploadAvatar godoc
// @Summary     Upload admin profile picture
// @Tags        Admin Auth
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       file  formData  file  true  "Avatar file (jpg, png, webp only)"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /auth/avatar [post]
func HandleUploadAvatar(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)

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
		slog.Error("[upload] failed to store admin avatar", "key", fileKey, "error", err)
		return response.InternalError(c, "failed to save avatar")
	}

	avatarURL := storage.Store.PublicURL(fileKey)

	// Read-old-URL-then-update was two unsynchronized DB calls — concurrent
	// requests could race on the stale read and each overwrite the other's
	// URL, orphaning every upload but the last writer. Lock the row for the
	// duration of the swap so concurrent calls serialize instead of racing.
	var oldAvatarURL string
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var admin models.Admin
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", adminID).First(&admin).Error; err != nil {
			return err
		}
		if admin.AvatarURL != nil {
			oldAvatarURL = *admin.AvatarURL
		}
		return tx.Model(&models.Admin{}).Where("id = ?", adminID).Update("avatar_url", avatarURL).Error
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

// HandleDeleteAvatar removes the current admin profile picture.
func HandleDeleteAvatar(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)
	var admin models.Admin
	database.DB.Where("id = ?", adminID).First(&admin)
	if admin.AvatarURL != nil && *admin.AvatarURL != "" {
		if key := extractKey(*admin.AvatarURL); key != "" {
			_ = storage.Store.Delete(context.Background(), key)
		}
	}
	database.DB.Model(&models.Admin{}).Where("id = ?", adminID).Update("avatar_url", nil)
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
