package models

import "time"

// WalletTopup tracks a Razorpay add-money order. Mirrors ProductPurchase: the
// PENDING→SUCCESS status flip is the idempotency gate that makes the wallet
// credit exactly-once across the verify endpoint and the webhook.
type WalletTopup struct {
	ID                string        `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string        `gorm:"index;not null"                  json:"-"`
	AmountInPaise     int64         `gorm:"not null"                        json:"amount_in_paise"`
	Currency          string        `gorm:"default:'INR'"                   json:"currency"`
	Status            PaymentStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	ProviderOrderID   *string       `gorm:"uniqueIndex"                     json:"provider_order_id,omitempty"`
	ProviderPaymentID *string       `gorm:"uniqueIndex"                     json:"provider_payment_id,omitempty"`
	GatewayResponse   JSONMap       `gorm:"type:jsonb"                      json:"-"`
	PaidAt            *time.Time    `                                       json:"paid_at,omitempty"`
	CreatedAt         time.Time     `                                       json:"created_at"`
}
