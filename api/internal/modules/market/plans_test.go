package market

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActivateMarketPlanSub_CreditsPlatformWalletExactlyOnce covers the
// Razorpay path (also used by the webhook via CaptureMarketPlanOrder): a
// market-plan payment is platform revenue in full, not just a fee.
func TestActivateMarketPlanSub_CreditsPlatformWalletExactlyOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		user := testutil.MustCreateUser(t, tx)
		plan := testutil.MustCreateMarketPlan(t, tx, 49900)

		sub := models.MarketPlanSubscription{
			UserID:      user.ID,
			PlanID:      plan.ID,
			Status:      models.SubscriptionPending,
			StartDate:   time.Now(),
			ExpiryDate:  time.Now(),
			AmountMinor: plan.PriceMinor,
		}
		require.NoError(t, tx.Create(&sub).Error)
		sub.Plan = plan // activateMarketPlanSub reads sub.Plan.DurationDays

		ok := activateMarketPlanSub(&sub, "pay_test1")
		assert.True(t, ok)

		ok = activateMarketPlanSub(&sub, "pay_test1")
		assert.False(t, ok, "activating an already-ACTIVE subscription must be a no-op")

		var total int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ? AND reference_id = ?", models.PlatformSourceMarketPlan, sub.ID).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&total).Error)
		assert.Equal(t, plan.PriceMinor, total, "must credit the full plan price exactly once")
	})
}

// TestHandleSubscribeMarketPlanWithWallet_CreditsFullAmount covers the
// wallet-funded path: money in the buyer's internal wallet was already real
// money the platform held, so spending it on a plan realizes the full amount
// as platform revenue (unlike a product sale, where only the fee is new
// revenue — the net still belongs to the seller).
func TestHandleSubscribeMarketPlanWithWallet_CreditsFullAmount(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		user := testutil.MustCreateUser(t, tx)
		require.NoError(t, tx.Model(&user).Update("wallet_balance_minor", int64(100000)).Error)
		plan := testutil.MustCreateMarketPlan(t, tx, 49900)

		app := testutil.FiberApp(map[string]string{"userID": user.ID})
		app.Post("/plans/:id/wallet", HandleSubscribeMarketPlanWithWallet)

		req := httptest.NewRequest("POST", "/plans/"+plan.ID+"/wallet", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var buyerAfter models.User
		require.NoError(t, tx.Select("wallet_balance_minor").First(&buyerAfter, "id = ?", user.ID).Error)
		assert.Equal(t, int64(100000-49900), buyerAfter.WalletBalanceMinor)

		var platformTotal int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ?", models.PlatformSourceMarketPlan).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&platformTotal).Error)
		assert.Equal(t, plan.PriceMinor, platformTotal, "platform wallet gets the full plan price, not just a cut")
	})
}
