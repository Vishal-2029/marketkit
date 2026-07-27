package models

import "time"

// MarketPlanSubscription tracks a user's Product Market plan subscription.
// Completely separate from learning Subscription (subscription.go).
type MarketPlanSubscription struct {
	ID                string             `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string             `gorm:"index;not null"                                 json:"user_id"`
	PlanID            string             `gorm:"not null"                                       json:"plan_id"`
	Status            SubscriptionStatus `gorm:"type:varchar(20);default:'ACTIVE'"              json:"status"`
	StartDate         time.Time          `gorm:"default:now()"                                  json:"start_date"`
	ExpiryDate        time.Time          `gorm:"index"                                          json:"expiry_date"`
	AmountInPaise     int64              `gorm:"default:0"                                      json:"amount_in_paise"`
	ProviderOrderID   *string            `gorm:"uniqueIndex"                                    json:"provider_order_id,omitempty"`
	ProviderPaymentID *string            `gorm:"uniqueIndex"                                    json:"provider_payment_id,omitempty"`
	PaidAt            *time.Time         `                                                      json:"paid_at,omitempty"`
	CreatedAt         time.Time          `                                                      json:"created_at"`
	UpdatedAt         time.Time          `                                                      json:"updated_at"`

	User User       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Plan MarketPlan `gorm:"foreignKey:PlanID"                             json:"plan,omitempty"`
}

func (MarketPlanSubscription) TableName() string { return "market_plan_subscriptions" }
