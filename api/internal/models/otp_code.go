package models

import "time"

type OtpCode struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	AdminID   string    `gorm:"index;not null"                                 json:"admin_id"`
	CodeHash  string    `gorm:"not null"                                       json:"-"`
	ExpiresAt time.Time `gorm:"not null"                                       json:"expires_at"`
	Used      bool      `gorm:"default:false"                                  json:"used"`
	CreatedAt time.Time `                                                      json:"created_at"`

	Admin Admin `gorm:"foreignKey:AdminID;constraint:OnDelete:CASCADE" json:"-"`
}
