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
// platform's cut on a product sale is exactly the fee (10% by default),
// credited in the same transaction as the seller's net — calling it twice
// (simulating verify racing the webhook) must only credit once.
func TestCapturePurchase_CreditsSellerAndPlatformFeeExactlyOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx)
		product := testutil.MustCreateProduct(t, tx, seller.ID, 100000) // ₹1000

		purchase := models.ProductPurchase{
			ProductID:   product.ID,
			BuyerID:     buyer.ID,
			SellerID:    seller.ID,
			AmountMinor: product.PriceMinor,
			Status:      models.PaymentPending,
		}
		require.NoError(t, tx.Create(&purchase).Error)

		ok := capturePurchase(&purchase, "pay_test1", nil)
		assert.True(t, ok, "first capture must succeed")

		// Calling again (webhook racing verify, or a Razorpay retry) must be a
		// no-op — capturePurchase's own status != SUCCESS guard should catch it.
		ok = capturePurchase(&purchase, "pay_test1", nil)
		assert.False(t, ok, "second capture on an already-SUCCESS purchase must not re-flip")

		var updated models.ProductPurchase
		require.NoError(t, tx.First(&updated, "id = ?", purchase.ID).Error)
		assert.Equal(t, models.PaymentSuccess, updated.Status)
		assert.Equal(t, int64(10000), updated.FeeMinor, "default platform fee is 10%%")
		assert.Equal(t, int64(90000), updated.SellerNetMinor)

		var sellerAfter models.User
		require.NoError(t, tx.Select("wallet_balance_minor").First(&sellerAfter, "id = ?", seller.ID).Error)
		assert.Equal(t, int64(90000), sellerAfter.WalletBalanceMinor, "seller wallet must be credited exactly once")

		var d models.Product
		require.NoError(t, tx.First(&d, "id = ?", product.ID).Error)
		assert.Equal(t, 1, d.SalesCount, "sales_count must increment exactly once")

		var platformFeeTotal int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ?", models.PlatformSourcePlatformFee).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&platformFeeTotal).Error)
		assert.Equal(t, int64(10000), platformFeeTotal, "platform wallet must be credited the fee exactly once, not the gross")

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(10000), w.BalanceMinor)
	})
}
