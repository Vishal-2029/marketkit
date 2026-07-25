package models

import "time"

type RefundStatus string

const (
	RefundPending  RefundStatus = "PENDING"
	RefundApproved RefundStatus = "APPROVED"
	RefundRejected RefundStatus = "REJECTED"
)

type RefundRequest struct {
	ID          string       `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	PaymentID   string       `gorm:"index;not null"                                 json:"payment_id"`
	RequestedBy string       `gorm:"not null"                                       json:"requested_by"`
	Reason      string       `gorm:"not null"                                       json:"reason"`
	Status      RefundStatus `gorm:"type:varchar(20);default:'PENDING';index"       json:"status"`
	ReviewedBy  *string      `                                                      json:"reviewed_by,omitempty"`
	ReviewNote  string       `                                                      json:"review_note"`
	ReviewedAt  *time.Time   `                                                      json:"reviewed_at,omitempty"`
	RefundID    string       `gorm:"default:''"                                     json:"refund_id"` // filled on approval
	CreatedAt   time.Time    `                                                      json:"created_at"`
	UpdatedAt   time.Time    `                                                      json:"updated_at"`

	Payment          Payment `gorm:"foreignKey:PaymentID"                             json:"payment,omitempty"`
	RequestedByAdmin Admin   `gorm:"foreignKey:RequestedBy;references:ID"             json:"requested_by_admin,omitempty"`
	ReviewedByAdmin  *Admin  `gorm:"foreignKey:ReviewedBy;references:ID"              json:"reviewed_by_admin,omitempty"`
}
