package market

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
)

// HandleAdminListDesigns godoc
// @Summary     List all marketplace designs (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       search  query  string  false  "Search by title or seller"
// @Param       page    query  int     false  "Page number"
// @Param       limit   query  int     false  "Page size"
// @Success     200  {object}  []models.Design
// @Failure     401  {object}  map[string]string
// @Router      /market/designs [get]
func HandleAdminListDesigns(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.Design{}).Preload("Seller").Preload("Category")
	if s := c.Query("search"); s != "" {
		q = q.Joins("JOIN users ON users.id = designs.seller_id").
			Where("designs.title ILIKE ? OR users.name ILIKE ? OR users.email ILIKE ?",
				"%"+s+"%", "%"+s+"%", "%"+s+"%")
	}
	if catID := c.Query("category_id"); catID != "" {
		q = q.Where("designs.category_id = ?", catID)
	}

	var total int64
	q.Count(&total)

	var designs []models.Design
	if err := q.Offset((page - 1) * limit).Limit(limit).Order("designs.created_at DESC").Find(&designs).Error; err != nil {
		return response.InternalError(c, "failed to fetch designs")
	}

	for i := range designs {
		designs[i].SellerName = designs[i].Seller.Name
		designs[i].SellerEmail = designs[i].Seller.Email
	}
	populateDesignURLs(designs, "", nil)

	return response.Paginated(c, designs, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleAdminDeleteDesign godoc
// @Summary     Take down a marketplace design (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/designs/{id} [delete]
func HandleAdminDeleteDesign(c *fiber.Ctx) error {
	var d models.Design
	if err := database.DB.First(&d, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "design not found")
	}
	if err := deleteDesignRow(&d); err != nil {
		return response.InternalError(c, "failed to delete design")
	}
	return response.OK(c, fiber.Map{"message": "design removed"})
}

// HandleAdminListPurchases godoc
// @Summary     List all design purchases (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       page   query  int  false  "Page number"
// @Param       limit  query  int  false  "Page size"
// @Success     200  {object}  []models.DesignPurchase
// @Failure     401  {object}  map[string]string
// @Router      /market/purchases [get]
func HandleAdminListPurchases(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.DesignPurchase{}).Preload("Buyer").Preload("Seller").Preload("Design")
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}

	var total int64
	q.Count(&total)

	var purchases []models.DesignPurchase
	if err := q.Offset((page - 1) * limit).Limit(limit).Order("created_at DESC").Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}

	for i := range purchases {
		purchases[i].BuyerName = purchases[i].Buyer.Name
		purchases[i].BuyerEmail = purchases[i].Buyer.Email
		purchases[i].SellerName = purchases[i].Seller.Name
		purchases[i].SellerEmail = purchases[i].Seller.Email
		purchases[i].DesignTitle = purchases[i].Design.Title
		// Clear the preloaded relations now that the name/email fields above
		// have been copied out — otherwise the full nested User structs
		// (phone, wallet balance, avatar, status) ride along unnecessarily.
		purchases[i].Buyer = models.User{}
		purchases[i].Seller = models.User{}
	}

	return response.Paginated(c, purchases, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// MarketUserSummary is one row of the admin Sellers or Buyers table — a
// user aggregated across everything they've done in the Design Market,
// whether that's listing designs, buying them, or both.
type MarketUserSummary struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	DesignCount           int64  `json:"design_count"`
	PurchaseCount         int64  `json:"purchase_count"`
	TotalIncomeInPaise    int64  `json:"total_income_in_paise"`
	SellerIncomeInPaise   int64  `json:"seller_income_in_paise"`
	PlatformIncomeInPaise int64  `json:"platform_income_in_paise"`
}

