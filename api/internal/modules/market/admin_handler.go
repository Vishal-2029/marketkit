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

// HandleAdminListProducts godoc
// @Summary     List all marketplace products (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       search  query  string  false  "Search by title or seller"
// @Param       page    query  int     false  "Page number"
// @Param       limit   query  int     false  "Page size"
// @Success     200  {object}  []models.Product
// @Failure     401  {object}  map[string]string
// @Router      /market/products [get]
func HandleAdminListProducts(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.Product{}).Preload("Seller").Preload("Category")
	if s := c.Query("search"); s != "" {
		q = q.Joins("JOIN users ON users.id = products.seller_id").
			Where("products.title ILIKE ? OR users.name ILIKE ? OR users.email ILIKE ?",
				"%"+s+"%", "%"+s+"%", "%"+s+"%")
	}
	if catID := c.Query("category_id"); catID != "" {
		q = q.Where("products.category_id = ?", catID)
	}

	var total int64
	q.Count(&total)

	var products []models.Product
	if err := q.Offset((page - 1) * limit).Limit(limit).Order("products.created_at DESC").Find(&products).Error; err != nil {
		return response.InternalError(c, "failed to fetch products")
	}

	for i := range products {
		products[i].SellerName = products[i].Seller.Name
		products[i].SellerEmail = products[i].Seller.Email
	}
	populateProductURLs(products, "", nil)

	return response.Paginated(c, products, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleAdminDeleteProduct godoc
// @Summary     Take down a marketplace product (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       id  path  string  true  "Product ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/products/{id} [delete]
func HandleAdminDeleteProduct(c *fiber.Ctx) error {
	var d models.Product
	if err := database.DB.First(&d, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "product not found")
	}
	if err := deleteProductRow(&d); err != nil {
		return response.InternalError(c, "failed to delete product")
	}
	return response.OK(c, fiber.Map{"message": "product removed"})
}

// HandleAdminListPurchases godoc
// @Summary     List all product purchases (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       page   query  int  false  "Page number"
// @Param       limit  query  int  false  "Page size"
// @Success     200  {object}  []models.ProductPurchase
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

	q := database.DB.Model(&models.ProductPurchase{}).Preload("Buyer").Preload("Seller").Preload("Product")
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}

	var total int64
	q.Count(&total)

	var purchases []models.ProductPurchase
	if err := q.Offset((page - 1) * limit).Limit(limit).Order("created_at DESC").Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}

	for i := range purchases {
		purchases[i].BuyerName = purchases[i].Buyer.Name
		purchases[i].BuyerEmail = purchases[i].Buyer.Email
		purchases[i].SellerName = purchases[i].Seller.Name
		purchases[i].SellerEmail = purchases[i].Seller.Email
		purchases[i].ProductTitle = purchases[i].Product.Title
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
// user aggregated across everything they've done in the Product Market,
// whether that's listing products, buying them, or both.
type MarketUserSummary struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	ProductCount          int64  `json:"product_count"`
	PurchaseCount         int64  `json:"purchase_count"`
	TotalIncomeInPaise    int64  `json:"total_income_in_paise"`
	SellerIncomeInPaise   int64  `json:"seller_income_in_paise"`
	PlatformIncomeInPaise int64  `json:"platform_income_in_paise"`
}

// HandleAdminListMarketUsers godoc
// @Summary     List Product Market participants with earnings (admin)
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

	// role scopes the list to sellers (uploaded at least one product) or
	// buyers (bought at least one product); default keeps both.
	roleClause := "AND (EXISTS (SELECT 1 FROM products d WHERE d.seller_id = u.id) OR EXISTS (SELECT 1 FROM product_purchases dp WHERE dp.buyer_id = u.id))"
	switch strings.TrimSpace(c.Query("role")) {
	case "seller":
		roleClause = "AND EXISTS (SELECT 1 FROM products d WHERE d.seller_id = u.id)"
	case "buyer":
		roleClause = "AND EXISTS (SELECT 1 FROM product_purchases dp WHERE dp.buyer_id = u.id)"
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
			COALESCE(dc.product_count, 0)             AS product_count,
			COALESCE(pc.purchase_count, 0)           AS purchase_count,
			COALESCE(s.total_income_in_paise, 0)      AS total_income_in_paise,
			COALESCE(s.seller_income_in_paise, 0)     AS seller_income_in_paise,
			COALESCE(s.platform_income_in_paise, 0)   AS platform_income_in_paise
		FROM users u
		LEFT JOIN (
			SELECT seller_id, COUNT(*) AS product_count
			FROM products GROUP BY seller_id
		) dc ON dc.seller_id = u.id
		LEFT JOIN (
			SELECT buyer_id, COUNT(*) AS purchase_count
			FROM product_purchases WHERE status = 'SUCCESS' GROUP BY buyer_id
		) pc ON pc.buyer_id = u.id
		LEFT JOIN (
			SELECT seller_id,
				SUM(amount_in_paise)     AS total_income_in_paise,
				SUM(seller_net_in_paise) AS seller_income_in_paise,
				SUM(fee_in_paise)        AS platform_income_in_paise
			FROM product_purchases WHERE status = 'SUCCESS' GROUP BY seller_id
		) s ON s.seller_id = u.id
		WHERE u.deleted_at IS NULL ` + searchClause + `
		` + roleClause + `
		ORDER BY product_count DESC, purchase_count DESC, u.joined_at DESC
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

// MarketUserProductRow is one product in a seller's "User details" drill-down —
// enough to show what they've listed and what it has earned everyone.
type MarketUserProductRow struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	PriceInPaise      int64    `json:"price_in_paise"`
	PreviewURLs       []string `json:"preview_urls"`
	SellCount         int64    `json:"sell_count"`
	UserProfitInPaise int64    `json:"user_profit_in_paise"`
	PfProfitInPaise   int64    `json:"pf_profit_in_paise"`
}

