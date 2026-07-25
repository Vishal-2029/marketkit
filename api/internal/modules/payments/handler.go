package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/market"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/internal/subscriptions"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.Payment{}).Preload("User").Preload("Plan")
	if s := c.Query("search"); s != "" {
		q = q.Joins("JOIN users ON users.id = payments.user_id").
			Where("users.name ILIKE ? OR users.email ILIKE ?", "%"+s+"%", "%"+s+"%")
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("payments.status = ?", st)
	}
	if gw := c.Query("gateway"); gw != "" {
		q = q.Where("gateway = ?", gw)
	}

	var total int64
	q.Count(&total)

	var payments []models.Payment
	if err := q.Offset((page - 1) * limit).Limit(limit).Order("created_at DESC").Find(&payments).Error; err != nil {
		return response.InternalError(c, "failed to fetch payments")
	}
	for i := range payments {
		payments[i].User = trimUserForResponse(payments[i].User)
	}

	return response.Paginated(c, payments, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

func HandleGet(c *fiber.Ctx) error {
	var p models.Payment
	if err := database.DB.Preload("User").Preload("Plan").First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "payment not found")
	}
	p.User = trimUserForResponse(p.User)
	return response.OK(c, p)
}

// trimUserForResponse strips a preloaded User down to the fields admin UIs
// actually display (name/email) so responses don't unnecessarily carry
// phone, wallet balance, avatar, and status for every user referenced.
func trimUserForResponse(u models.User) models.User {
	return models.User{ID: u.ID, Name: u.Name, Email: u.Email}
}

func HandleManual(c *fiber.Ctx) error {
	var body struct {
		UserID string `json:"user_id"`
		PlanID string `json:"plan_id"`
		Notes  string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil || body.UserID == "" || body.PlanID == "" {
		return response.BadRequest(c, "user_id and plan_id are required")
	}

	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", body.PlanID).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", body.UserID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)

	payment, err := createManualPayment(&plan, body.UserID, body.Notes, expiresAt)
	if err != nil {
		return response.InternalError(c, "failed to create manual payment")
	}

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType: models.EventManualActivation, ActorAdminID: &adminID,
		TargetID: &body.UserID,
		Details:  models.JSONMap{"plan_id": body.PlanID, "amount_paise": plan.PriceInPaise},
	})

	go sendSubscriptionEmails(&user, &plan, &payment, expiresAt, false)
	go fcm.SendToUser(body.UserID, "Subscription Activated",
		"Your "+plan.Name+" plan is now active. Enjoy learning!")

	return response.Created(c, payment)
}

// createManualPayment is the money-moving core of HandleManual, split out so
// it can be exercised directly in tests without also triggering the
// handler's fire-and-forget notification/email goroutines.
func createManualPayment(plan *models.Plan, userID, notes string, expiresAt time.Time) (models.Payment, error) {
	now := time.Now()
	payment := models.Payment{
		UserID: userID, PlanID: plan.ID,
		AmountInPaise: plan.PriceInPaise,
		Gateway:       models.GatewayManual, Status: models.PaymentSuccess,
		Notes: notes, PaidAt: &now,
	}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		if _, err := platform_wallet.Apply(tx, models.PlatformSourceLearningPlan, plan.PriceInPaise,
			&payment.ID, models.JSONMap{"plan_id": plan.ID, "plan_name": plan.Name, "paid_via": "MANUAL"}); err != nil {
			return err
		}
		return subscriptions.ActivateOrExtend(tx, userID, plan.ID, expiresAt, "manual")
	})
	return payment, err
}

