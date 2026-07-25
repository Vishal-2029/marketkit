package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type PaymentGateway string
type PaymentStatus string

const (
	GatewayRazorpay PaymentGateway = "RAZORPAY"
	GatewayManual   PaymentGateway = "MANUAL"

	PaymentSuccess  PaymentStatus = "SUCCESS"
	PaymentFailed   PaymentStatus = "FAILED"
	PaymentRefunded PaymentStatus = "REFUNDED"
	PaymentPending  PaymentStatus = "PENDING"
)

// JSONMap is a helper type for storing arbitrary JSON in Postgres
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONMap: expected []byte, got %T", value)
	}
	return json.Unmarshal(b, j)
}

type Payment struct {
	ID                string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID            string         `gorm:"index;not null"                                 json:"user_id"`
	PlanID            string         `gorm:"not null"                                       json:"plan_id"`
	AmountInPaise     int64          `gorm:"not null"                                       json:"amount_in_paise"`
	Currency          string         `gorm:"default:'INR'"                                  json:"currency"`
	Gateway           PaymentGateway `gorm:"type:varchar(20);not null"                      json:"gateway"`
	Status            PaymentStatus  `gorm:"type:varchar(20);not null"                      json:"status"`
	RazorpayOrderID   *string        `gorm:"uniqueIndex"                                    json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID *string        `gorm:"uniqueIndex"                                    json:"razorpay_payment_id,omitempty"`
	GatewayResponse   JSONMap        `gorm:"type:jsonb"                                     json:"gateway_response,omitempty"`
	Notes             string         `                                                      json:"notes"`
	PaidAt            *time.Time     `                                                      json:"paid_at,omitempty"`
	CreatedAt         time.Time      `                                                      json:"created_at"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Plan Plan `gorm:"foreignKey:PlanID"                             json:"plan,omitempty"`
}
