package models

import "time"

// ProductPurchaseMessage is a private support thread scoped to one purchase,
// visible only to the buyer and admins — the seller is never a participant.
// Mirrors VideoComment's ThreadUserID/IsAdmin pattern.
type ProductPurchaseMessage struct {
	ID         string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	PurchaseID string `gorm:"index;not null"                                 json:"purchase_id"`

	// ThreadUserID is always the buyer's user ID — for a buyer-authored row
	// UserID equals ThreadUserID; for an admin-authored reply UserID is the
	// "admin" sentinel while ThreadUserID stays the buyer, so a single
	// Where("purchase_id = ? AND thread_user_id = ?") query returns the
	// whole thread and nothing from any other buyer.
	UserID       string `gorm:"index;not null"             json:"-"`
	ThreadUserID string `gorm:"index;not null;default:''"  json:"thread_user_id"`
	IsAdmin      bool   `gorm:"not null;default:false"     json:"is_admin"`

	UserName  string    `gorm:"not null"           json:"user_name"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `                          json:"created_at"`
}
