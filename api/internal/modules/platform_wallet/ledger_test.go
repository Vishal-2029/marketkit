package platform_wallet

import (
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.RunMain(m) }

func TestApply_CreditThenDebit(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		balance, err := Apply(tx, models.PlatformSourceLearningPlan, 50000, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(50000), balance)

		var ledger1 models.PlatformLedger
		require.NoError(t, tx.Where("source = ?", models.PlatformSourceLearningPlan).First(&ledger1).Error)
		assert.Equal(t, "CREDIT", ledger1.Type)
		assert.Equal(t, int64(50000), ledger1.AmountInPaise)
		assert.Equal(t, int64(50000), ledger1.BalanceAfterInPaise)

		balance, err = Apply(tx, models.PlatformSourceWithdrawal, -20000, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(30000), balance)

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(30000), w.BalanceInPaise)

		var sum int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Select("COALESCE(SUM(amount_in_paise), 0)").Scan(&sum).Error)
		assert.Equal(t, w.BalanceInPaise, sum, "balance must always equal SUM(ledger.amount_in_paise)")
	})
}

func TestApply_RejectsOverdraft(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		_, err := Apply(tx, models.PlatformSourceLearningPlan, 10000, nil, nil)
		require.NoError(t, err)

		_, err = Apply(tx, models.PlatformSourceWithdrawal, -10001, nil, nil)
		assert.ErrorIs(t, err, ErrInsufficientBalance)

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(10000), w.BalanceInPaise, "a rejected debit must not change the balance")
	})
}

func TestBackfill_SumsHistoricalRowsAndIsIdempotent(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		user := testutil.MustCreateUser(t, tx)
		plan := testutil.MustCreatePlan(t, tx, 99900)
		testutil.MustCreatePayment(t, tx, user.ID, plan.ID, 99900, models.PaymentSuccess)
		// A FAILED payment must never be counted.
		testutil.MustCreatePayment(t, tx, user.ID, plan.ID, 99900, models.PaymentFailed)

		marketPlan := testutil.MustCreateMarketPlan(t, tx, 49900)
		testutil.MustCreateMarketPlanSub(t, tx, user.ID, marketPlan.ID, 49900, true)
		// An unpaid (pending) subscription must never be counted.
		testutil.MustCreateMarketPlanSub(t, tx, user.ID, marketPlan.ID, 49900, false)

		design := testutil.MustCreateDesign(t, tx, user.ID, 20000)
		testutil.MustCreateDesignPurchase(t, tx, design.ID, user.ID, user.ID, 20000, 2000, models.PaymentSuccess)

		// Backfill reads database.DB directly (not the tx param), which
		// testutil.WithTx has already swapped to this same transaction.
		Backfill()

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(99900+49900+2000), w.BalanceInPaise)
		require.NotNil(t, w.BackfilledAt)

		// Re-running must be a no-op.
		Backfill()
		var w2 models.PlatformWallet
		require.NoError(t, tx.First(&w2, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, w.BalanceInPaise, w2.BalanceInPaise, "re-running backfill must not double-credit")

		var ledgerCount int64
		tx.Model(&models.PlatformLedger{}).Count(&ledgerCount)
		assert.Equal(t, int64(3), ledgerCount, "exactly one ledger row per source, even after a second run")
	})
}
