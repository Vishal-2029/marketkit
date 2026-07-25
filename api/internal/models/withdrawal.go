package models

import "time"

const (
	WithdrawalMethodUPI  = "UPI"
	WithdrawalMethodBank = "BANK"

	// No approval gate by design: the balance is deducted and the request is
	// APPROVED the moment it's created. SETTLED means the admin actually
	// transferred the money (manually, outside the app).
	WithdrawalStatusApproved = "APPROVED"
	WithdrawalStatusSettled  = "SETTLED"
)

type Withdrawal struct {
	ID            string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID        string `gorm:"index;not null"            json:"-"`
	AmountInPaise int64  `gorm:"not null"                  json:"amount_in_paise"`
	Method        string `gorm:"type:varchar(10);not null" json:"method"`
	// Payout destination snapshotted at request time.
	UpiID             string     `gorm:"type:varchar(100)"               json:"upi_id,omitempty"`
	BankAccountNumber string     `gorm:"type:varchar(30)"                json:"bank_account_number,omitempty"`
	BankIfsc          string     `gorm:"type:varchar(11)"                json:"bank_ifsc,omitempty"`
	BankHolderName    string     `gorm:"type:varchar(100)"               json:"bank_holder_name,omitempty"`
	Status            string     `gorm:"type:varchar(20);not null;index" json:"status"`
	SettledBy         *string    `                                       json:"settled_by,omitempty"`
	SettledAt         *time.Time `                                       json:"settled_at,omitempty"`
	Note              string     `gorm:"type:varchar(500);default:''"    json:"note"`
	CreatedAt         time.Time  `                                       json:"created_at"`

	// Populated at query time for admin endpoints only; not stored in DB.
	UserName  string `gorm:"-" json:"user_name,omitempty"`
	UserEmail string `gorm:"-" json:"user_email,omitempty"`
}
