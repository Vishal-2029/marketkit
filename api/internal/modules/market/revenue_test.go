package market

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleMarketRevenueSummary_ReconcilesWithPlatformWalletAndExcludesLearning
// covers Task 4's acceptance criteria directly: the market revenue numbers
// must equal the platform wallet's MARKET_PLAN + PLATFORM_FEE sources, and a
// learning payment must never affect them.
func TestHandleMarketRevenueSummary_ReconcilesWithPlatformWalletAndExcludesLearning(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx)
		product := testutil.MustCreateProduct(t, tx, seller.ID, 100000)
		purchase := testutil.MustCreateProductPurchase(t, tx, product.ID, buyer.ID, seller.ID, 100000, 10000, models.PaymentSuccess)
		_, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, purchase.FeeInPaise, &purchase.ID, nil)
		require.NoError(t, err)

		marketPlan := testutil.MustCreateMarketPlan(t, tx, 49900)
		sub := testutil.MustCreateMarketPlanSub(t, tx, buyer.ID, marketPlan.ID, 49900, true)
		_, err = platform_wallet.Apply(tx, models.PlatformSourceMarketPlan, sub.AmountInPaise, &sub.ID, nil)
		require.NoError(t, err)

		// A learning payment must never leak into these numbers.
		plan := testutil.MustCreatePlan(t, tx, 999900)
		testutil.MustCreatePayment(t, tx, buyer.ID, plan.ID, 999900, models.PaymentSuccess)

		app := testutil.FiberApp(nil)
		app.Get("/x", HandleMarketRevenueSummary)
		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data struct {
				PlatformRevenuePaise int64 `json:"platform_revenue_paise"`
				PlanRevenuePaise     int64 `json:"plan_revenue_paise"`
				FeeRevenuePaise      int64 `json:"fee_revenue_paise"`
				GrossSalesPaise      int64 `json:"gross_sales_paise"`
				SellerPayoutsPaise   int64 `json:"seller_payouts_paise"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))

		assert.Equal(t, int64(49900), parsed.Data.PlanRevenuePaise)
		assert.Equal(t, int64(10000), parsed.Data.FeeRevenuePaise)
		assert.Equal(t, int64(59900), parsed.Data.PlatformRevenuePaise, "must equal plan + fee, matching the platform wallet's two market-side sources")
		assert.Equal(t, int64(100000), parsed.Data.GrossSalesPaise)
		assert.Equal(t, int64(90000), parsed.Data.SellerPayoutsPaise)

		// The learning Payment row above was created directly (bypassing the
		// payments handlers), so nothing credited the platform wallet for it —
		// its balance must equal exactly the two market-side sources.
		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, parsed.Data.PlatformRevenuePaise, w.BalanceInPaise)
	})
}
