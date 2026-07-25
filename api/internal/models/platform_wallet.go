package models

import "time"

// Ledger sources — LEARNING_PLAN/MARKET_PLAN/PLATFORM_FEE are credits,
// WITHDRAWAL is the only debit source.
const (
	PlatformSourceLearningPlan = "LEARNING_PLAN"
	PlatformSourceMarketPlan   = "MARKET_PLAN"
	PlatformSourcePlatformFee  = "PLATFORM_FEE"
	PlatformSourceWithdrawal   = "WITHDRAWAL"
)

// PlatformWalletSingletonID is the fixed row ID for the one organization-wide
// platform wallet balance — there is exactly one row, ever.
const PlatformWalletSingletonID = "platform"

// PlatformWallet is the single-row balance for the organization-wide
// platform wallet (super-admin only). SUM(platform_ledger.amount_in_paise)
// must always equal BalanceInPaise.
type PlatformWallet struct {
	ID             string     `gorm:"primaryKey;type:varchar(20)" json:"id"`
	BalanceInPaise int64      `gorm:"not null;default:0"          json:"balance_in_paise"`
	BackfilledAt   *time.Time `                                    json:"backfilled_at,omitempty"`
	UpdatedAt      time.Time  `                                    json:"updated_at"`
}

// PlatformLedger is an append-only ledger row for the platform wallet,
// mirroring WalletTransaction's product. Type is CREDIT/DEBIT (derived from
// the amount's sign); Source is one of the PlatformSource* constants.
type PlatformLedger struct {
	ID                  string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Type                string    `gorm:"type:varchar(10);not null"                      json:"type"`
	Source              string    `gorm:"type:varchar(20);not null;index"                json:"source"`
	AmountInPaise       int64     `gorm:"not null"                                       json:"amount_in_paise"`
	BalanceAfterInPaise int64     `gorm:"not null"                                       json:"balance_after_in_paise"`
	ReferenceID         *string   `gorm:"index"                                          json:"reference_id,omitempty"`
	Metadata            JSONMap   `gorm:"type:jsonb"                                     json:"metadata,omitempty"`
	CreatedAt           time.Time `                                                      json:"created_at"`
}
