package market

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/internal/payments"
	"github.com/marketkit/api/internal/payments/provider"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/money"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandleCreatePurchaseOrder godoc
// @Summary     Create a Razorpay order for a product purchase
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "product_id"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/order [post]
func HandleCreatePurchaseOrder(c *fiber.Ctx) error {
	var body struct {
		ProductID string `json:"product_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.ProductID == "" {
		return response.BadRequest(c, "product_id is required")
	}

	userID, _ := c.Locals("userID").(string)

	var product models.Product
	if err := database.DB.First(&product, "id = ? AND is_active = true", body.ProductID).Error; err != nil {
		return response.NotFound(c, "product not found")
	}
	if product.SellerID == userID {
		return response.BadRequest(c, "you own this product")
	}

	var existing int64
	database.DB.Model(&models.ProductPurchase{}).
		Where("buyer_id = ? AND product_id = ? AND status = ?", userID, product.ID, models.PaymentSuccess).
		Count(&existing)
	if existing > 0 {
		return response.BadRequest(c, "you already purchased this product")
	}

	if config.App.RazorpayKeyID == "" || config.App.RazorpayKeySecret == "" ||
		config.App.RazorpayKeyID == "rzp_test_xxxx" || config.App.RazorpayKeySecret == "xxxx" {
		return response.InternalError(c, "razorpay is not configured on the server")
	}

	order, err := createRazorpayOrder(product.PriceMinor, userID, product.ID)
	if err != nil {
		slog.Error("market: razorpay order creation failed", "error", err, "user_id", userID, "product_id", product.ID)
		return response.InternalError(c, "failed to create payment order")
	}

	purchase := models.ProductPurchase{
		ProductID:       product.ID,
		BuyerID:         userID,
		SellerID:        product.SellerID,
		AmountMinor:     product.PriceMinor,
		Status:          models.PaymentPending,
		ProviderOrderID: &order.ID,
	}
	database.DB.Create(&purchase)

	// Same response shape as the plans order endpoint so the app checkout code is shared.
	return response.OK(c, payments.NewCheckout(order, product.PriceMinor))
}

// HandleVerifyPurchase godoc
// @Summary     Verify a Razorpay product purchase signature
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "provider_order_id, provider_payment_id, provider_signature"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/verify [post]
func HandleVerifyPurchase(c *fiber.Ctx) error {
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

	// Verify signature: HMAC_SHA256(order_id|payment_id, key_secret)
	msg := body.ProviderOrderID + "|" + body.ProviderPaymentID
	mac := hmac.New(sha256.New, []byte(config.App.RazorpayKeySecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(body.ProviderSignature), []byte(expected)) {
		return response.BadRequest(c, "invalid razorpay signature")
	}

	var p models.ProductPurchase
	if err := database.DB.
		Where("buyer_id = ? AND provider_order_id = ?", userID, body.ProviderOrderID).
		First(&p).Error; err != nil {
		return response.NotFound(c, "purchase not found")
	}
	if p.Status == models.PaymentSuccess {
		return response.OK(c, fiber.Map{"message": "purchase already verified", "purchase_id": p.ID})
	}

	if !capturePurchase(&p, body.ProviderPaymentID, nil) {
		return response.OK(c, fiber.Map{"message": "purchase already verified", "purchase_id": p.ID})
	}

	go notifySeller(p.SellerID, p.ProductID, p.AmountMinor)
	sendPurchaseEmailAsync(p.ID)

	return response.OK(c, fiber.Map{"message": "purchase verified", "purchase_id": p.ID})
}

// capturePurchase atomically flips a purchase to SUCCESS, increments the
// product's sales counter, and credits the seller's wallet with the sale
// amount net of the platform fee. The status != SUCCESS guard makes it
// idempotent across the verify endpoint and the webhook racing each other —
// only the caller that flips the row reaches the wallet credit, and both live
// in one transaction, so the credit is exactly-once. Returns true only for
// the request that actually flipped the row.
func capturePurchase(p *models.ProductPurchase, razorpayPaymentID string, gatewayResponse models.JSONMap) bool {
	// Read the fee outside the transaction to keep the locked section short;
	// the snapshot on the purchase row is what makes it durable.
	pct := sellerFeePercent(p.SellerID)
	fee, net := wallet.SplitFee(p.AmountMinor, pct)

	captured := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":              models.PaymentSuccess,
			"provider_payment_id": razorpayPaymentID,
			"paid_at":             time.Now(),
			"fee_minor":           fee,
			"seller_net_minor":    net,
			"paid_via":            "RAZORPAY",
		}
		if gatewayResponse != nil {
			updates["gateway_response"] = gatewayResponse
		}
		result := tx.Model(&models.ProductPurchase{}).
			Where("id = ? AND status != ?", p.ID, models.PaymentSuccess).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&models.Product{}).
			Where("id = ?", p.ProductID).
			UpdateColumn("sales_count", gorm.Expr("sales_count + 1")).Error; err != nil {
			return err
		}
		if _, err := wallet.Apply(tx, p.SellerID, models.WalletTxSaleCredit, net, &p.ID,
			models.JSONMap{"fee_minor": fee, "fee_percent": pct, "product_id": p.ProductID}); err != nil {
			return err
		}
		if fee > 0 {
			if _, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, fee, &p.ID,
				models.JSONMap{"fee_percent": pct, "product_id": p.ProductID, "seller_id": p.SellerID}); err != nil {
				return err
			}
		}
		captured = true
		return nil
	})
	if err != nil {
		slog.Error("market: failed to capture purchase", "purchase_id", p.ID, "error", err)
		return false
	}
	return captured
}

// CaptureRazorpayOrder resolves a webhook payment.captured event against the
// product_purchases table. Called by the payments webhook handler when no
// subscription payment matches the order ID. Returns true if a marketplace
// purchase was captured.
func CaptureOrder(orderID, razorpayPaymentID string, entity models.JSONMap) bool {
	var p models.ProductPurchase
	if err := database.DB.Where("provider_order_id = ?", orderID).First(&p).Error; err != nil {
		return false
	}
	if p.Status == models.PaymentSuccess {
		return true
	}
	if !capturePurchase(&p, razorpayPaymentID, entity) {
		return true
	}
	go notifySeller(p.SellerID, p.ProductID, p.AmountMinor)
	sendPurchaseEmailAsync(p.ID)
	return true
}

func notifySeller(sellerID, productID string, amountMinor int64) {
	var d models.Product
	title := "your product"
	if err := database.DB.Select("title").First(&d, "id = ?", productID).Error; err == nil {
		title = "'" + d.Title + "'"
	}
	_, net := wallet.SplitFee(amountMinor, sellerFeePercent(sellerID))
	msg := fmt.Sprintf("Your product %s just sold! %s has been added to your wallet.",
		title, money.Format(net, config.App.PaymentCurrency))
	if err := fcm.SendToUser(sellerID, "Product Sold", msg); err != nil {
		slog.Error("market: seller sale notification failed", "seller_id", sellerID, "error", err)
	}
}

// HandleDownloadURL godoc
// @Summary     Get a signed download URL for a purchased product file
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Product ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/products/{id}/download-url [get]
func HandleDownloadURL(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Product
	if err := database.DB.First(&d, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "product not found")
	}

	if d.SellerID != userID {
		var purchased int64
		database.DB.Model(&models.ProductPurchase{}).
			Where("buyer_id = ? AND product_id = ? AND status = ?", userID, d.ID, models.PaymentSuccess).
			Count(&purchased)
		if purchased == 0 {
			return response.Forbidden(c, "purchase this product to download it")
		}
	}

	url, err := storage.Store.SignedURL(c.Context(), d.FileKey, 15*time.Minute)
	if err != nil {
		return response.InternalErrorWithLog(c, "market: signed download url", err)
	}
	return response.OK(c, fiber.Map{
		"url":       url,
		"file_name": d.FileName,
	})
}

// HandleMyPurchases godoc
// @Summary     List own successful product purchases
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.ProductPurchase
// @Failure     401  {object}  map[string]string
// @Router      /user/market/my/purchases [get]
func HandleMyPurchases(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var purchases []models.ProductPurchase
	if err := database.DB.Preload("Product").
		Where("buyer_id = ? AND status = ?", userID, models.PaymentSuccess).
		Order("paid_at DESC").
		Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}
	if purchases == nil {
		purchases = []models.ProductPurchase{}
	}
	for i := range purchases {
		products := []models.Product{purchases[i].Product}
		attachSellerNames(products)
		populateProductURLs(products, userID, map[string]bool{purchases[i].ProductID: true})
		purchases[i].Product = products[0]
	}
	return response.OK(c, purchases)
}

// HandleEarnings godoc
// @Summary     Seller earnings summary (net of platform fee; credited to wallet)
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /user/market/my/earnings [get]
func HandleEarnings(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var summary struct {
		TotalEarnedMinor int64 `json:"total_earned_minor"`
		TotalSales       int64 `json:"total_sales"`
	}
	// Rows from before the wallet era have seller_net_minor = 0 and were
	// paid out gross — count those at amount_minor, new rows at net.
	netExpr := "CASE WHEN seller_net_minor > 0 THEN seller_net_minor ELSE amount_minor END"
	database.DB.Model(&models.ProductPurchase{}).
		Where("seller_id = ? AND status = ?", userID, models.PaymentSuccess).
		Select("COALESCE(SUM(" + netExpr + "), 0) AS total_earned_minor, COUNT(*) AS total_sales").
		Scan(&summary)

	type item struct {
		ProductID   string `json:"product_id"`
		Title       string `json:"title"`
		Sales       int64  `json:"sales"`
		EarnedMinor int64  `json:"earned_minor"`
	}
	var items []item
	database.DB.Model(&models.ProductPurchase{}).
		Joins("JOIN products ON products.id = product_purchases.product_id").
		Where("product_purchases.seller_id = ? AND product_purchases.status = ?", userID, models.PaymentSuccess).
		Select("product_purchases.product_id, products.title, COUNT(*) AS sales, SUM(CASE WHEN product_purchases.seller_net_minor > 0 THEN product_purchases.seller_net_minor ELSE product_purchases.amount_minor END) AS earned_minor").
		Group("product_purchases.product_id, products.title").
		Order("earned_minor DESC").
		Scan(&items)
	if items == nil {
		items = []item{}
	}

	return response.OK(c, fiber.Map{
		"total_earned_minor": summary.TotalEarnedMinor,
		"total_sales":        summary.TotalSales,
		"items":              items,
	})
}

// createRazorpayOrder creates an order via the Razorpay REST API (no SDK),
// mirroring the user_payments module's helper.
func createRazorpayOrder(amountMinor int64, buyerID, productID string) (provider.Order, error) {
	order, err := payments.CreateOrder(context.Background(), amountMinor,
		payments.Receipt("mkt", buyerID, productID),
		map[string]string{"buyer_id": buyerID, "product_id": productID})
	if err != nil {
		return provider.Order{}, err
	}
	return order, nil
}
