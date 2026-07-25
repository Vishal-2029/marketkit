package market

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

type marketPlanWithStats struct {
	models.MarketPlan
	Subscribers int64 `json:"subscribers"`
}

// sellerFeePercent returns the platform fee percent for a seller, applying
// FeeDiscountPct from their ACTIVE market plan subscription if any.
// Placeholder perk: FeeDiscountPct (0-100) knocks off market_fee_percent.
func sellerFeePercent(sellerID string) int64 {
	base := wallet.FeePercent()
	discount := activeSellerFeeDiscount(sellerID)
	if discount <= 0 {
		return base
	}
	if discount > 100 {
		discount = 100
	}
	// Knocks off: effective = base * (100 - discount) / 100
	return base * int64(100-discount) / 100
}

// activeSellerFeeDiscount returns FeeDiscountPct for the seller's ACTIVE
// market plan subscription, or 0 if none.
func activeSellerFeeDiscount(sellerID string) int {
	type row struct {
		FeeDiscountPct int
	}
	var r row
	err := database.DB.Table("market_plan_subscriptions").
		Select("market_plans.fee_discount_pct").
		Joins("JOIN market_plans ON market_plans.id = market_plan_subscriptions.plan_id").
		Where("market_plan_subscriptions.user_id = ? AND market_plan_subscriptions.status = ? AND market_plan_subscriptions.expiry_date > ?",
			sellerID, models.SubscriptionActive, time.Now()).
		Order("market_plan_subscriptions.expiry_date DESC").
		Limit(1).
		Scan(&r).Error
	if err != nil {
		return 0
	}
	return r.FeeDiscountPct
}

// activeFeaturedSellerIDs returns user IDs among the given set that currently
// have an ACTIVE market plan with FeaturedSeller=true.
func activeFeaturedSellerIDs(userIDs []string) map[string]bool {
	out := map[string]bool{}
	if len(userIDs) == 0 {
		return out
	}
	var ids []string
	database.DB.Table("market_plan_subscriptions").
		Select("DISTINCT market_plan_subscriptions.user_id").
		Joins("JOIN market_plans ON market_plans.id = market_plan_subscriptions.plan_id").
		Where("market_plan_subscriptions.user_id IN ? AND market_plan_subscriptions.status = ? AND market_plan_subscriptions.expiry_date > ? AND market_plans.featured_seller = ?",
			userIDs, models.SubscriptionActive, time.Now(), true).
		Pluck("market_plan_subscriptions.user_id", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// HandleGetSellerFee returns the platform fee percent for the current user
// as a seller, applying FeeDiscountPct from their ACTIVE market plan if any.
func HandleGetSellerFee(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	return response.OK(c, fiber.Map{"fee_percent": sellerFeePercent(userID)})
}

// HandleListMarketPlans godoc
// @Summary     List active Product Market plans
// @Tags        User Market Plans
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.MarketPlan
// @Router      /user/market/plans [get]
func HandleListMarketPlans(c *fiber.Ctx) error {
	var plans []models.MarketPlan
	if err := database.DB.Where("is_active = true").Order("price_in_paise ASC").Find(&plans).Error; err != nil {
		return response.InternalError(c, "failed to fetch market plans")
	}
	if plans == nil {
		plans = []models.MarketPlan{}
	}
	return response.OK(c, plans)
}

// HandleMyMarketPlan godoc
// @Summary     Current user's active Product Market plan subscription
// @Tags        User Market Plans
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  models.MarketPlanSubscription
// @Router      /user/market/plans/my [get]
func HandleMyMarketPlan(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var sub models.MarketPlanSubscription
	err := database.DB.Preload("Plan").
		Where("user_id = ? AND status = ? AND expiry_date > ?", userID, models.SubscriptionActive, time.Now()).
		Order("expiry_date DESC").
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.OK(c, nil)
		}
		return response.InternalError(c, "failed to fetch subscription")
	}
	return response.OK(c, sub)
}

