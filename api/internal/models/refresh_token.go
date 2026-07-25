package models

import "time"

type RefreshToken struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"-"`
	AdminID   string    `gorm:"not null;index"                                 json:"-"`
	TokenHash string    `gorm:"not null;uniqueIndex"                           json:"-"`
	ExpiresAt time.Time `gorm:"not null"                                       json:"-"`
	Revoked   bool      `gorm:"default:false"                                  json:"-"`
	CreatedAt time.Time `                                                      json:"-"`
}
