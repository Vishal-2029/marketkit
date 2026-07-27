package plans

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

type planWithStats struct {
	models.Plan
	Subscribers int64 `json:"subscribers"`
}

// HandlePublicList returns all active plans without requiring authentication.
// Used by the Flutter app to display subscription options.
// HandlePublicList godoc
// @Summary     List active plans (public)
// @Tags        Plans
// @Produce     json
// @Success     200  {object}  []models.Plan
// @Failure     500  {object}  map[string]string
// @Router      /plans/public [get]
func HandlePublicList(c *fiber.Ctx) error {
	var plans []models.Plan
	if err := database.DB.Where("is_active = true").Order("price_in_paise ASC").Find(&plans).Error; err != nil {
		return response.InternalError(c, "failed to fetch plans")
	}
	return response.OK(c, plans)
}

// HandleList godoc
// @Summary     List all plans with subscriber counts (admin)
// @Tags        Admin Plans
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []planWithStats
// @Failure     401  {object}  map[string]string
// @Router      /plans [get]
func HandleList(c *fiber.Ctx) error {
	var result []planWithStats
	if err := database.DB.Raw(`
		SELECT plans.*, COALESCE(s.cnt, 0) AS subscribers
		FROM plans
		LEFT JOIN (
			SELECT plan_id, COUNT(*) AS cnt
			FROM subscriptions
			WHERE status = ?
			GROUP BY plan_id
		) s ON s.plan_id = plans.id
		ORDER BY plans.created_at DESC
	`, models.SubscriptionActive).Scan(&result).Error; err != nil {
		return response.InternalError(c, "failed to fetch plans")
	}
	if result == nil {
		result = []planWithStats{}
	}
	return response.OK(c, result)
}

// HandleCreate godoc
// @Summary     Create a new plan (admin)
// @Tags        Admin Plans
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]interface{}  true  "Plan details"
// @Success     201  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /plans [post]
func HandleCreate(c *fiber.Ctx) error {
	var body struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		PriceInPaise int64    `json:"price_in_paise"`
		Features     []string `json:"features"`
		DurationDays int      `json:"duration_days"`
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
		body.DurationDays = 365
	}

	plan := models.Plan{
		Name:         body.Name,
		Description:  body.Description,
		PriceInPaise: body.PriceInPaise,
		Features:     body.Features,
		DurationDays: body.DurationDays,
		IsActive:     true,
	}
	if err := database.DB.Create(&plan).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return response.BadRequest(c, "a plan with this name already exists")
		}
		return response.InternalError(c, "failed to create plan")
	}
	return response.Created(c, planWithStats{Plan: plan, Subscribers: 0})
}

// HandleUpdate godoc
// @Summary     Update a plan (admin)
// @Tags        Admin Plans
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Plan ID"
// @Param       body  body  map[string]interface{}  true  "Plan fields to update"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /plans/{id} [put]
func HandleUpdate(c *fiber.Ctx) error {
	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	var body struct {
		Name         *string   `json:"name"`
		Description  *string   `json:"description"`
		PriceInPaise *int64    `json:"price_in_paise"`
		Features     *[]string `json:"features"`
		DurationDays *int      `json:"duration_days"`
		IsActive     *bool     `json:"is_active"`
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
	if body.DurationDays != nil && *body.DurationDays <= 0 {
		return response.BadRequest(c, "duration must be greater than 0 days")
	}
	if body.DurationDays != nil {
		updates["duration_days"] = *body.DurationDays
	}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&plan).Updates(updates).Error; err != nil {
			return response.InternalError(c, "failed to update plan")
		}
	}

	// Features is a json-serialized column. A map-based Updates() writes the
	// raw Go slice and bypasses the serializer, storing "[CATEGORY_A]" instead
	// of ["CATEGORY_A"] — which then fails to unmarshal on the next read. Go
	// through the struct field so the serializer runs.
	if body.Features != nil {
		plan.Features = *body.Features
		if err := database.DB.Model(&plan).Select("Features").Updates(&plan).Error; err != nil {
			return response.InternalError(c, "failed to update plan")
		}
	}

	// Re-read so the response reflects every column, not just the ones the
	// caller happened to send.
	if err := database.DB.First(&plan, "id = ?", plan.ID).Error; err != nil {
		return response.InternalError(c, "failed to load updated plan")
	}
	return response.OK(c, plan)
}

// HandleDelete godoc
// @Summary     Delete a plan (admin)
// @Tags        Admin Plans
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Plan ID"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /plans/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	var plan models.Plan
	if err := database.DB.First(&plan, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "plan not found")
	}

	var activeSubscribers int64
	database.DB.Model(&models.Subscription{}).
		Where("plan_id = ? AND status = ?", plan.ID, models.SubscriptionActive).
		Count(&activeSubscribers)
	if activeSubscribers > 0 {
		return response.BadRequest(c, "cannot delete a plan with active subscribers")
	}

	if err := database.DB.Delete(&models.Plan{}, "id = ?", plan.ID).Error; err != nil {
		return response.InternalError(c, "failed to delete plan")
	}
	return response.OK(c, fiber.Map{"message": "plan deleted"})
}
