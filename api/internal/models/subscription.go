package models

import "time"

type SubscriptionStatus string

const (
	SubscriptionPending   SubscriptionStatus = "PENDING" // unused by Learning; reused by MarketPlanSubscription for unpaid orders
	SubscriptionActive    SubscriptionStatus = "ACTIVE"
	SubscriptionExpired   SubscriptionStatus = "EXPIRED"
	SubscriptionSuspended SubscriptionStatus = "SUSPENDED"
	SubscriptionCancelled SubscriptionStatus = "CANCELLED"
)

type Subscription struct {
	ID                string             `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string             `gorm:"index;not null"                                 json:"user_id"`
	PlanID            string             `gorm:"not null"                                       json:"plan_id"`
	Status            SubscriptionStatus `gorm:"type:varchar(20);default:'ACTIVE'"              json:"status"`
	StartDate         time.Time          `gorm:"default:now()"                                  json:"start_date"`
	ExpiryDate        time.Time          `gorm:"index"                                    json:"expiry_date"`
	ActivatedBy       string             `                                                json:"activated_by"`
	ExpiryWarningSent bool               `gorm:"default:false"                            json:"expiry_warning_sent"`
	CreatedAt         time.Time          `                                                      json:"created_at"`
	UpdatedAt         time.Time          `                                                      json:"updated_at"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Plan Plan `gorm:"foreignKey:PlanID"                             json:"plan,omitempty"`
}