// HandleCancelMyMarketPlan godoc
// @Summary     Cancel the current user's active Product Market plan (no refund)
// @Tags        User Market Plans
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Router      /user/market/plans/my [delete]
func HandleCancelMyMarketPlan(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var sub models.MarketPlanSubscription
	err := database.DB.
		Where("user_id = ? AND status = ? AND expiry_date > ?", userID, models.SubscriptionActive, time.Now()).
		Order("expiry_date DESC").
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.BadRequest(c, "no active market plan to cancel")
		}
		return response.InternalError(c, "failed to fetch subscription")
	}

	result := database.DB.Model(&models.MarketPlanSubscription{}).
		Where("id = ? AND status = ?", sub.ID, models.SubscriptionActive).
		Update("status", models.SubscriptionCancelled)
	if result.Error != nil {
		return response.InternalError(c, "failed to cancel subscription")
	}
	if result.RowsAffected == 0 {
		return response.BadRequest(c, "no active market plan to cancel")
	}

	return response.OK(c, fiber.Map{"message": "market plan cancelled", "subscription_id": sub.ID})
}

// HandleCreateMarketPlanOrder godoc
// @Summary     Create a Razorpay order for a Product Market plan
// @Tags        User Market Plans
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Market Plan ID"
// @Success     200  {object}  map[string]interface{}
// @Router      /user/market/plans/{id}/order [post]
func HandleCreateMarketPlanOrder(c *fiber.Ctx) error {
	planID := c.Params("id")
	userID, _ := c.Locals("userID").(string)

	var plan models.MarketPlan
	if err := database.DB.First(&plan, "id = ? AND is_active = true", planID).Error; err != nil {
		return response.NotFound(c, "market plan not found")
	}

	var activeCount int64
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("user_id = ? AND status = ? AND expiry_date > ?", userID, models.SubscriptionActive, time.Now()).
		Count(&activeCount)
	if activeCount > 0 {
		return response.BadRequest(c, "you already have an active market plan")
	}

	if config.App.RazorpayKeyID == "" || config.App.RazorpayKeySecret == "" ||
		config.App.RazorpayKeyID == "rzp_test_xxxx" || config.App.RazorpayKeySecret == "xxxx" {
		return response.InternalError(c, "razorpay is not configured on the server")
	}

	rzpOrderID, err := createMarketPlanRazorpayOrder(plan.PriceInPaise, userID, plan.ID)
	if err != nil {
		slog.Error("market plans: razorpay order creation failed", "error", err, "user_id", userID, "plan_id", plan.ID)
		return response.InternalError(c, "failed to create payment order")
	}

	sub := models.MarketPlanSubscription{
		UserID:          userID,
		PlanID:          plan.ID,
		Status:          models.SubscriptionPending,
		StartDate:       time.Now(),
		ExpiryDate:      time.Now(), // set on verify
		AmountInPaise:   plan.PriceInPaise,
		RazorpayOrderID: &rzpOrderID,
	}
	if err := database.DB.Create(&sub).Error; err != nil {
		return response.InternalError(c, "failed to create pending subscription")
	}

	return response.OK(c, fiber.Map{
		"order_id": rzpOrderID,
		"amount":   plan.PriceInPaise,
		"currency": "INR",
		"key_id":   config.App.RazorpayKeyID,
	})
}