// HandleAdminListMarketUsers godoc
// @Summary     List Design Market participants with earnings (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       search  query  string  false  "Search by name or email"
// @Param       role    query  string  false  "Filter: seller | buyer (default: either)"
// @Param       page    query  int     false  "Page number"
// @Param       limit   query  int     false  "Page size"
// @Success     200  {object}  []MarketUserSummary
// @Failure     401  {object}  map[string]string
// @Router      /market/users [get]
func HandleAdminListMarketUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	search := strings.TrimSpace(c.Query("search"))
	searchClause := ""
	args := []interface{}{}
	if search != "" {
		searchClause = "AND (u.name ILIKE ? OR u.email ILIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// role scopes the list to sellers (uploaded at least one design) or
	// buyers (bought at least one design); default keeps both.
	roleClause := "AND (EXISTS (SELECT 1 FROM designs d WHERE d.seller_id = u.id) OR EXISTS (SELECT 1 FROM design_purchases dp WHERE dp.buyer_id = u.id))"
	switch strings.TrimSpace(c.Query("role")) {
	case "seller":
		roleClause = "AND EXISTS (SELECT 1 FROM designs d WHERE d.seller_id = u.id)"
	case "buyer":
		roleClause = "AND EXISTS (SELECT 1 FROM design_purchases dp WHERE dp.buyer_id = u.id)"
	}

	countSQL := `
		SELECT COUNT(*) FROM users u
		WHERE u.deleted_at IS NULL ` + searchClause + `
		` + roleClause
	var total int64
	if err := database.DB.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return response.InternalError(c, "failed to count market users")
	}

	listSQL := `
		SELECT u.id, u.name, u.email, u.phone,
			COALESCE(dc.design_count, 0)             AS design_count,
			COALESCE(pc.purchase_count, 0)           AS purchase_count,
			COALESCE(s.total_income_in_paise, 0)      AS total_income_in_paise,
			COALESCE(s.seller_income_in_paise, 0)     AS seller_income_in_paise,
			COALESCE(s.platform_income_in_paise, 0)   AS platform_income_in_paise
		FROM users u
		LEFT JOIN (
			SELECT seller_id, COUNT(*) AS design_count
			FROM designs GROUP BY seller_id
		) dc ON dc.seller_id = u.id
		LEFT JOIN (
			SELECT buyer_id, COUNT(*) AS purchase_count
			FROM design_purchases WHERE status = 'SUCCESS' GROUP BY buyer_id
		) pc ON pc.buyer_id = u.id
		LEFT JOIN (
			SELECT seller_id,
				SUM(amount_in_paise)     AS total_income_in_paise,
				SUM(seller_net_in_paise) AS seller_income_in_paise,
				SUM(fee_in_paise)        AS platform_income_in_paise
			FROM design_purchases WHERE status = 'SUCCESS' GROUP BY seller_id
		) s ON s.seller_id = u.id
		WHERE u.deleted_at IS NULL ` + searchClause + `
		` + roleClause + `
		ORDER BY design_count DESC, purchase_count DESC, u.joined_at DESC
		LIMIT ? OFFSET ?`

	var rows []MarketUserSummary
	listArgs := append(append([]interface{}{}, args...), limit, (page-1)*limit)
	if err := database.DB.Raw(listSQL, listArgs...).Scan(&rows).Error; err != nil {
		return response.InternalError(c, "failed to fetch market users")
	}

	return response.Paginated(c, rows, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// MarketUserDesignRow is one design in a seller's "User details" drill-down —
// enough to show what they've listed and what it has earned everyone.
type MarketUserDesignRow struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	PriceInPaise      int64    `json:"price_in_paise"`
	PreviewURLs       []string `json:"preview_urls"`
	SellCount         int64    `json:"sell_count"`
	UserProfitInPaise int64    `json:"user_profit_in_paise"`
	PfProfitInPaise   int64    `json:"pf_profit_in_paise"`
}

// MarketUserPurchaseRow is one purchase in a buyer's "Account detail"
// drill-down — shown even for users who have never listed a design.
type MarketUserPurchaseRow struct {
	ID            string     `json:"id"`
	DesignID      string     `json:"design_id"`
	DesignTitle   string     `json:"design_title"`
	PreviewURL    string     `json:"preview_url,omitempty"`
	AmountInPaise int64      `json:"amount_in_paise"`
	FeeInPaise    int64      `json:"fee_in_paise"`
	SellerName    string     `json:"seller_name"`
	Gateway       string     `json:"gateway"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}

// HandleAdminMarketUserDesigns godoc
// @Summary     One market user's designs listed and designs purchased (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/users/{id}/designs [get]
func HandleAdminMarketUserDesigns(c *fiber.Ctx) error {
	userID := c.Params("id")

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	var designs []models.Design
	if err := database.DB.Where("seller_id = ?", userID).
		Order("created_at DESC").Find(&designs).Error; err != nil {
		return response.InternalError(c, "failed to fetch designs")
	}
	populateDesignURLs(designs, "", nil)

	type earnings struct {
		DesignID   string `json:"design_id"`
		SellCount  int64  `json:"sell_count"`
		UserProfit int64  `json:"user_profit"`
		PfProfit   int64  `json:"pf_profit"`
	}
	var stats []earnings
	if err := database.DB.Raw(`
		SELECT design_id,
			COUNT(*)                     AS sell_count,
			COALESCE(SUM(seller_net_in_paise), 0) AS user_profit,
			COALESCE(SUM(fee_in_paise), 0)        AS pf_profit
		FROM design_purchases
		WHERE seller_id = ? AND status = 'SUCCESS'
		GROUP BY design_id`, userID).Scan(&stats).Error; err != nil {
		return response.InternalError(c, "failed to fetch design earnings")
	}
	statsByDesign := make(map[string]earnings, len(stats))
	for _, s := range stats {
		statsByDesign[s.DesignID] = s
	}

	rows := make([]MarketUserDesignRow, 0, len(designs))
	for _, d := range designs {
		e := statsByDesign[d.ID]
		rows = append(rows, MarketUserDesignRow{
			ID:                d.ID,
			Title:             d.Title,
			PriceInPaise:      d.PriceInPaise,
			PreviewURLs:       d.PreviewURLs,
			SellCount:         e.SellCount,
			UserProfitInPaise: e.UserProfit,
			PfProfitInPaise:   e.PfProfit,
		})
	}

	var purchases []models.DesignPurchase
	if err := database.DB.Preload("Design").Preload("Seller").
		Where("buyer_id = ? AND status = ?", userID, models.PaymentSuccess).
		Order("paid_at DESC").
		Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}

	purchaseRows := make([]MarketUserPurchaseRow, 0, len(purchases))
	for _, p := range purchases {
		row := MarketUserPurchaseRow{
			ID:            p.ID,
			DesignID:      p.DesignID,
			DesignTitle:   p.Design.Title,
			AmountInPaise: p.AmountInPaise,
			FeeInPaise:    p.FeeInPaise,
			SellerName:    p.Seller.Name,
			Gateway:       p.PaidVia,
			PaidAt:        p.PaidAt,
		}
		if len(p.Design.PreviewKeys) > 0 {
			row.PreviewURL = storage.Store.PublicURL(p.Design.PreviewKeys[0])
		}
		purchaseRows = append(purchaseRows, row)
	}

	return response.OK(c, fiber.Map{
		"user": fiber.Map{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"phone": user.Phone,
		},
		"designs_sold": rows,
		"purchases":    purchaseRows,
	})
}
