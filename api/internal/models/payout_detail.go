package models

import "time"

// PayoutDetail holds a user's saved payout destinations (one row per user,
// upserted). Withdrawals snapshot these fields so later edits don't rewrite
// where past requests were meant to be paid.
type PayoutDetail struct {
	ID                string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string    `gorm:"uniqueIndex;not null"  json:"-"`
	UpiID             string    `gorm:"type:varchar(100)"     json:"upi_id"`
	BankAccountNumber string    `gorm:"type:varchar(30)"      json:"bank_account_number"`
	BankIfsc          string    `gorm:"type:varchar(11)"      json:"bank_ifsc"`
	BankHolderName    string    `gorm:"type:varchar(100)"     json:"bank_holder_name"`
	CreatedAt         time.Time `                             json:"created_at"`
	UpdatedAt         time.Time `                             json:"updated_at"`
}