// HandleVerifyMarketPlan godoc
// @Summary     Verify Razorpay payment and activate Product Market plan
// @Tags        User Market Plans
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "razorpay_order_id, razorpay_payment_id, razorpay_signature"
// @Success     200  {object}  map[string]string
// @Router      /user/market/plans/verify [post]
func HandleVerifyMarketPlan(c *fiber.Ctx) error {
	var body struct {
		RazorpayOrderID   string `json:"razorpay_order_id"`
		RazorpayPaymentID string `json:"razorpay_payment_id"`
		RazorpaySignature string `json:"razorpay_signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.RazorpayOrderID == "" || body.RazorpayPaymentID == "" || body.RazorpaySignature == "" {
		return response.BadRequest(c, "razorpay_order_id, razorpay_payment_id, and razorpay_signature are required")
	}

	userID, _ := c.Locals("userID").(string)

	msg := body.RazorpayOrderID + "|" + body.RazorpayPaymentID
	mac := hmac.New(sha256.New, []byte(config.App.RazorpayKeySecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(body.RazorpaySignature), []byte(expected)) {
		return response.BadRequest(c, "invalid razorpay signature")
	}

	var sub models.MarketPlanSubscription
	if err := database.DB.Preload("Plan").
		Where("user_id = ? AND razorpay_order_id = ?", userID, body.RazorpayOrderID).
		First(&sub).Error; err != nil {
		return response.NotFound(c, "pending subscription not found")
	}
	if sub.Status == models.SubscriptionActive {
		return response.OK(c, fiber.Map{"message": "plan already activated", "subscription_id": sub.ID})
	}

	var activeCount int64
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("user_id = ? AND status = ? AND expiry_date > ? AND id != ?",
			userID, models.SubscriptionActive, time.Now(), sub.ID).
		Count(&activeCount)
	if activeCount > 0 {
		return response.BadRequest(c, "you already have an active market plan")
	}

	if !activateMarketPlanSub(&sub, body.RazorpayPaymentID) {
		return response.OK(c, fiber.Map{"message": "plan already activated", "subscription_id": sub.ID})
	}

	return response.OK(c, fiber.Map{"message": "market plan activated", "subscription_id": sub.ID})
}

// HandleSubscribeMarketPlanWithWallet godoc
// @Summary     Pay for a Product Market plan using wallet balance
// @Tags        User Market Plans
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Market Plan ID"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Router      /user/market/plans/{id}/wallet [post]
func HandleSubscribeMarketPlanWithWallet(c *fiber.Ctx) error {
	planID := c.Params("id")
	userID, _ := c.Locals("userID").(string)

	var plan models.MarketPlan
	if err := database.DB.First(&plan, "id = ? AND is_active = true", planID).Error; err != nil {
		return response.NotFound(c, "market plan not found")
	}

	var activeCount int64
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("user_id = ? AND status = ? AND expiry_date > ?", userID, models.SubscriptionActive, time.Now()).
		Count(&activeCount)
	if activeCount > 0 {
		return response.BadRequest(c, "you already have an active market plan")
	}

	now := time.Now()
	expiresAt := marketPlanExpiry(now, plan.DurationDays)

	sub := models.MarketPlanSubscription{
		UserID:        userID,
		PlanID:        plan.ID,
		Status:        models.SubscriptionActive,
		StartDate:     now,
		ExpiryDate:    expiresAt,
		AmountInPaise: plan.PriceInPaise,
		PaidAt:        &now,
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sub).Error; err != nil {
			return err
		}
		if _, err := wallet.Apply(tx, userID, models.WalletTxPlanDebit, -plan.PriceInPaise,
			&sub.ID, models.JSONMap{"plan_id": plan.ID, "plan_name": plan.Name}); err != nil {
			return err
		}
		_, err := platform_wallet.Apply(tx, models.PlatformSourceMarketPlan, plan.PriceInPaise,
			&sub.ID, models.JSONMap{"plan_id": plan.ID, "plan_name": plan.Name, "paid_via": "WALLET"})
		return err
	})
	if err == wallet.ErrInsufficientBalance {
		return response.BadRequest(c, "insufficient wallet balance")
	}
	if err != nil {
		return response.InternalErrorWithLog(c, "market plans: wallet subscribe", err)
	}

	return response.OK(c, fiber.Map{"message": "market plan activated", "subscription_id": sub.ID})
}

func marketPlanExpiry(start time.Time, durationDays int) time.Time {
	if durationDays <= 0 {
		durationDays = 30
	}
	return start.AddDate(0, 0, durationDays)
}

// activateMarketPlanSub flips a PENDING market plan subscription to ACTIVE.
// Idempotent via the status != ACTIVE guard.
func activateMarketPlanSub(sub *models.MarketPlanSubscription, razorpayPaymentID string) bool {
	now := time.Now()
	expiresAt := marketPlanExpiry(now, sub.Plan.DurationDays)

	activated := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.MarketPlanSubscription{}).
			Where("id = ? AND status = ?", sub.ID, models.SubscriptionPending).
			Updates(map[string]interface{}{
				"status":              models.SubscriptionActive,
				"razorpay_payment_id": razorpayPaymentID,
				"paid_at":             now,
				"start_date":          now,
				"expiry_date":         expiresAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if _, err := platform_wallet.Apply(tx, models.PlatformSourceMarketPlan, sub.AmountInPaise,
			&sub.ID, models.JSONMap{"plan_id": sub.PlanID, "paid_via": "RAZORPAY"}); err != nil {
			return err
		}
		activated = true
		return nil
	})
	if err != nil {
		slog.Error("market plans: failed to activate subscription", "subscription_id", sub.ID, "error", err)
		return false
	}
	return activated
}

// CaptureMarketPlanOrder resolves a webhook payment.captured event against
// market_plan_subscriptions. Returns true if a market plan sub was activated.
func CaptureMarketPlanOrder(orderID, razorpayPaymentID string) bool {
	var sub models.MarketPlanSubscription
	if err := database.DB.Preload("Plan").
		Where("razorpay_order_id = ?", orderID).First(&sub).Error; err != nil {
		return false
	}
	if sub.Status == models.SubscriptionActive {
		return true
	}
	var activeCount int64
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("user_id = ? AND status = ? AND expiry_date > ? AND id != ?",
			sub.UserID, models.SubscriptionActive, time.Now(), sub.ID).
		Count(&activeCount)
	if activeCount > 0 {
		return true // already has active; ignore duplicate payment
	}
	activateMarketPlanSub(&sub, razorpayPaymentID)
	return true
}

// ── Admin handlers ────────────────────────────────────────────────────────────

// HandleAdminListMarketPlans godoc
// @Summary     List all Product Market plans with subscriber counts (admin)
// @Tags        Admin Market Plans
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []marketPlanWithStats
// @Router      /market/plans [get]
func HandleAdminListMarketPlans(c *fiber.Ctx) error {
	var result []marketPlanWithStats
	if err := database.DB.Raw(`
		SELECT market_plans.*, COALESCE(s.cnt, 0) AS subscribers
		FROM market_plans
		LEFT JOIN (
			SELECT plan_id, COUNT(*) AS cnt
			FROM market_plan_subscriptions
			WHERE status = ?
			GROUP BY plan_id
		) s ON s.plan_id = market_plans.id
		ORDER BY market_plans.created_at DESC
	`, models.SubscriptionActive).Scan(&result).Error; err != nil {
		return response.InternalError(c, "failed to fetch market plans")
	}
	if result == nil {
		result = []marketPlanWithStats{}
	}
	return response.OK(c, result)
}

// HandleAdminCreateMarketPlan godoc
// @Summary     Create a Product Market plan (admin)
// @Tags        Admin Market Plans
// @Produce     json
// @Security    AdminAuth
// @Router      /market/plans [post]
func HandleAdminCreateMarketPlan(c *fiber.Ctx) error {
	var body struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		PriceInPaise   int64  `json:"price_in_paise"`
		DurationDays   int    `json:"duration_days"`
		FeeDiscountPct int    `json:"fee_discount_pct"`
		FeaturedSeller bool   `json:"featured_seller"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.Name == "" {
		return response.BadRequest(c, "plan name is required")
	}
	if body.PriceInPaise <= 0 {
		return response.BadRequest(c, "price must be greater than 0")
	}
	if body.DurationDays <= 0 {
		body.DurationDays = 30
	}
	if body.FeeDiscountPct < 0 || body.FeeDiscountPct > 100 {
		return response.BadRequest(c, "fee_discount_pct must be between 0 and 100")
	}

	plan := models.MarketPlan{
		Name:           body.Name,
		Description:    body.Description,
		PriceInPaise:   body.PriceInPaise,
		DurationDays:   body.DurationDays,
		FeeDiscountPct: body.FeeDiscountPct,
		FeaturedSeller: body.FeaturedSeller,
		IsActive:       true,
	}
	if err := database.DB.Create(&plan).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return response.BadRequest(c, "a market plan with this name already exists")
		}
		return response.InternalError(c, "failed to create market plan")
	}
	return response.Created(c, marketPlanWithStats{MarketPlan: plan, Subscribers: 0})
}

