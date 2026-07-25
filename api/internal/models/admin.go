package models

import (
	"strings"
	"time"
)

type Admin struct {
	ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	FirstName    string    `                            json:"first_name"`
	LastName     string    `                            json:"last_name"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	Phone        string    `gorm:"index"                json:"phone"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"`
	AvatarURL    *string   `gorm:"type:varchar(500)"    json:"avatar_url"`
	IsActive     bool      `gorm:"default:true"         json:"is_active"`
	IsSuper      bool      `gorm:"default:false"        json:"is_super"`
	CreatedAt    time.Time `                            json:"created_at"`
	UpdatedAt    time.Time `                            json:"updated_at"`

	OtpCodes []OtpCode `gorm:"foreignKey:AdminID;constraint:OnDelete:CASCADE" json:"-"`
}

func (a *Admin) FullName() string {
	return strings.TrimSpace(a.FirstName + " " + a.LastName)
}
