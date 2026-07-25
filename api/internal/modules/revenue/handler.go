// Package revenue is learning-only. Every handler here reads exclusively
// from payments / plans / subscriptions / users — product-market money
// (product_purchases, market_plan_subscriptions) must never appear in these
// queries. Product Market's own revenue lives in the market module
// (HandleMarketRevenueSummary / HandleMarketRevenueMonthly).
package revenue

import (
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleSummary godoc
// @Summary     Revenue summary
// @Tags        Revenue
// @Produce     json
// @Success     200  {object}  map[string]interface{}
// @Router      /revenue/summary [get]
// Learning-only scope: sums payments/subscriptions/users. Never touches
// product_purchases or market_plan_subscriptions.
func HandleSummary(c *fiber.Ctx) error {
	var totalRevenue, monthRevenue int64
	var activeSubs, totalUsers int64

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	database.DB.Model(&models.Payment{}).
		Where("status = ?", models.PaymentSuccess).
		Select("COALESCE(SUM(amount_in_paise), 0)").Scan(&totalRevenue)

	database.DB.Model(&models.Payment{}).
		Where("status = ? AND paid_at >= ?", models.PaymentSuccess, startOfMonth).
		Select("COALESCE(SUM(amount_in_paise), 0)").Scan(&monthRevenue)

	database.DB.Model(&models.Subscription{}).
		Where("status = ?", models.SubscriptionActive).Count(&activeSubs)

	database.DB.Model(&models.User{}).Count(&totalUsers)

	var arpu int64
	if activeSubs > 0 {
		arpu = monthRevenue / activeSubs
	}

	return response.OK(c, fiber.Map{
		"total_revenue_paise":  totalRevenue,
		"month_revenue_paise":  monthRevenue,
		"active_subscriptions": activeSubs,
		"total_users":          totalUsers,
		"arpu_paise":           arpu,
	})
}

// HandleMonthly godoc
// @Summary     Monthly revenue breakdown (admin)
// @Tags        Revenue
// @Produce     json
// @Security    AdminAuth
// @Param       year  query  int  false  "Year (default: current year)"
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /revenue/monthly [get]
// Learning-only scope: sums the payments table only. Never touches
// product_purchases or market_plan_subscriptions.
func HandleMonthly(c *fiber.Ctx) error {
	year := c.Query("year", strconv.Itoa(time.Now().Year()))

	type monthlyRow struct {
		Month   int   `json:"month"`
		Revenue int64 `json:"revenue_paise"`
	}
	var rows []monthlyRow
	database.DB.Raw(`
		SELECT EXTRACT(MONTH FROM paid_at)::int AS month,
		       COALESCE(SUM(amount_in_paise), 0) AS revenue_paise
		FROM payments
		WHERE status = ? AND EXTRACT(YEAR FROM paid_at) = ?
		GROUP BY month ORDER BY month
	`, models.PaymentSuccess, year).Scan(&rows)

	return response.OK(c, rows)
}

// HandleByPlan godoc
// @Summary     Revenue breakdown by plan (admin)
// @Tags        Revenue
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /revenue/by-plan [get]
// Learning-only scope: joins plans/subscriptions/payments only. Never
// touches product_purchases or market_plan_subscriptions.
func HandleByPlan(c *fiber.Ctx) error {
	type planRow struct {
		PlanID      string `json:"plan_id"`
		PlanName    string `json:"plan_name"`
		Subscribers int64  `json:"subscribers"`
		Revenue     int64  `json:"revenue_paise"`
	}
	var rows []planRow
	database.DB.Raw(`
		SELECT p.id AS plan_id, p.name AS plan_name,
		       COUNT(DISTINCT s.id) AS subscribers,
		       COALESCE(SUM(pay.amount_in_paise), 0) AS revenue
		FROM plans p
		LEFT JOIN subscriptions s ON s.plan_id = p.id AND s.status = 'ACTIVE'
		LEFT JOIN payments pay ON pay.plan_id = p.id AND pay.status = 'SUCCESS'
		GROUP BY p.id, p.name
		ORDER BY revenue DESC
	`).Scan(&rows)
	return response.OK(c, rows)
}

// HandleRenewalStats godoc
// @Summary     Renewal and churn statistics
// @Tags        Revenue
// @Produce     json
// @Success     200  {object}  map[string]interface{}
// @Router      /revenue/renewal-stats [get]
// Learning-only scope: reads the learning subscriptions/plans tables only.
// Never touches product_purchases or market_plan_subscriptions.
func HandleRenewalStats(c *fiber.Ctx) error {
	now := time.Now()
	cutoff := now.AddDate(0, 0, -90)
	next30 := now.AddDate(0, 0, 30)

	// Subscriptions whose expiry fell within the last 90 days
	var totalExpired int64
	database.DB.Model(&models.Subscription{}).
		Where("expiry_date >= ? AND expiry_date < ?", cutoff, now).
		Count(&totalExpired)

	// Renewals: distinct users who expired in window AND got a new subscription afterwards
	var renewals int64
	database.DB.Raw(`
		SELECT COUNT(DISTINCT s1.user_id)
		FROM subscriptions s1
		WHERE s1.expiry_date >= ? AND s1.expiry_date < ?
		  AND EXISTS (
		      SELECT 1 FROM subscriptions s2
		      WHERE s2.user_id = s1.user_id
		        AND s2.start_date > s1.expiry_date
		  )
	`, cutoff, now).Scan(&renewals)

	var renewalRate float64
	if totalExpired > 0 {
		renewalRate = float64(renewals) / float64(totalExpired) * 100
	}
	var churnRate float64
	if totalExpired == 0 {
		churnRate = 0
	} else {
		churnRate = 100.0 - renewalRate
	}

	type planForecast struct {
		PlanName      string `json:"plan_name"`
		ExpiringCount int64  `json:"expiring_count"`
		ValuePaise    int64  `json:"expected_value_paise"`
	}
	var byPlan []planForecast
	database.DB.Raw(`
		SELECT p.name AS plan_name,
		       COUNT(s.id) AS expiring_count,
		       COALESCE(SUM(p.price_in_paise), 0) AS value_paise
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.status = 'ACTIVE' AND s.expiry_date <= ?
		GROUP BY p.id, p.name
		ORDER BY expiring_count DESC
	`, next30).Scan(&byPlan)
	if byPlan == nil {
		byPlan = []planForecast{}
	}

	return response.OK(c, fiber.Map{
		"total_expired_90d": totalExpired,
		"renewals_90d":      renewals,
		"renewal_rate_pct":  math.Round(renewalRate*10) / 10,
		"churn_rate_pct":    math.Round(churnRate*10) / 10,
		"forecast_by_plan":  byPlan,
	})
}

// HandleForecast godoc
// @Summary     Upcoming subscription expiry forecast (admin)
// @Tags        Revenue
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /revenue/forecast [get]
// Learning-only scope: reads the learning subscriptions/plans/users tables
// only. Never touches product_purchases or market_plan_subscriptions.
func HandleForecast(c *fiber.Ctx) error {
	next30 := time.Now().AddDate(0, 0, 30)

	type forecastRow struct {
		UserID     string    `json:"user_id"`
		UserName   string    `json:"user_name"`
		PlanName   string    `json:"plan_name"`
		ExpiryDate time.Time `json:"expiry_date"`
		ValuePaise int64     `json:"value_paise"`
	}
	var rows []forecastRow
	database.DB.Raw(`
		SELECT u.id AS user_id, u.name AS user_name, p.name AS plan_name,
		       s.expiry_date, p.price_in_paise AS value_paise
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN plans p ON p.id = s.plan_id
		WHERE s.status = 'ACTIVE' AND s.expiry_date <= ?
		ORDER BY s.expiry_date ASC
	`, next30).Scan(&rows)

	return response.OK(c, rows)
}
