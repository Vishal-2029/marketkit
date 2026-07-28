package models

import "time"

const (
	WalletTxTopup         = "TOPUP"
	WalletTxPurchaseDebit = "PURCHASE_DEBIT"
	WalletTxSaleCredit    = "SALE_CREDIT"
	WalletTxWithdrawal    = "WITHDRAWAL"
	WalletTxPlanDebit     = "PLAN_DEBIT"
)

// WalletTransaction is an append-only ledger row. Rows are inserted only for
// completed events (never PENDING) — pending topups live in wallet_topups.
// Credits are positive, debits negative, so SUM(amount_minor) per user
// must always equal users.wallet_balance_minor.
type WalletTransaction struct {
	ID     string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID string `gorm:"index;not null"                                 json:"-"`
	Type   string `gorm:"type:varchar(20);not null"                      json:"type"`
	// Signed: TOPUP/SALE_CREDIT positive, PURCHASE_DEBIT/WITHDRAWAL/PLAN_DEBIT negative.
	AmountMinor int64 `gorm:"not null" json:"amount_minor"`
	// Balance snapshot taken inside the row lock, for drift auditing.
	BalanceAfterMinor int64     `gorm:"not null"   json:"balance_after_minor"`
	ReferenceID       *string   `gorm:"index"      json:"reference_id,omitempty"`
	Metadata          JSONMap   `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt         time.Time `                  json:"created_at"`
}
