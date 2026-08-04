package market

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandlePurchaseWithWallet godoc
// @Summary     Buy a product instantly using wallet balance
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "product_id"
// @Success     200  {object}  models.ProductPurchase
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/wallet [post]
func HandlePurchaseWithWallet(c *fiber.Ctx) error {
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

	purchase, err := purchaseWithWallet(&product, userID)
	if err == wallet.ErrInsufficientBalance {
		return response.BadRequest(c, "insufficient wallet balance")
	}
	// The COUNT above cannot see a purchase committed by a concurrent request,
	// so the unique index is what actually settles a double-click. Losing that
	// race is not a server error — the buyer simply already owns it.
	if isDuplicatePurchase(err) {
		return response.BadRequest(c, "you already purchased this product")
	}
	if err != nil {
		return response.InternalErrorWithLog(c, "market: wallet purchase", err)
	}

	go notifySeller(product.SellerID, product.ID, product.PriceMinor)
	sendPurchaseEmailAsync(purchase.ID)

	return response.OK(c, purchase)
}

// purchaseWithWallet is the money-moving core of HandlePurchaseWithWallet,
// split out so it can be exercised directly in tests without also triggering
// the handler's fire-and-forget notification/email goroutines.
// isDuplicatePurchase reports whether err is the unique-index violation from
// idx_product_purchases_one_success_per_buyer.
func isDuplicatePurchase(err error) bool {
	return err != nil && strings.Contains(err.Error(), "idx_product_purchases_one_success_per_buyer")
}

func purchaseWithWallet(product *models.Product, userID string) (models.ProductPurchase, error) {
	pct := sellerFeePercent(product.SellerID)
	fee, net := wallet.SplitFee(product.PriceMinor, pct)
	now := time.Now()

	purchase := models.ProductPurchase{
		ProductID:      product.ID,
		BuyerID:        userID,
		SellerID:       product.SellerID,
		AmountMinor:    product.PriceMinor,
		Status:         models.PaymentSuccess,
		FeeMinor:       fee,
		SellerNetMinor: net,
		PaidVia:        "WALLET",
		PaidAt:         &now,
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}
		// Fixed lock order everywhere: debit the buyer before crediting the
		// seller (see wallet.Apply).
		if _, err := wallet.Apply(tx, userID, models.WalletTxPurchaseDebit, -product.PriceMinor,
			&purchase.ID, models.JSONMap{"product_id": product.ID}); err != nil {
			return err
		}
		if err := tx.Model(&models.Product{}).
			Where("id = ?", product.ID).
			UpdateColumn("sales_count", gorm.Expr("sales_count + 1")).Error; err != nil {
			return err
		}
		if _, err := wallet.Apply(tx, product.SellerID, models.WalletTxSaleCredit, net,
			&purchase.ID, models.JSONMap{"fee_minor": fee, "fee_percent": pct, "product_id": product.ID}); err != nil {
			return err
		}
		if fee > 0 {
			if _, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, fee, &purchase.ID,
				models.JSONMap{"fee_percent": pct, "product_id": product.ID, "seller_id": product.SellerID}); err != nil {
				return err
			}
		}
		return nil
	})
	return purchase, err
}
