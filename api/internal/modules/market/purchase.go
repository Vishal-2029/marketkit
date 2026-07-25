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
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandleCreatePurchaseOrder godoc
// @Summary     Create a Razorpay order for a design purchase
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "design_id"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/order [post]
func HandleCreatePurchaseOrder(c *fiber.Ctx) error {
	var body struct {
		DesignID string `json:"design_id"`
	}
	if err := c.BodyParser(&body); err != nil || body.DesignID == "" {
		return response.BadRequest(c, "design_id is required")
	}

	userID, _ := c.Locals("userID").(string)

	var design models.Design
	if err := database.DB.First(&design, "id = ? AND is_active = true", body.DesignID).Error; err != nil {
		return response.NotFound(c, "design not found")
	}
	if design.SellerID == userID {
		return response.BadRequest(c, "you own this design")
	}

	var existing int64
	database.DB.Model(&models.DesignPurchase{}).
		Where("buyer_id = ? AND design_id = ? AND status = ?", userID, design.ID, models.PaymentSuccess).
		Count(&existing)
	if existing > 0 {
		return response.BadRequest(c, "you already purchased this design")
	}

	if config.App.RazorpayKeyID == "" || config.App.RazorpayKeySecret == "" ||
		config.App.RazorpayKeyID == "rzp_test_xxxx" || config.App.RazorpayKeySecret == "xxxx" {
		return response.InternalError(c, "razorpay is not configured on the server")
	}

	rzpOrderID, err := createRazorpayOrder(design.PriceInPaise, userID, design.ID)
	if err != nil {
		slog.Error("market: razorpay order creation failed", "error", err, "user_id", userID, "design_id", design.ID)
		return response.InternalError(c, "failed to create payment order")
	}

	purchase := models.DesignPurchase{
		DesignID:        design.ID,
		BuyerID:         userID,
		SellerID:        design.SellerID,
		AmountInPaise:   design.PriceInPaise,
		Status:          models.PaymentPending,
		RazorpayOrderID: &rzpOrderID,
	}
	database.DB.Create(&purchase)

	// Same response shape as the plans order endpoint so the app checkout code is shared.
	return response.OK(c, fiber.Map{
		"order_id": rzpOrderID,
		"amount":   design.PriceInPaise,
		"currency": "INR",
		"key_id":   config.App.RazorpayKeyID,
	})
}

