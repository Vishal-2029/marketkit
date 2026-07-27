package user_payments

import (
	"context"
	"time"

	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/payments"
	"github.com/marketkit/api/internal/payments/provider"
	"github.com/marketkit/api/internal/subscriptions"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandleCreateOrder godoc
// @Summary     Create a Razorpay payment order
// @Tags        User Payments
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "plan_id"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/payments/order [post]
func HandleCreateOrder(c *fiber.Ctx) error {
	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.PlanID == "" {
		return response.BadRequest(c, "plan_id is required")
	}

	var plan models.Plan
	if err := database.DB.First(&plan, "id = ? AND is_active = true", body.PlanID).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	userID, _ := c.Locals("userID").(string)

	if config.App.RazorpayKeyID == "" || config.App.RazorpayKeySecret == "" ||
		config.App.RazorpayKeyID == "rzp_test_xxxx" || config.App.RazorpayKeySecret == "xxxx" {
		return response.InternalError(c, "razorpay is not configured on the server")
	}

	// Create Razorpay order via REST API (no SDK dependency)
	rzpOrderID, err := createRazorpayOrder(plan.PriceInPaise, userID, plan.ID)
	if err != nil {
		slog.Error("razorpay order creation failed", "error", err, "user_id", userID, "plan_id", plan.ID)
		return response.InternalError(c, "failed to create payment order")
	}

	// Store pending payment record
	payment := models.Payment{
		UserID:          userID,
		PlanID:          plan.ID,
		AmountInPaise:   plan.PriceInPaise,
		Provider:        models.ProviderRazorpay,
		Status:          models.PaymentPending,
		ProviderOrderID: &rzpOrderID,
	}
	database.DB.Create(&payment)

	return response.OK(c, fiber.Map{
		"order_id": rzpOrderID,
		"amount":   plan.PriceInPaise,
		"currency": "INR",
		"key_id":   config.App.RazorpayKeyID,
	})
}

// HandleVerifyPayment godoc
// @Summary     Verify a Razorpay payment signature and activate subscription
// @Tags        User Payments
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "provider_order_id, provider_payment_id, provider_signature"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/payments/verify [post]
func HandleVerifyPayment(c *fiber.Ctx) error {
	var body struct {
		ProviderOrderID   string `json:"provider_order_id"`
		ProviderPaymentID string `json:"provider_payment_id"`
		ProviderSignature string `json:"provider_signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.ProviderOrderID == "" || body.ProviderPaymentID == "" || body.ProviderSignature == "" {
		return response.BadRequest(c, "provider_order_id, provider_payment_id, and provider_signature are required")
	}

	userID, _ := c.Locals("userID").(string)

	// Signature scheme is gateway-specific; the provider owns the check.
	if err := payments.VerifyCheckout(provider.CheckoutResult{
		OrderID:   body.ProviderOrderID,
		PaymentID: body.ProviderPaymentID,
		Signature: body.ProviderSignature,
	}); err != nil {
		slog.Warn("checkout verification failed", "user_id", c.Locals("userID"), "error", err)
		return response.BadRequest(c, "invalid payment signature")
	}

	// Load pending payment tied to this user + order.
	var p models.Payment
	if err := database.DB.Preload("User").Preload("Plan").
		Where("user_id = ? AND provider_order_id = ?", userID, body.ProviderOrderID).
		First(&p).Error; err != nil {
		return response.NotFound(c, "payment not found")
	}

	// Idempotent success.
	if p.Status == models.PaymentSuccess {
		return response.OK(c, fiber.Map{"message": "payment already verified"})
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, p.Plan.DurationDays)

	captured, err := captureVerifiedPayment(&p, body.ProviderPaymentID, now, expiresAt)
	if err != nil {
		return response.InternalErrorWithLog(c, "user_payments: verify capture", err)
	}
	if !captured {
		return response.OK(c, fiber.Map{"message": "payment already verified"})
	}

	// Fire-and-forget notification/email.
	go func() {
		email.SendPaymentReceiptEmail(p.User.Email, email.PaymentReceiptData{
			Name:          p.User.Name,
			PlanName:      p.Plan.Name,
			Amount:        email.FormatAmount(p.Plan.PriceInPaise),
			TransactionID: body.ProviderPaymentID,
			Provider:      string(p.Provider),
			PaidAt:        email.FormatDate(now),
			ExpiresAt:     email.FormatDate(expiresAt),
		})
		subData := email.SubscriptionEmailData{
			Name:      p.User.Name,
			PlanName:  p.Plan.Name,
			ExpiresAt: email.FormatDate(expiresAt),
			Features:  p.Plan.FeatureList(),
		}
		email.SendNewSubscriptionEmail(p.User.Email, subData)
		if adminEmail := config.App.AdminEmail; adminEmail != "" {
			email.SendAdminSubAlert(adminEmail, email.AdminSubAlertData{
				UserName:  p.User.Name,
				UserEmail: p.User.Email,
				PlanName:  p.Plan.Name,
				Amount:    email.FormatAmount(p.Plan.PriceInPaise),
				Provider:  string(p.Provider),
				PaidAt:    email.FormatDate(now),
				IsUpgrade: false,
			})
		}
		_ = fcm.SendToUser(p.UserID, "Subscription Activated",
			"Your "+p.Plan.Name+" plan is now active. Enjoy learning!")
	}()

	return response.OK(c, fiber.Map{"message": "payment verified and subscription activated"})
}

// captureVerifiedPayment is the money-moving core of HandleVerifyPayment,
// split out so it can be exercised directly in tests without also
// triggering the handler's fire-and-forget notification/email goroutine.
// Flips p to SUCCESS, credits the platform wallet, and activates the
// subscription, all in one transaction. The AND status != SUCCESS guard
// makes this atomic: if two requests for the same payment race past the
// plain SELECT check in the caller, only one can actually flip the row, so
// captured is false for the other and nothing here runs twice.
func captureVerifiedPayment(p *models.Payment, razorpayPaymentID string, paidAt, expiresAt time.Time) (captured bool, err error) {
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Payment{}).
			Where("id = ? AND status != ?", p.ID, models.PaymentSuccess).
			Updates(map[string]interface{}{
				"status":              models.PaymentSuccess,
				"provider_payment_id": razorpayPaymentID,
				"paid_at":             paidAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		if _, err := platform_wallet.Apply(tx, models.PlatformSourceLearningPlan, p.AmountInPaise,
			&p.ID, models.JSONMap{"plan_id": p.PlanID, "plan_name": p.Plan.Name, "paid_via": "RAZORPAY"}); err != nil {
			return err
		}

		if err := subscriptions.ActivateOrExtend(tx, p.UserID, p.PlanID, expiresAt, "razorpay-verify"); err != nil {
			return err
		}

		captured = true
		return nil
	})
	return captured, err
}

func createRazorpayOrder(amountInPaise int64, userID, planID string) (string, error) {
	order, err := payments.CreateOrder(context.Background(), amountInPaise,
		payments.Receipt("u", userID, planID),
		map[string]string{"user_id": userID, "plan_id": planID})
	if err != nil {
		return "", err
	}
	return order.ID, nil
}
