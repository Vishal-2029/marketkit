package models

import "time"

// WalletTopup tracks a Razorpay add-money order. Mirrors DesignPurchase: the
// PENDING→SUCCESS status flip is the idempotency gate that makes the wallet
// credit exactly-once across the verify endpoint and the webhook.
type WalletTopup struct {
	ID                string        `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string        `gorm:"index;not null"                  json:"-"`
	AmountInPaise     int64         `gorm:"not null"                        json:"amount_in_paise"`
	Currency          string        `gorm:"default:'INR'"                   json:"currency"`
	Status            PaymentStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	RazorpayOrderID   *string       `gorm:"uniqueIndex"                     json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID *string       `gorm:"uniqueIndex"                     json:"razorpay_payment_id,omitempty"`
	GatewayResponse   JSONMap       `gorm:"type:jsonb"                      json:"-"`
	PaidAt            *time.Time    `                                       json:"paid_at,omitempty"`
	CreatedAt         time.Time     `                                       json:"created_at"`
}
