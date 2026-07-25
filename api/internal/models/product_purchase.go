package models

import "time"

type ProductPurchase struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProductID string `gorm:"index;not null"                                 json:"product_id"`
	BuyerID   string `gorm:"index;not null"                                 json:"-"`
	// SellerID is denormalized from Product so earnings survive product deletion.
	SellerID          string        `gorm:"index;not null"            json:"-"`
	AmountInPaise     int64         `gorm:"not null"                  json:"amount_in_paise"`
	Currency          string        `gorm:"default:'INR'"             json:"currency"`
	Status            PaymentStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	RazorpayOrderID   *string       `gorm:"uniqueIndex"               json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID *string       `gorm:"uniqueIndex"               json:"razorpay_payment_id,omitempty"`
	GatewayResponse   JSONMap       `gorm:"type:jsonb"                json:"-"`
	// Fee snapshot taken at capture time so historic sales keep the fee
	// percent they were sold under. Zero on rows from before the wallet era.
	FeeInPaise       int64      `gorm:"default:0"                        json:"fee_in_paise"`
	SellerNetInPaise int64      `gorm:"default:0"                        json:"seller_net_in_paise"`
	PaidVia          string     `gorm:"type:varchar(10);default:'RAZORPAY'" json:"paid_via"`
	PaidAt           *time.Time `                                        json:"paid_at,omitempty"`
	CreatedAt        time.Time  `                                        json:"created_at"`

	Product Product `gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE" json:"product,omitempty"`
	Buyer   User    `gorm:"foreignKey:BuyerID;constraint:OnDelete:CASCADE"  json:"buyer,omitempty"`
	Seller  User    `gorm:"foreignKey:SellerID;constraint:OnDelete:CASCADE" json:"seller,omitempty"`

	// Populated at query time for admin endpoints only; not stored in DB.
	BuyerName    string `gorm:"-" json:"buyer_name,omitempty"`
	BuyerEmail   string `gorm:"-" json:"buyer_email,omitempty"`
	SellerName   string `gorm:"-" json:"seller_name,omitempty"`
	SellerEmail  string `gorm:"-" json:"seller_email,omitempty"`
	ProductTitle string `gorm:"-" json:"product_title,omitempty"`
}
