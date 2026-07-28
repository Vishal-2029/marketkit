package payments

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

func TestMain(m *testing.M) { testutil.RunMain(m) }

// TestCreateManualPayment_CreditsLearningPlanExactlyOnce covers the
// admin-manual-activation flip point (HandleManual). Learning-plan payments
// are platform revenue in full, unlike a product sale's fee-only cut.
func TestCreateManualPayment_CreditsLearningPlanExactlyOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		plan := testutil.MustCreatePlan(t, tx, 99900)
		user := testutil.MustCreateUser(t, tx)
		expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)

		payment, err := createManualPayment(&plan, user.ID, "manual test", expiresAt)
		require.NoError(t, err)
		assert.Equal(t, models.PaymentSuccess, payment.Status)

		var total int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ? AND reference_id = ?", models.PlatformSourceLearningPlan, payment.ID).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&total).Error)
		assert.Equal(t, plan.PriceMinor, total)

		var sub models.Subscription
		require.NoError(t, tx.Where("user_id = ? AND plan_id = ?", user.ID, plan.ID).First(&sub).Error)
		assert.Equal(t, models.SubscriptionActive, sub.Status)
	})
}

// TestHandleActivate_IdempotentAndCreditsOnce covers the admin
// activate-a-pending-payment flip point, including the idempotency guard
// this endpoint was missing before (calling it twice must credit once).
func TestHandleActivate_IdempotentAndCreditsOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		plan := testutil.MustCreatePlan(t, tx, 49900)
		user := testutil.MustCreateUser(t, tx)
		payment := testutil.MustCreatePayment(t, tx, user.ID, plan.ID, plan.PriceMinor, models.PaymentPending)

		adminID := testutil.MustCreateAdmin(t, tx, false).ID
		app := testutil.FiberApp(map[string]string{"adminID": adminID})
		app.Post("/x/:id/activate", HandleActivate)

		req := httptest.NewRequest("POST", "/x/"+payment.ID+"/activate", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		req2 := httptest.NewRequest("POST", "/x/"+payment.ID+"/activate", nil)
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		assert.Equal(t, 200, resp2.StatusCode, "second activate call must be a no-op 200, not an error")

		var total int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ? AND reference_id = ?", models.PlatformSourceLearningPlan, payment.ID).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&total).Error)
		assert.Equal(t, plan.PriceMinor, total, "must credit exactly once across both calls")
	})
}
