package user_payments

import (
	"testing"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.RunMain(m) }

// TestCaptureVerifiedPayment_CreditsLearningPlanExactlyOnce covers the
// user-side Razorpay verify flip point — the fourth and last place a
// learning payment can flip to SUCCESS (alongside HandleManual,
// HandleActivate, and the webhook).
func TestCaptureVerifiedPayment_CreditsLearningPlanExactlyOnce(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		plan := testutil.MustCreatePlan(t, tx, 149900)
		user := testutil.MustCreateUser(t, tx)
		payment := testutil.MustCreatePayment(t, tx, user.ID, plan.ID, plan.PriceMinor, models.PaymentPending)
		payment.Plan = plan
		expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)

		captured, err := captureVerifiedPayment(&payment, "pay_test1", time.Now(), expiresAt)
		require.NoError(t, err)
		assert.True(t, captured)

		captured, err = captureVerifiedPayment(&payment, "pay_test1", time.Now(), expiresAt)
		require.NoError(t, err)
		assert.False(t, captured, "a second capture on an already-SUCCESS payment must be a no-op")

		var total int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Where("source = ? AND reference_id = ?", models.PlatformSourceLearningPlan, payment.ID).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&total).Error)
		assert.Equal(t, plan.PriceMinor, total, "must credit exactly once across both calls")

		var sub models.Subscription
		require.NoError(t, tx.Where("user_id = ? AND plan_id = ?", user.ID, plan.ID).First(&sub).Error)
		assert.Equal(t, models.SubscriptionActive, sub.Status)
	})
}
