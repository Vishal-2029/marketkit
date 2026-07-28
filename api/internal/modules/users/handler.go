package users

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var cryptoRand = rand.Read

// HandleList godoc
// @Summary     List all users
// @Tags        Users
// @Produce     json
// @Security    AdminAuth
// @Param       page  query  int  false  "Page number"
// @Param       limit  query  int  false  "Items per page"
// @Param       search  query  string  false  "Search term"
// @Param       status  query  string  false  "User status"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/users [get]
func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	q := database.DB.Model(&models.User{})
	if s := c.Query("search"); s != "" {
		q = q.Where("name ILIKE ? OR email ILIKE ?", "%"+s+"%", "%"+s+"%")
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if m := c.Query("app_mode"); m != "" {
		q = q.Where("current_app_mode = ?", m)
	}

	var total int64
	q.Count(&total)

	var users []models.User
	if err := q.Preload("Subscriptions.Plan").Offset(offset).Limit(limit).Order("joined_at DESC").Find(&users).Error; err != nil {
		return response.InternalError(c, "failed to fetch users")
	}

	return response.Paginated(c, users, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleCreate godoc
// @Summary     Create a new user
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]interface{}  true  "Name, email, phone, and optional plan ID"
// @Success     201  {object}  models.User
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/users [post]
func HandleCreate(c *fiber.Ctx) error {
	var body struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		Password     string `json:"password"`
		FreePlanID   string `json:"free_plan_id"`
		DurationDays *int   `json:"duration_days"`
	}
	if err := c.BodyParser(&body); err != nil || body.Name == "" || body.Email == "" {
		return response.BadRequest(c, "name and email are required")
	}
	if body.Password != "" && len(body.Password) < 6 {
		return response.BadRequest(c, "password must be at least 6 characters")
	}

	// Check email uniqueness
	var count int64
	database.DB.Model(&models.User{}).Where("LOWER(email) = LOWER(?)", body.Email).Count(&count)
	if count > 0 {
		return response.BadRequest(c, "an account with this email already exists")
	}

	user := models.User{Name: body.Name, Email: body.Email, Phone: body.Phone, Status: models.UserStatusActive}
	if body.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			return response.InternalError(c, "failed to hash password")
		}
		user.PasswordHash = string(hash)
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return response.InternalError(c, "failed to create user")
	}

	if body.FreePlanID != "" {
		var plan models.Plan
		if database.DB.First(&plan, "id = ?", body.FreePlanID).Error == nil {
			duration := plan.DurationDays
			if body.DurationDays != nil && *body.DurationDays > 0 {
				duration = *body.DurationDays
			}
			sub := models.Subscription{
				UserID:      user.ID,
				PlanID:      plan.ID,
				ExpiryDate:  time.Now().AddDate(0, 0, duration),
				ActivatedBy: "free",
			}
			database.DB.Create(&sub)
		}
	}

	return response.Created(c, user)
}

// HandleSendUserOTP sends a verification OTP to an email during admin-initiated user creation.
func HandleSendUserOTP(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return response.BadRequest(c, "email is required")
	}

	// Check email is not already taken
	var count int64
	database.DB.Model(&models.User{}).Where("LOWER(email) = LOWER(?)", body.Email).Count(&count)
	if count > 0 {
		return response.BadRequest(c, "an account with this email already exists")
	}
	var adminCount int64
	database.DB.Model(&models.Admin{}).Where("LOWER(email) = LOWER(?)", body.Email).Count(&adminCount)
	if adminCount > 0 {
		return response.BadRequest(c, "an account with this email already exists")
	}

	otp, err := generateAdminOTP()
	if err != nil {
		return response.InternalError(c, "failed to generate OTP")
	}

	hash, err := bcryptHash(otp)
	if err != nil {
		return response.InternalError(c, "failed to process OTP")
	}

	// Clean up any prior pending OTPs for this email, then store the new one.
	database.DB.Where("email = ?", body.Email).Delete(&models.PendingUserOTP{})

	expiresAt := time.Now().Add(10 * time.Minute)
	database.DB.Create(&models.PendingUserOTP{
		Email:     body.Email,
		CodeHash:  hash,
		ExpiresAt: expiresAt,
	})

	name := body.Name
	if name == "" {
		name = "there"
	}
	if err := email.SendOTPEmail(body.Email, name, otp); err != nil {
		return response.InternalError(c, "failed to send OTP email")
	}

	return response.OK(c, fiber.Map{"message": "OTP sent to " + body.Email})
}

