package market

import (
	"sync"
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A buyer who double-clicks — or whose client retries — must not be charged
// twice for the same product. The handler's "already purchased?" check is a
// plain COUNT outside the transaction, so on its own it loses the race: both
// requests read zero, both charge. The database has to be the arbiter.
//
// Not run inside WithTx: concurrency needs real, separate transactions.
func TestPurchaseWithWallet_ConcurrentDoubleBuyChargesOnce(t *testing.T) {
	seller := testutil.MustCreateUser(t, database.DB)
	buyer := testutil.MustCreateUser(t, database.DB)
	product := testutil.MustCreateProduct(t, database.DB, seller.ID, 5000)

	// This test cannot use WithTx — concurrency needs real, separate
	// transactions — so it must undo its own writes. The platform wallet is a
	// shared singleton: leaving fee credits behind breaks every other test that
	// asserts on its balance.
	var platformBefore int64
	database.DB.Model(&models.PlatformWallet{}).
		Where("id = ?", models.PlatformWalletSingletonID).
		Select("balance_minor").Scan(&platformBefore)

	t.Cleanup(func() {
		var ids []string
		database.DB.Model(&models.ProductPurchase{}).
			Where("product_id = ?", product.ID).Pluck("id", &ids)
		if len(ids) > 0 {
			database.DB.Where("reference_id IN ?", ids).Delete(&models.PlatformLedger{})
		}
		database.DB.Model(&models.PlatformWallet{}).
			Where("id = ?", models.PlatformWalletSingletonID).
			Update("balance_minor", platformBefore)

		database.DB.Where("product_id = ?", product.ID).Delete(&models.ProductPurchase{})
		database.DB.Where("user_id IN ?", []string{buyer.ID, seller.ID}).Delete(&models.WalletTransaction{})
		database.DB.Where("id = ?", product.ID).Delete(&models.Product{})
		database.DB.Unscoped().Where("id IN ?", []string{buyer.ID, seller.ID}).Delete(&models.User{})
	})

	// Fund the buyer for two purchases, so a double charge would actually
	// succeed rather than being stopped by an insufficient balance.
	require.NoError(t, database.DB.Transaction(func(tx *gorm.DB) error {
		_, err := wallet.Apply(tx, buyer.ID, models.WalletTxTopup, 20000, nil, nil)
		return err
	}))

	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // fire both as close together as possible
			_, errs[i] = purchaseWithWallet(&product, buyer.ID)
		}(i)
	}
	close(start)
	wg.Wait()

	var succeeded int
	for _, err := range errs {
		if err == nil {
			succeeded++
		}
	}

	var purchases int64
	database.DB.Model(&models.ProductPurchase{}).
		Where("buyer_id = ? AND product_id = ? AND status = ?", buyer.ID, product.ID, models.PaymentSuccess).
		Count(&purchases)

	var fresh models.User
	require.NoError(t, database.DB.First(&fresh, "id = ?", buyer.ID).Error)

	assert.Equal(t, 1, succeeded, "exactly one of two concurrent purchases may succeed")
	assert.EqualValues(t, 1, purchases, "the buyer must own the product once, not twice")
	assert.EqualValues(t, 15000, fresh.WalletBalanceMinor, "buyer must be debited once (20000 - 5000)")
}
