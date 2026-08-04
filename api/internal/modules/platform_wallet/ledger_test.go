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
		assert.Equal(t, int64(50000), ledger1.AmountMinor)
		assert.Equal(t, int64(50000), ledger1.BalanceAfterMinor)

		balance, err = Apply(tx, models.PlatformSourceWithdrawal, -20000, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(30000), balance)

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(30000), w.BalanceMinor)

		var sum int64
		require.NoError(t, tx.Model(&models.PlatformLedger{}).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&sum).Error)
		assert.Equal(t, w.BalanceMinor, sum, "balance must always equal SUM(ledger.amount_minor)")
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
		assert.Equal(t, int64(10000), w.BalanceMinor, "a rejected debit must not change the balance")
	})
}
