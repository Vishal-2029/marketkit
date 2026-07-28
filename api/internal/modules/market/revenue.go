package market

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleMarketRevenueSummary godoc
// @Summary     Product Market revenue summary — market-only, never learning (admin)
// @Tags        Admin Market Revenue
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Router      /market/revenue/summary [get]
func HandleMarketRevenueSummary(c *fiber.Ctx) error {
	var planRevenue, feeRevenue, grossSales, sellerPayouts int64
	var planCount, saleCount int64

	// Plan revenue: full amount of every paid market-plan subscription
	// (Razorpay or wallet) — matches the platform wallet's MARKET_PLAN source.
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("paid_at IS NOT NULL").
		Select("COALESCE(SUM(amount_minor), 0)").Scan(&planRevenue)
	database.DB.Model(&models.MarketPlanSubscription{}).
		Where("paid_at IS NOT NULL").Count(&planCount)

	// Product sales: platform keeps only the fee — the rest (seller_net) is
	// the seller's money, not platform revenue. Both are surfaced here for
	// context but only the fee counts toward platform revenue.
	database.DB.Model(&models.ProductPurchase{}).
		Where("status = ?", models.PaymentSuccess).
		Select("COALESCE(SUM(fee_minor), 0) AS fee, COALESCE(SUM(amount_minor), 0) AS gross, COALESCE(SUM(seller_net_minor), 0) AS payouts").
		Row().Scan(&feeRevenue, &grossSales, &sellerPayouts)
	database.DB.Model(&models.ProductPurchase{}).
		Where("status = ?", models.PaymentSuccess).Count(&saleCount)

	return response.OK(c, fiber.Map{
		"platform_revenue_minor": planRevenue + feeRevenue,
		"plan_revenue_minor":     planRevenue,
		"fee_revenue_minor":      feeRevenue,
		"gross_sales_minor":      grossSales,
		"seller_payouts_minor":   sellerPayouts,
		"plan_count":             planCount,
		"sale_count":             saleCount,
	})
}

// HandleMarketRevenueMonthly godoc
// @Summary     Monthly Product Market revenue breakdown — market-only (admin)
// @Tags        Admin Market Revenue
// @Produce     json
// @Security    AdminAuth
// @Param       year  query  int  false  "Year (default: current year)"
// @Success     200  {object}  []map[string]interface{}
// @Router      /market/revenue/monthly [get]
func HandleMarketRevenueMonthly(c *fiber.Ctx) error {
	year := c.Query("year", strconv.Itoa(time.Now().Year()))

	type row struct {
		Month int
		Total int64
	}

	var planRows []row
	database.DB.Raw(`
		SELECT EXTRACT(MONTH FROM paid_at)::int AS month,
		       COALESCE(SUM(amount_minor), 0) AS total
		FROM market_plan_subscriptions
		WHERE paid_at IS NOT NULL AND EXTRACT(YEAR FROM paid_at) = ?
		GROUP BY month
	`, year).Scan(&planRows)

	var feeRows []row
	database.DB.Raw(`
		SELECT EXTRACT(MONTH FROM paid_at)::int AS month,
		       COALESCE(SUM(fee_minor), 0) AS total
		FROM product_purchases
		WHERE status = ? AND EXTRACT(YEAR FROM paid_at) = ?
		GROUP BY month
	`, models.PaymentSuccess, year).Scan(&feeRows)

	planByMonth := make(map[int]int64, len(planRows))
	for _, r := range planRows {
		planByMonth[r.Month] = r.Total
	}
	feeByMonth := make(map[int]int64, len(feeRows))
	for _, r := range feeRows {
		feeByMonth[r.Month] = r.Total
	}

	type monthResult struct {
		Month             int   `json:"month"`
		PlanRevenueMinor  int64 `json:"plan_revenue_minor"`
		FeeRevenueMinor   int64 `json:"fee_revenue_minor"`
		TotalRevenueMinor int64 `json:"total_revenue_minor"`
	}
	results := make([]monthResult, 12)
	for m := 1; m <= 12; m++ {
		results[m-1] = monthResult{
			Month:             m,
			PlanRevenueMinor:  planByMonth[m],
			FeeRevenueMinor:   feeByMonth[m],
			TotalRevenueMinor: planByMonth[m] + feeByMonth[m],
		}
	}

	return response.OK(c, results)
}