// HandleVerifyPurchase godoc
// @Summary     Verify a Razorpay design purchase signature
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "razorpay_order_id, razorpay_payment_id, razorpay_signature"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/verify [post]
func HandleVerifyPurchase(c *fiber.Ctx) error {
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

	// Verify signature: HMAC_SHA256(order_id|payment_id, key_secret)
	msg := body.RazorpayOrderID + "|" + body.RazorpayPaymentID
	mac := hmac.New(sha256.New, []byte(config.App.RazorpayKeySecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(body.RazorpaySignature), []byte(expected)) {
		return response.BadRequest(c, "invalid razorpay signature")
	}

	var p models.DesignPurchase
	if err := database.DB.
		Where("buyer_id = ? AND razorpay_order_id = ?", userID, body.RazorpayOrderID).
		First(&p).Error; err != nil {
		return response.NotFound(c, "purchase not found")
	}
	if p.Status == models.PaymentSuccess {
		return response.OK(c, fiber.Map{"message": "purchase already verified", "purchase_id": p.ID})
	}

	if !capturePurchase(&p, body.RazorpayPaymentID, nil) {
		return response.OK(c, fiber.Map{"message": "purchase already verified", "purchase_id": p.ID})
	}

	go notifySeller(p.SellerID, p.DesignID, p.AmountInPaise)
	sendPurchaseEmailAsync(p.ID)

	return response.OK(c, fiber.Map{"message": "purchase verified", "purchase_id": p.ID})
}

// capturePurchase atomically flips a purchase to SUCCESS, increments the
// design's sales counter, and credits the seller's wallet with the sale
// amount net of the platform fee. The status != SUCCESS guard makes it
// idempotent across the verify endpoint and the webhook racing each other —
// only the caller that flips the row reaches the wallet credit, and both live
// in one transaction, so the credit is exactly-once. Returns true only for
// the request that actually flipped the row.
func capturePurchase(p *models.DesignPurchase, razorpayPaymentID string, gatewayResponse models.JSONMap) bool {
	// Read the fee outside the transaction to keep the locked section short;
	// the snapshot on the purchase row is what makes it durable.
	pct := sellerFeePercent(p.SellerID)
	fee, net := wallet.SplitFee(p.AmountInPaise, pct)

	captured := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":              models.PaymentSuccess,
			"razorpay_payment_id": razorpayPaymentID,
			"paid_at":             time.Now(),
			"fee_in_paise":        fee,
			"seller_net_in_paise": net,
			"paid_via":            "RAZORPAY",
		}
		if gatewayResponse != nil {
			updates["gateway_response"] = gatewayResponse
		}
		result := tx.Model(&models.DesignPurchase{}).
			Where("id = ? AND status != ?", p.ID, models.PaymentSuccess).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&models.Design{}).
			Where("id = ?", p.DesignID).
			UpdateColumn("sales_count", gorm.Expr("sales_count + 1")).Error; err != nil {
			return err
		}
		if _, err := wallet.Apply(tx, p.SellerID, models.WalletTxSaleCredit, net, &p.ID,
			models.JSONMap{"fee_in_paise": fee, "fee_percent": pct, "design_id": p.DesignID}); err != nil {
			return err
		}
		if fee > 0 {
			if _, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, fee, &p.ID,
				models.JSONMap{"fee_percent": pct, "design_id": p.DesignID, "seller_id": p.SellerID}); err != nil {
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
// design_purchases table. Called by the payments webhook handler when no
// subscription payment matches the order ID. Returns true if a marketplace
// purchase was captured.
func CaptureRazorpayOrder(orderID, razorpayPaymentID string, entity models.JSONMap) bool {
	var p models.DesignPurchase
	if err := database.DB.Where("razorpay_order_id = ?", orderID).First(&p).Error; err != nil {
		return false
	}
	if p.Status == models.PaymentSuccess {
		return true
	}
	if !capturePurchase(&p, razorpayPaymentID, entity) {
		return true
	}
	go notifySeller(p.SellerID, p.DesignID, p.AmountInPaise)
	sendPurchaseEmailAsync(p.ID)
	return true
}

func notifySeller(sellerID, designID string, amountInPaise int64) {
	var d models.Design
	title := "your design"
	if err := database.DB.Select("title").First(&d, "id = ?", designID).Error; err == nil {
		title = "'" + d.Title + "'"
	}
	_, net := wallet.SplitFee(amountInPaise, sellerFeePercent(sellerID))
	msg := fmt.Sprintf("Your design %s just sold! ₹%d has been added to your wallet.", title, net/100)
	if err := fcm.SendToUser(sellerID, "Design Sold", msg); err != nil {
		slog.Error("market: seller sale notification failed", "seller_id", sellerID, "error", err)
	}
}

// HandleDownloadURL godoc
// @Summary     Get a signed download URL for a purchased design file
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/designs/{id}/download-url [get]
func HandleDownloadURL(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Design
	if err := database.DB.First(&d, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "design not found")
	}

	if d.SellerID != userID {
		var purchased int64
		database.DB.Model(&models.DesignPurchase{}).
			Where("buyer_id = ? AND design_id = ? AND status = ?", userID, d.ID, models.PaymentSuccess).
			Count(&purchased)
		if purchased == 0 {
			return response.Forbidden(c, "purchase this design to download it")
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
// @Summary     List own successful design purchases
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.DesignPurchase
// @Failure     401  {object}  map[string]string
// @Router      /user/market/my/purchases [get]
func HandleMyPurchases(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var purchases []models.DesignPurchase
	if err := database.DB.Preload("Design").
		Where("buyer_id = ? AND status = ?", userID, models.PaymentSuccess).
		Order("paid_at DESC").
		Find(&purchases).Error; err != nil {
		return response.InternalError(c, "failed to fetch purchases")
	}
	if purchases == nil {
		purchases = []models.DesignPurchase{}
	}
	for i := range purchases {
		designs := []models.Design{purchases[i].Design}
		attachSellerNames(designs)
		populateDesignURLs(designs, userID, map[string]bool{purchases[i].DesignID: true})
		purchases[i].Design = designs[0]
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
		TotalEarnedInPaise int64 `json:"total_earned_in_paise"`
		TotalSales         int64 `json:"total_sales"`
	}
	// Rows from before the wallet era have seller_net_in_paise = 0 and were
	// paid out gross — count those at amount_in_paise, new rows at net.
	netExpr := "CASE WHEN seller_net_in_paise > 0 THEN seller_net_in_paise ELSE amount_in_paise END"
	database.DB.Model(&models.DesignPurchase{}).
		Where("seller_id = ? AND status = ?", userID, models.PaymentSuccess).
		Select("COALESCE(SUM(" + netExpr + "), 0) AS total_earned_in_paise, COUNT(*) AS total_sales").
		Scan(&summary)

	type item struct {
		DesignID      string `json:"design_id"`
		Title         string `json:"title"`
		Sales         int64  `json:"sales"`
		EarnedInPaise int64  `json:"earned_in_paise"`
	}
	var items []item
	database.DB.Model(&models.DesignPurchase{}).
		Joins("JOIN designs ON designs.id = design_purchases.design_id").
		Where("design_purchases.seller_id = ? AND design_purchases.status = ?", userID, models.PaymentSuccess).
		Select("design_purchases.design_id, designs.title, COUNT(*) AS sales, SUM(CASE WHEN design_purchases.seller_net_in_paise > 0 THEN design_purchases.seller_net_in_paise ELSE design_purchases.amount_in_paise END) AS earned_in_paise").
		Group("design_purchases.design_id, designs.title").
		Order("earned_in_paise DESC").
		Scan(&items)
	if items == nil {
		items = []item{}
	}

	return response.OK(c, fiber.Map{
		"total_earned_in_paise": summary.TotalEarnedInPaise,
		"total_sales":           summary.TotalSales,
		"items":                 items,
	})
}

// createRazorpayOrder creates an order via the Razorpay REST API (no SDK),
// mirroring the user_payments module's helper.
func createRazorpayOrder(amountInPaise int64, buyerID, designID string) (string, error) {
	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("mkt_%s_%s", buyerID[:8], designID[:8]),
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