// HandleAdminUpdateMarketPlan godoc
// @Summary     Update a Product Market plan (admin)
// @Tags        Admin Market Plans
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Plan ID"
// @Router      /market/plans/{id} [put]
func HandleAdminUpdateMarketPlan(c *fiber.Ctx) error {
	var plan models.MarketPlan
	if err := database.DB.First(&plan, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "market plan not found")
	}

	var body struct {
		Name           *string `json:"name"`
		Description    *string `json:"description"`
		PriceInPaise   *int64  `json:"price_in_paise"`
		DurationDays   *int    `json:"duration_days"`
		FeeDiscountPct *int    `json:"fee_discount_pct"`
		FeaturedSeller *bool   `json:"featured_seller"`
		IsActive       *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	updates := map[string]interface{}{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Description != nil {
		updates["description"] = *body.Description
	}
	if body.PriceInPaise != nil {
		if *body.PriceInPaise <= 0 {
			return response.BadRequest(c, "price must be greater than 0")
		}
		updates["price_in_paise"] = *body.PriceInPaise
	}
	if body.DurationDays != nil {
		if *body.DurationDays <= 0 {
			return response.BadRequest(c, "duration must be greater than 0 days")
		}
		updates["duration_days"] = *body.DurationDays
	}
	if body.FeeDiscountPct != nil {
		if *body.FeeDiscountPct < 0 || *body.FeeDiscountPct > 100 {
			return response.BadRequest(c, "fee_discount_pct must be between 0 and 100")
		}
		updates["fee_discount_pct"] = *body.FeeDiscountPct
	}
	if body.FeaturedSeller != nil {
		updates["featured_seller"] = *body.FeaturedSeller
	}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}

	if err := database.DB.Model(&plan).Updates(updates).Error; err != nil {
		return response.InternalError(c, "failed to update market plan")
	}
	database.DB.First(&plan, "id = ?", plan.ID)
	return response.OK(c, plan)
}