func HandleRazorpayWebhook(c *fiber.Ctx) error {
	body := c.Body()
	signature := c.Get("X-Razorpay-Signature")
	secret := config.App.RazorpayWebhookSecret

	// Fail closed: an empty secret makes the HMAC trivially forgeable (anyone
	// can compute HMAC-SHA256 with a known/empty key), so an unconfigured
	// secret must reject every request rather than "verify" against one.
	if secret == "" {
		slog.Error("razorpay webhook: RAZORPAY_WEBHOOK_SECRET is not configured — rejecting request")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webhook not configured"})
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signature"})
	}

	var event map[string]interface{}
	if err := c.BodyParser(&event); err != nil {
		return response.BadRequest(c, "invalid payload")
	}

	eventType, _ := event["event"].(string)
	if eventType == "payment.captured" {
		payload, _ := event["payload"].(map[string]interface{})
		paymentEntity, _ := payload["payment"].(map[string]interface{})
		entity, _ := paymentEntity["entity"].(map[string]interface{})

		razorpayPaymentID := fmt.Sprintf("%v", entity["id"])
		orderID := fmt.Sprintf("%v", entity["order_id"])

		// The flip and the platform-wallet credit run in one transaction so a
		// rollback (e.g. the credit failing) undoes the status flip too —
		// otherwise a payment could end up SUCCESS with no matching credit.
		var flipped bool
		txErr := database.DB.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&models.Payment{}).
				Where("razorpay_order_id = ? AND status != ?", orderID, models.PaymentSuccess).
				Updates(map[string]interface{}{
					"status":              models.PaymentSuccess,
					"razorpay_payment_id": razorpayPaymentID,
					"paid_at":             time.Now(),
					"gateway_response":    models.JSONMap(entity),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			var flippedP models.Payment
			if err := tx.Where("razorpay_order_id = ?", orderID).First(&flippedP).Error; err != nil {
				return err
			}
			if _, err := platform_wallet.Apply(tx, models.PlatformSourceLearningPlan, flippedP.AmountInPaise,
				&flippedP.ID, models.JSONMap{"plan_id": flippedP.PlanID, "paid_via": "RAZORPAY"}); err != nil {
				return err
			}
			flipped = true
			return nil
		})
		if txErr != nil {
			slog.Error("webhook: failed to capture payment", "order_id", orderID, "error", txErr)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "payment capture failed"})
		}
		if !flipped {
			// Not a subscription payment (or already captured). Razorpay order
			// IDs are account-unique, so try the product marketplace, then
			// wallet topups. Either way return 200 — Razorpay retries on
			// anything else.
			if !market.CaptureRazorpayOrder(orderID, razorpayPaymentID, models.JSONMap(entity)) {
				if !market.CaptureMarketPlanOrder(orderID, razorpayPaymentID) {
					wallet.CaptureRazorpayOrder(orderID, razorpayPaymentID, models.JSONMap(entity))
				}
			}
			return c.SendStatus(fiber.StatusOK)
		}

		var p models.Payment
		if err := database.DB.Preload("User").Preload("Plan").
			Where("razorpay_order_id = ?", orderID).First(&p).Error; err == nil {

			expiresAt := time.Now().AddDate(0, 0, p.Plan.DurationDays)
			if err := database.DB.Transaction(func(tx *gorm.DB) error {
				return subscriptions.ActivateOrExtend(tx, p.UserID, p.PlanID, expiresAt, "razorpay")
			}); err != nil {
				slog.Error("webhook: failed to activate subscription", "user_id", p.UserID, "error", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "subscription activation failed"})
			}

			go func() {
				sendSubscriptionEmails(&p.User, &p.Plan, &p, expiresAt, false)
				if err := fcm.SendToUser(p.UserID, "Subscription Activated",
					"Your "+p.Plan.Name+" plan is now active. Enjoy learning!"); err != nil {
					slog.Error("webhook: FCM notification failed", "user_id", p.UserID, "error", err)
				}
			}()
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func HandleActivate(c *fiber.Ctx) error {
	var p models.Payment
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "payment not found")
	}
	if p.Status == models.PaymentSuccess {
		return response.OK(c, fiber.Map{"message": "payment already activated"})
	}

	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", p.PlanID).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", p.UserID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, plan.DurationDays)

	// The status != SUCCESS guard (matching the webhook/verify handlers'
	// pattern) makes the platform-wallet credit below exactly-once even if
	// this endpoint is ever called twice on the same payment.
	activated := false
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Payment{}).
			Where("id = ? AND status != ?", p.ID, models.PaymentSuccess).
			Updates(map[string]interface{}{
				"status":  models.PaymentSuccess,
				"paid_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		activated = true
		if _, err := platform_wallet.Apply(tx, models.PlatformSourceLearningPlan, p.AmountInPaise,
			&p.ID, models.JSONMap{"plan_id": p.PlanID, "plan_name": plan.Name, "paid_via": "ADMIN_MANUAL"}); err != nil {
			return err
		}
		return subscriptions.ActivateOrExtend(tx, p.UserID, p.PlanID, expiresAt, "admin-manual")
	}); err != nil {
		return response.InternalError(c, "failed to activate payment")
	}
	if !activated {
		return response.OK(c, fiber.Map{"message": "payment already activated"})
	}

	p.PaidAt = &now
	go sendSubscriptionEmails(&user, &plan, &p, expiresAt, false)

	return response.OK(c, fiber.Map{"message": "payment activated and subscription created"})
}

func sendSubscriptionEmails(user *models.User, plan *models.Plan, payment *models.Payment, expiresAt time.Time, isUpgrade bool) {
	paidAt := time.Now()
	if payment.PaidAt != nil {
		paidAt = *payment.PaidAt
	}

	txnID := payment.ID
	if payment.RazorpayPaymentID != nil {
		txnID = *payment.RazorpayPaymentID
	}

	gateway := string(payment.Gateway)

	email.SendPaymentReceiptEmail(user.Email, email.PaymentReceiptData{
		Name:          user.Name,
		PlanName:      plan.Name,
		Amount:        email.FormatAmount(plan.PriceInPaise),
		TransactionID: txnID,
		Gateway:       gateway,
		PaidAt:        email.FormatDate(paidAt),
		ExpiresAt:     email.FormatDate(expiresAt),
	})

	subData := email.SubscriptionEmailData{
		Name:       user.Name,
		PlanName:   plan.Name,
		ExpiresAt:  email.FormatDate(expiresAt),
		HasWillcom: plan.HasWillcom,
		HasE4:      plan.HasE4,
		HasMecad:   plan.HasMecad,
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
			Amount:    email.FormatAmount(plan.PriceInPaise),
			Gateway:   gateway,
			PaidAt:    email.FormatDate(paidAt),
			IsUpgrade: isUpgrade,
		})
	}
}
