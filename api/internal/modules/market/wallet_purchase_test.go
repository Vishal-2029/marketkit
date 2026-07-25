package market

import (
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPurchaseWithWallet_CreditsOnlyFee mirrors
// TestCapturePurchase_CreditsSellerAndPlatformFeeExactlyOnce but paid from
// the buyer's internal wallet — the platform wallet must still gain only the
// fee, not the gross, since the buyer's wallet balance was already real
// platform-held money.
func TestPurchaseWithWallet_CreditsOnlyFee(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx)
		require.NoError(t, tx.Model(&buyer).Update("wallet_balance_in_paise", int64(200000)).Error)
		design := testutil.MustCreateDesign(t, tx, seller.ID, 100000)

		_, err := purchaseWithWallet(&design, buyer.ID)
		require.NoError(t, err)

		var buyerAfter, sellerAfter models.User
		require.NoError(t, tx.Select("wallet_balance_in_paise").First(&buyerAfter, "id = ?", buyer.ID).Error)
		require.NoError(t, tx.Select("wallet_balance_in_paise").First(&sellerAfter, "id = ?", seller.ID).Error)
		assert.Equal(t, int64(200000-100000), buyerAfter.WalletBalanceInPaise)
		assert.Equal(t, int64(90000), sellerAfter.WalletBalanceInPaise)

		var platformFeeTotal int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ?", models.PlatformSourcePlatformFee).
			Select("COALESCE(SUM(amount_in_paise), 0)").Scan(&platformFeeTotal).Error)
		assert.Equal(t, int64(10000), platformFeeTotal, "only the fee is new platform revenue, never the gross")
	})
}