// MarketUserPurchaseRow is one purchase in a buyer's "Account detail"
// drill-down — shown even for users who have never listed a product.
type MarketUserPurchaseRow struct {
	ID            string     `json:"id"`
	ProductID     string     `json:"product_id"`
	ProductTitle  string     `json:"product_title"`
	PreviewURL    string     `json:"preview_url,omitempty"`
	AmountInPaise int64      `json:"amount_in_paise"`
	FeeInPaise    int64      `json:"fee_in_paise"`
	SellerName    string     `json:"seller_name"`
	Provider      string     `json:"provider"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}

// HandleAdminMarketUserProducts godoc
// @Summary     One market user's products listed and products purchased (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       id  path  string  true  "User ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/users/{id}/products [get]
func HandleAdminMarketUserProducts(c *fiber.Ctx) error {
	userID := c.Params("id")

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	var products []models.Product
	if err := database.DB.Where("seller_id = ?", userID).
		Order("created_at DESC").Find(&products).Error; err != nil {
		return response.InternalError(c, "failed to fetch products")
	}
	populateProductURLs(products, "", nil)

	type earnings struct {
		ProductID  string `json:"product_id"`
		SellCount  int64  `json:"sell_count"`
		UserProfit int64  `json:"user_profit"`
		PfProfit   int64  `json:"pf_profit"`
	}
	var stats []earnings
	if err := database.DB.Raw(`
		SELECT product_id,
			COUNT(*)                     AS sell_count,
			COALESCE(SUM(seller_net_in_paise), 0) AS user_profit,
			COALESCE(SUM(fee_in_paise), 0)        AS pf_profit
		FROM product_purchases
		WHERE seller_id = ? AND status = 'SUCCESS'
		GROUP BY product_id`, userID).Scan(&stats).Error; err != nil {
		return response.InternalError(c, "failed to fetch product earnings")
	}
	statsByProduct := make(map[string]earnings, len(stats))
	for _, s := range stats {
		statsByProduct[s.ProductID] = s
	}

	rows := make([]MarketUserProductRow, 0, len(products))
	for _, d := range products {
		e := statsByProduct[d.ID]
		rows = append(rows, MarketUserProductRow{
			ID:                d.ID,
			Title:             d.Title,
			PriceInPaise:      d.PriceInPaise,
			PreviewURLs:       d.PreviewURLs,
			SellCount:         e.SellCount,
			UserProfitInPaise: e.UserProfit,
			PfProfitInPaise:   e.PfProfit,
		})
	}

	var purchases []models.ProductPurchase
	if err := database.DB.Preload("Product").Preload("Seller").
		Where("buyer_id = ? AND status = ?", userID, models.PaymentSuccess).
		Order("paid_at DESC").
		Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}

	purchaseRows := make([]MarketUserPurchaseRow, 0, len(purchases))
	for _, p := range purchases {
		row := MarketUserPurchaseRow{
			ID:            p.ID,
			ProductID:     p.ProductID,
			ProductTitle:  p.Product.Title,
			AmountInPaise: p.AmountInPaise,
			FeeInPaise:    p.FeeInPaise,
			SellerName:    p.Seller.Name,
			Provider:      p.PaidVia,
			PaidAt:        p.PaidAt,
		}
		if len(p.Product.PreviewKeys) > 0 {
			row.PreviewURL = storage.Store.PublicURL(p.Product.PreviewKeys[0])
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
		"products_sold": rows,
		"purchases":     purchaseRows,
	})
}
