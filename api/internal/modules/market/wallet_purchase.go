package market

import (
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
// @Summary     Buy a design instantly using wallet balance
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "design_id"
// @Success     200  {object}  models.DesignPurchase
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/wallet [post]
func HandlePurchaseWithWallet(c *fiber.Ctx) error {
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

	purchase, err := purchaseWithWallet(&design, userID)
	if err == wallet.ErrInsufficientBalance {
		return response.BadRequest(c, "insufficient wallet balance")
	}
	if err != nil {
		return response.InternalErrorWithLog(c, "market: wallet purchase", err)
	}

	go notifySeller(design.SellerID, design.ID, design.PriceInPaise)
	sendPurchaseEmailAsync(purchase.ID)

	return response.OK(c, purchase)
}

// purchaseWithWallet is the money-moving core of HandlePurchaseWithWallet,
// split out so it can be exercised directly in tests without also triggering
// the handler's fire-and-forget notification/email goroutines.
func purchaseWithWallet(design *models.Design, userID string) (models.DesignPurchase, error) {
	pct := sellerFeePercent(design.SellerID)
	fee, net := wallet.SplitFee(design.PriceInPaise, pct)
	now := time.Now()

	purchase := models.DesignPurchase{
		DesignID:         design.ID,
		BuyerID:          userID,
		SellerID:         design.SellerID,
		AmountInPaise:    design.PriceInPaise,
		Status:           models.PaymentSuccess,
		FeeInPaise:       fee,
		SellerNetInPaise: net,
		PaidVia:          "WALLET",
		PaidAt:           &now,
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}
		// Fixed lock order everywhere: debit the buyer before crediting the
		// seller (see wallet.Apply).
		if _, err := wallet.Apply(tx, userID, models.WalletTxPurchaseDebit, -design.PriceInPaise,
			&purchase.ID, models.JSONMap{"design_id": design.ID}); err != nil {
			return err
		}
		if err := tx.Model(&models.Design{}).
			Where("id = ?", design.ID).
			UpdateColumn("sales_count", gorm.Expr("sales_count + 1")).Error; err != nil {
			return err
		}
		if _, err := wallet.Apply(tx, design.SellerID, models.WalletTxSaleCredit, net,
			&purchase.ID, models.JSONMap{"fee_in_paise": fee, "fee_percent": pct, "design_id": design.ID}); err != nil {
			return err
		}
		if fee > 0 {
			if _, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, fee, &purchase.ID,
				models.JSONMap{"fee_percent": pct, "design_id": design.ID, "seller_id": design.SellerID}); err != nil {
				return err
			}
		}
		return nil
	})
	return purchase, err
}
