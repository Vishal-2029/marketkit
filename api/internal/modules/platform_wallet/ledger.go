// Package platform_wallet implements the single, organization-wide platform
// wallet: the running total of platform income (learning-plan payments,
// market-plan payments, and product-sale platform fees) minus super-admin
// withdrawals. It is not per-admin and not split — there is exactly one
// balance, visible and spendable only by super-admins.
package platform_wallet

import (
	"errors"
	"log/slog"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientBalance = errors.New("insufficient platform wallet balance")

// Apply mutates the singleton platform wallet balance by amountMinor
// (credits positive, debits negative) and appends the matching ledger row.
// It row-locks the wallet record so concurrent credits/debits serialize.
//
// Must be called inside a caller-provided transaction, in the same
// transaction as the event that earns/spends it (a product sale, a plan
// activation, a withdrawal) — a rollback must undo the platform credit
// together with whatever else that transaction did.
func Apply(tx *gorm.DB, source string, amountMinor int64, referenceID *string, meta models.JSONMap) (int64, error) {
	var w models.PlatformWallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&w, "id = ?", models.PlatformWalletSingletonID).Error; err != nil {
		return 0, err
	}

	newBalance := w.BalanceMinor + amountMinor
	if newBalance < 0 {
		return 0, ErrInsufficientBalance
	}

	if err := tx.Model(&models.PlatformWallet{}).
		Where("id = ?", models.PlatformWalletSingletonID).
		UpdateColumn("balance_minor", newBalance).Error; err != nil {
		return 0, err
	}

	txType := "CREDIT"
	if amountMinor < 0 {
		txType = "DEBIT"
	}
	if err := tx.Create(&models.PlatformLedger{
		Type:              txType,
		Source:            source,
		AmountMinor:       amountMinor,
		BalanceAfterMinor: newBalance,
		ReferenceID:       referenceID,
		Metadata:          meta,
	}).Error; err != nil {
		return 0, err
	}

	return newBalance, nil
}

// Backfill seeds the platform ledger/balance from historical rows the first
// time it runs: SUM(fee_minor) over successful product_purchases, plus
// historical learning payments, plus paid market_plan_subscriptions. Guarded
// by PlatformWallet.BackfilledAt so re-runs on every boot never double-credit.
func Backfill() {
	var w models.PlatformWallet
	if err := database.DB.First(&w, "id = ?", models.PlatformWalletSingletonID).Error; err != nil {
		slog.Error("platform_wallet: backfill skipped, singleton row missing", "error", err)
		return
	}
	if w.BackfilledAt != nil {
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var learningTotal, marketPlanTotal, feeTotal int64

		if err := tx.Model(&models.Payment{}).
			Where("status = ?", models.PaymentSuccess).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&learningTotal).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.MarketPlanSubscription{}).
			Where("paid_at IS NOT NULL").
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&marketPlanTotal).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ProductPurchase{}).
			Where("status = ?", models.PaymentSuccess).
			Select("COALESCE(SUM(fee_minor), 0)").Scan(&feeTotal).Error; err != nil {
			return err
		}

		meta := models.JSONMap{"backfill": true}
		if learningTotal > 0 {
			if _, err := Apply(tx, models.PlatformSourceLearningPlan, learningTotal, nil, meta); err != nil {
				return err
			}
		}
		if marketPlanTotal > 0 {
			if _, err := Apply(tx, models.PlatformSourceMarketPlan, marketPlanTotal, nil, meta); err != nil {
				return err
			}
		}
		if feeTotal > 0 {
			if _, err := Apply(tx, models.PlatformSourcePlatformFee, feeTotal, nil, meta); err != nil {
				return err
			}
		}

		now := time.Now()
		return tx.Model(&models.PlatformWallet{}).
			Where("id = ?", models.PlatformWalletSingletonID).
			Update("backfilled_at", now).Error
	})
	if err != nil {
		slog.Error("platform_wallet: backfill failed", "error", err)
		return
	}
	slog.Info("platform_wallet: backfill complete")
}
