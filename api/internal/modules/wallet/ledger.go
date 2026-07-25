package wallet

import (
	"errors"
	"strconv"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientBalance = errors.New("insufficient wallet balance")

// Apply mutates a user's wallet by amountInPaise (credits positive, debits
// negative) and appends the matching ledger row. It row-locks the user record
// so concurrent debits serialize and the balance can never go negative.
//
// Must be called inside a caller-provided transaction; a rollback undoes the
// balance change and the ledger row together. When one transaction touches
// two wallets (a wallet purchase), always debit the buyer before crediting
// the seller — a single fixed lock order prevents deadlocks.
func Apply(tx *gorm.DB, userID, txType string, amountInPaise int64, referenceID *string, meta models.JSONMap) (int64, error) {
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "wallet_balance_in_paise").
		First(&user, "id = ?", userID).Error; err != nil {
		return 0, err
	}

	newBalance := user.WalletBalanceInPaise + amountInPaise
	if newBalance < 0 {
		return 0, ErrInsufficientBalance
	}

	if err := tx.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("wallet_balance_in_paise", newBalance).Error; err != nil {
		return 0, err
	}

	if err := tx.Create(&models.WalletTransaction{
		UserID:              userID,
		Type:                txType,
		AmountInPaise:       amountInPaise,
		BalanceAfterInPaise: newBalance,
		ReferenceID:         referenceID,
		Metadata:            meta,
	}).Error; err != nil {
		return 0, err
	}

	return newBalance, nil
}

func settingInt(key string, fallback int64) int64 {
	var s models.PlatformSetting
	if err := database.DB.First(&s, "key = ?", key).Error; err != nil {
		return fallback
	}
	v, err := strconv.ParseInt(s.Value, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

// FeePercent is the platform's cut on each product sale (admin-tunable).
func FeePercent() int64 { return settingInt(models.SettingMarketFeePercent, 10) }

// MinWithdrawal is the smallest allowed withdrawal, in paise (admin-tunable).
func MinWithdrawal() int64 { return settingInt(models.SettingMinWithdrawalPaise, 10000) }

// SplitFee floors the fee in integer paise; the app's display-only breakdown
// uses the same math so the numbers always match.
func SplitFee(amountInPaise, percent int64) (fee, net int64) {
	fee = amountInPaise * percent / 100
	return fee, amountInPaise - fee
}
