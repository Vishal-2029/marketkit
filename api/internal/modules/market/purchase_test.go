package market

import (
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.RunMain(m) }

// TestCapturePurchase_CreditsSellerAndPlatformFeeExactlyOnce exercises the
// Razorpay capture path shared by HandleVerifyPurchase and the webhook. The
// platform's cut on a design sale is exactly the fee (10% by default),
// credited in the same transaction as the seller's net — calling it twice
// (simulating verify racing the webhook) must only credit once.
func TestCapturePurchase_CreditsSellerAndPlatformFeeExactlyOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx)
		design := testutil.MustCreateDesign(t, tx, seller.ID, 100000) // ₹1000

		purchase := models.DesignPurchase{
			DesignID:      design.ID,
			BuyerID:       buyer.ID,
			SellerID:      seller.ID,
			AmountInPaise: design.PriceInPaise,
			Status:        models.PaymentPending,
		}
		require.NoError(t, tx.Create(&purchase).Error)

		ok := capturePurchase(&purchase, "pay_test1", nil)
		assert.True(t, ok, "first capture must succeed")

		// Calling again (webhook racing verify, or a Razorpay retry) must be a
		// no-op — capturePurchase's own status != SUCCESS guard should catch it.
		ok = capturePurchase(&purchase, "pay_test1", nil)
		assert.False(t, ok, "second capture on an already-SUCCESS purchase must not re-flip")

		var updated models.DesignPurchase
		require.NoError(t, tx.First(&updated, "id = ?", purchase.ID).Error)
		assert.Equal(t, models.PaymentSuccess, updated.Status)
		assert.Equal(t, int64(10000), updated.FeeInPaise, "default platform fee is 10%%")
		assert.Equal(t, int64(90000), updated.SellerNetInPaise)

		var sellerAfter models.User
		require.NoError(t, tx.Select("wallet_balance_in_paise").First(&sellerAfter, "id = ?", seller.ID).Error)
		assert.Equal(t, int64(90000), sellerAfter.WalletBalanceInPaise, "seller wallet must be credited exactly once")

		var d models.Design
		require.NoError(t, tx.First(&d, "id = ?", design.ID).Error)
		assert.Equal(t, 1, d.SalesCount, "sales_count must increment exactly once")

		var platformFeeTotal int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ?", models.PlatformSourcePlatformFee).
			Select("COALESCE(SUM(amount_in_paise), 0)").Scan(&platformFeeTotal).Error)
		assert.Equal(t, int64(10000), platformFeeTotal, "platform wallet must be credited the fee exactly once, not the gross")

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(10000), w.BalanceInPaise)
	})
}