// HandleVerifyUserOTP verifies the OTP sent during admin-initiated user creation.
func HandleVerifyUserOTP(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" || body.OTP == "" {
		return response.BadRequest(c, "email and otp are required")
	}

	var pending models.PendingUserOTP
	if err := database.DB.
		Where("email = ?", body.Email).
		Order("created_at DESC").
		First(&pending).Error; err != nil {
		return response.BadRequest(c, "no pending OTP found for this email")
	}

	if time.Now().After(pending.ExpiresAt) {
		return response.BadRequest(c, "OTP has expired")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(pending.CodeHash), []byte(body.OTP)); err != nil {
		return response.Unauthorized(c, "invalid OTP")
	}

	// Consume the OTP record
	database.DB.Delete(&pending)

	return response.OK(c, fiber.Map{"verified": true})
}

func generateAdminOTP() (string, error) {
	b := make([]byte, 3)
	if _, err := cryptoRand(b); err != nil {
		return "", err
	}
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}

func bcryptHash(s string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return string(hash), err
}

// HandleGet godoc
// @Summary     Get user by ID
// @Tags        Users
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  models.User
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id} [get]
func HandleGet(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := database.DB.Preload("Subscriptions.Plan").First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}
	return response.OK(c, user)
}

// HandleUpdate godoc
// @Summary     Update user details
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Param       body  body  map[string]interface{}  true  "Name and optional status"
// @Success     200  {object}  models.User
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id} [put]
func HandleUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	var body struct {
		Name   *string            `json:"name"`
		Status *models.UserStatus `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	updates := map[string]interface{}{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Status != nil {
		updates["status"] = *body.Status
	}

	if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
		slog.Error("users: failed to update user", "id", user.ID, "error", err)
		return response.InternalError(c, "failed to update user")
	}

	if body.Status != nil && *body.Status == models.UserStatusSuspended {
		go email.SendAccountSuspendedEmail(user.Email, user.Name)
	}

	return response.OK(c, user)
}

// HandleDelete godoc
// @Summary     Delete user
// @Tags        Users
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	now := time.Now()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.UserRefreshToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.DeviceToken{}).Error; err != nil {
			return err
		}

		// Scrub PII before the soft-delete, matching the self-service delete
		// flow (user_auth/handler.go HandleDeleteAccount) — a soft-deleted
		// row otherwise keeps name/email/phone/password_hash at rest
		// indefinitely, retrievable via Unscoped() queries, backups, or
		// direct SQL access.
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"name":          "Deleted User",
			"email":         fmt.Sprintf("deleted-%s@deleted.invalid", id),
			"phone":         "",
			"password_hash": "",
			"avatar_url":    nil,
		}).Error; err != nil {
			return err
		}

		return tx.Delete(&user).Error
	}); err != nil {
		return response.InternalError(c, "failed to delete user")
	}

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventUserDeleted,
		ActorAdminID: &adminID,
		TargetID:     &id,
		IPAddress:    c.IP(),
	})

	return response.OK(c, fiber.Map{"message": "user deleted"})
}

// HandleForceLogout godoc
// @Summary     Force user logout
// @Tags        Users
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id}/force-logout [post]
func HandleForceLogout(c *fiber.Ctx) error {
	id := c.Params("id")
	now := time.Now()
	database.DB.Model(&models.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now)
	database.DB.Where("user_id = ?", id).Delete(&models.UserRefreshToken{})

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType: models.EventForceLogout, ActorAdminID: &adminID, TargetID: &id,
		IPAddress: c.IP(),
	})

	return response.OK(c, fiber.Map{"message": "all sessions revoked"})
}

// HandleSuspend godoc
// @Summary     Suspend user
// @Tags        Users
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id}/suspend [post]
func HandleSuspend(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	now := time.Now()
	newSessionID := uuid.New().String()

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"status":             models.UserStatusSuspended,
			"current_session_id": newSessionID,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.UserRefreshToken{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error
	}); err != nil {
		return response.InternalError(c, "failed to suspend user")
	}

	go email.SendAccountSuspendedEmail(user.Email, user.Name)

	return response.OK(c, fiber.Map{"message": "user suspended"})
}

// HandleAssignFreePlan godoc
// @Summary     Assign a free plan to a user (no payment required)
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Param       body  body  map[string]interface{}  true  "Plan ID and optional duration_days override"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id}/assign-free-plan [post]
func HandleAssignFreePlan(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		PlanID       string `json:"plan_id"`
		DurationDays *int   `json:"duration_days"`
	}
	if err := c.BodyParser(&body); err != nil || body.PlanID == "" {
		return response.BadRequest(c, "plan_id is required")
	}

	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", body.PlanID).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	duration := plan.DurationDays
	if body.DurationDays != nil && *body.DurationDays > 0 {
		duration = *body.DurationDays
	}

	// Cancel existing active subscriptions
	database.DB.Model(&models.Subscription{}).
		Where("user_id = ? AND status = ?", id, models.SubscriptionActive).
		Update("status", models.SubscriptionCancelled)

	sub := models.Subscription{
		UserID:      id,
		PlanID:      plan.ID,
		ExpiryDate:  time.Now().AddDate(0, 0, duration),
		ActivatedBy: "free",
	}
	if err := database.DB.Create(&sub).Error; err != nil {
		return response.InternalError(c, "failed to assign free plan")
	}

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventManualActivation,
		ActorAdminID: &adminID,
		TargetID:     &id,
		IPAddress:    c.IP(),
		Details:      models.JSONMap{"plan_id": body.PlanID, "plan_name": plan.Name, "duration_days": duration, "type": "free"},
	})

	go func() {
		subData := email.SubscriptionEmailData{
			Name:      user.Name,
			PlanName:  plan.Name,
			ExpiresAt: email.FormatDate(sub.ExpiryDate),
			Features:  plan.FeatureList(),
		}
		email.SendNewSubscriptionEmail(user.Email, subData)
	}()

	return response.OK(c, sub)
}

// HandleCancelFreePlan godoc
// @Summary     Cancel a user's active free plan
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id}/cancel-free-plan [post]
func HandleCancelFreePlan(c *fiber.Ctx) error {
	id := c.Params("id")

	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	// Only admin-granted free plans are cancellable here; paid subscriptions are untouched.
	var sub models.Subscription
	if err := database.DB.Preload("Plan").
		Where("user_id = ? AND status = ? AND activated_by = ?", id, models.SubscriptionActive, "free").
		First(&sub).Error; err != nil {
		return response.BadRequest(c, "user has no active free plan")
	}

	database.DB.Model(&sub).Update("status", models.SubscriptionCancelled)

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventPlanChanged,
		ActorAdminID: &adminID,
		TargetID:     &id,
		IPAddress:    c.IP(),
		Details:      models.JSONMap{"action": "cancel_free_plan", "plan_id": sub.PlanID, "plan_name": sub.Plan.Name},
	})

	go email.SendPlanExpiredEmail(user.Email, email.ExpiryEmailData{
		Name:      user.Name,
		PlanName:  sub.Plan.Name,
		ExpiresAt: email.FormatDate(time.Now()),
		DaysLeft:  0,
	})

	return response.OK(c, sub)
}

// HandleChangePlan godoc
// @Summary     Change user's subscription plan
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "User ID"
// @Param       body  body  map[string]interface{}  true  "New plan ID"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/users/{id}/plan [post]
func HandleChangePlan(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.PlanID == "" {
		return response.BadRequest(c, "plan_id is required")
	}

	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", body.PlanID).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	// Capture the current plan price before cancelling, to determine upgrade vs downgrade.
	var currentSub models.Subscription
	var currentPlan models.Plan
	if database.DB.Preload("Plan").
		Where("user_id = ? AND status = ?", id, models.SubscriptionActive).
		First(&currentSub).Error == nil {
		currentPlan = currentSub.Plan
	}
	isUpgrade := plan.PriceMinor > currentPlan.PriceMinor

	var sub models.Subscription
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Subscription{}).
			Where("user_id = ? AND status = ?", id, models.SubscriptionActive).
			Update("status", models.SubscriptionCancelled).Error; err != nil {
			return err
		}
		sub = models.Subscription{
			UserID: id, PlanID: plan.ID,
			ExpiryDate:  time.Now().AddDate(0, 0, plan.DurationDays),
			ActivatedBy: "admin",
		}
		return tx.Create(&sub).Error
	}); err != nil {
		return response.InternalError(c, "failed to change plan")
	}

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType: models.EventPlanChanged, ActorAdminID: &adminID, TargetID: &id,
		Details: models.JSONMap{"plan_id": body.PlanID},
	})

	var user models.User
	database.DB.First(&user, "id = ?", id)

	go func() {
		subData := email.SubscriptionEmailData{
			Name:      user.Name,
			PlanName:  plan.Name,
			ExpiresAt: email.FormatDate(sub.ExpiryDate),
			Features:  plan.FeatureList(),
		}
		if isUpgrade {
			email.SendPlanUpgradeEmail(user.Email, subData)
		} else {
			email.SendNewSubscriptionEmail(user.Email, subData)
		}

		if adminEmail := config.App.AdminEmail; adminEmail != "" {
			email.SendAdminSubAlert(adminEmail, email.AdminSubAlertData{
				UserName:  user.Name,
				UserEmail: user.Email,
				PlanName:  plan.Name,
				Amount:    email.FormatAmount(plan.PriceMinor),
				Provider:  "MANUAL",
				PaidAt:    email.FormatDate(time.Now()),
				IsUpgrade: isUpgrade,
			})
		}
	}()

	return response.OK(c, sub)
}
