package dashboard

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleStats godoc
// @Summary     Get site statistics for dashboard
// @Tags        Admin Dashboard
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /dashboard/stats [get]
func HandleStats(c *fiber.Ctx) error {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var totalUsers, activeSubs, totalVideos int64
	var monthRevenue int64

	database.DB.Model(&models.User{}).Count(&totalUsers)
	database.DB.Model(&models.Subscription{}).Where("status = ?", models.SubscriptionActive).Count(&activeSubs)
	database.DB.Model(&models.Video{}).Where("status = ?", models.VideoStatusPublished).Count(&totalVideos)
	database.DB.Model(&models.Payment{}).
		Where("status = ? AND paid_at >= ?", models.PaymentSuccess, startOfMonth).
		Select("COALESCE(SUM(amount_minor), 0)").Scan(&monthRevenue)

	return response.OK(c, fiber.Map{
		"total_users":         totalUsers,
		"active_subs":         activeSubs,
		"published_videos":    totalVideos,
		"month_revenue_minor": monthRevenue,
	})
}

// HandleRecentPayments godoc
// @Summary     Get recent payments (admin)
// @Tags        Admin Dashboard
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /dashboard/payments [get]
func HandleRecentPayments(c *fiber.Ctx) error {
	var payments []models.Payment
	database.DB.Preload("User").Preload("Plan").
		Where("status = ?", models.PaymentSuccess).
		Order("paid_at DESC").Limit(4).Find(&payments)
	// Trim the preloaded User down to name/email — the dashboard widget only
	// displays those, not phone/wallet balance/avatar/status.
	for i := range payments {
		payments[i].User = models.User{ID: payments[i].User.ID, Name: payments[i].User.Name, Email: payments[i].User.Email}
	}
	return response.OK(c, payments)
}

// HandleRecentActivity godoc
// @Summary     Get recent audit logs (admin)
// @Tags        Admin Dashboard
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /dashboard/activity [get]
func HandleRecentActivity(c *fiber.Ctx) error {
	var logs []models.AuditLog
	database.DB.Order("created_at DESC").Limit(10).Find(&logs)
	return response.OK(c, logs)
}

// HandleUserGrowth godoc
// @Summary     Get user growth by month over last 12 months (admin)
// @Tags        Admin Dashboard
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /dashboard/user-growth [get]
func HandleUserGrowth(c *fiber.Ctx) error {
	type monthRow struct {
		Month int   `json:"month"`
		Year  int   `json:"year"`
		Count int64 `json:"count"`
	}
	var rows []monthRow
	database.DB.Raw(`
		SELECT EXTRACT(MONTH FROM joined_at)::int AS month,
		       EXTRACT(YEAR FROM joined_at)::int AS year,
		       COUNT(*) AS count
		FROM users
		WHERE joined_at >= NOW() - INTERVAL '12 months'
		GROUP BY year, month ORDER BY year, month
	`).Scan(&rows)
	return response.OK(c, rows)
}

// HandlePlanDistribution godoc
// @Summary     Get subscription plan distribution (admin)
// @Tags        Admin Dashboard
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /dashboard/plan-distribution [get]
func HandlePlanDistribution(c *fiber.Ctx) error {
	type distRow struct {
		PlanName    string `json:"plan_name"`
		Subscribers int64  `json:"subscribers"`
	}
	var rows []distRow
	database.DB.Raw(`
		SELECT p.name AS plan_name, COUNT(s.id) AS subscribers
		FROM plans p
		LEFT JOIN subscriptions s ON s.plan_id = p.id AND s.status = 'ACTIVE'
		GROUP BY p.name ORDER BY subscribers DESC
	`).Scan(&rows)
	return response.OK(c, rows)
}