// HandleAdminDeleteMarketPlan godoc
// @Summary     Delete a Product Market plan (admin)
// @Tags        Admin Market Plans
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Plan ID"
// @Router      /market/plans/{id} [delete]
func HandleAdminDeleteMarketPlan(c *fiber.Ctx) error {
	var plan models.MarketPlan
	if err := database.DB.First(&plan, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "market plan not found")
	}

	var activeSubscribers int64
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("plan_id = ? AND status = ?", plan.ID, models.SubscriptionActive).
		Count(&activeSubscribers)
	if activeSubscribers > 0 {
		return response.BadRequest(c, "cannot delete a plan with active subscribers")
	}

	if err := database.DB.Delete(&models.MarketPlan{}, "id = ?", plan.ID).Error; err != nil {
		return response.InternalError(c, "failed to delete market plan")
	}
	return response.OK(c, fiber.Map{"message": "market plan deleted"})
}

type marketPlanPaymentRow struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	PlanID            string     `json:"plan_id"`
	Status            string     `json:"status"`
	StartDate         time.Time  `json:"start_date"`
	ExpiryDate        time.Time  `json:"expiry_date"`
	AmountInPaise     int64      `json:"amount_in_paise"`
	RazorpayOrderID   *string    `json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID *string    `json:"razorpay_payment_id,omitempty"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	Gateway           string     `json:"gateway"`
	User              *struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user,omitempty"`
	Plan *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"plan,omitempty"`
}

// HandleAdminListMarketPlanPayments godoc
// @Summary     List Product Market plan subscription payments (admin)
// @Tags        Admin Market Plans
// @Produce     json
// @Security    AdminAuth
// @Param       page    query  int     false  "Page"
// @Param       limit   query  int     false  "Limit"
// @Param       search  query  string  false  "Search user name or email"
// @Param       status  query  string  false  "ACTIVE|EXPIRED|CANCELLED|PENDING"
// @Success     200  {object}  map[string]interface{}
// @Router      /market/plan-payments [get]
func HandleAdminListMarketPlanPayments(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.MarketPlanSubscription{}).
		Preload("User").Preload("Plan")

	// Default: paid subscriptions only (exclude unpaid PENDING orders).
	if st := c.Query("status"); st != "" {
		q = q.Where("market_plan_subscriptions.status = ?", st)
	} else {
		q = q.Where(
			"market_plan_subscriptions.paid_at IS NOT NULL OR market_plan_subscriptions.status IN ?",
			[]string{
				string(models.SubscriptionActive),
				string(models.SubscriptionExpired),
				string(models.SubscriptionCancelled),
			},
		)
	}

	if s := c.Query("search"); s != "" {
		q = q.Joins("JOIN users ON users.id = market_plan_subscriptions.user_id").
			Where("users.name ILIKE ? OR users.email ILIKE ?", "%"+s+"%", "%"+s+"%")
	}

	var total int64
	q.Count(&total)

	var subs []models.MarketPlanSubscription
	if err := q.Offset((page - 1) * limit).Limit(limit).
		Order("market_plan_subscriptions.created_at DESC").
		Find(&subs).Error; err != nil {
		return response.InternalError(c, "failed to fetch market plan payments")
	}

	rows := make([]marketPlanPaymentRow, 0, len(subs))
	for _, sub := range subs {
		gateway := "WALLET"
		if sub.RazorpayPaymentID != nil && *sub.RazorpayPaymentID != "" {
			gateway = "RAZORPAY"
		}
		row := marketPlanPaymentRow{
			ID:                sub.ID,
			UserID:            sub.UserID,
			PlanID:            sub.PlanID,
			Status:            string(sub.Status),
			StartDate:         sub.StartDate,
			ExpiryDate:        sub.ExpiryDate,
			AmountInPaise:     sub.AmountInPaise,
			RazorpayOrderID:   sub.RazorpayOrderID,
			RazorpayPaymentID: sub.RazorpayPaymentID,
			PaidAt:            sub.PaidAt,
			CreatedAt:         sub.CreatedAt,
			Gateway:           gateway,
		}
		if sub.User.ID != "" {
			row.User = &struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			}{ID: sub.User.ID, Name: sub.User.Name, Email: sub.User.Email}
		}
		if sub.Plan.ID != "" {
			row.Plan = &struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}{ID: sub.Plan.ID, Name: sub.Plan.Name}
		}
		rows = append(rows, row)
	}

	return response.Paginated(c, rows, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

func createMarketPlanRazorpayOrder(amountInPaise int64, userID, planID string) (string, error) {
	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("mp_%s_%s", userID[:8], planID[:8]),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(config.App.RazorpayKeyID, config.App.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("razorpay error: %s", string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return "", fmt.Errorf("razorpay returned empty order id")
	}
	return id, nil
}
