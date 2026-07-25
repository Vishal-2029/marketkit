package models

import "time"

type UserOtpCode struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"-"`
	UserID    string    `gorm:"not null;index"                                 json:"-"`
	CodeHash  string    `gorm:"not null"                                       json:"-"`
	ExpiresAt time.Time `gorm:"not null"                                       json:"-"`
	Used      bool      `gorm:"default:false"                                  json:"-"`
	CreatedAt time.Time `                                                      json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}
