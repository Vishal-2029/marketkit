package models

import "time"

type UserSession struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID       string     `gorm:"index;not null"                                 json:"user_id"`
	DeviceInfo   string     `                                                      json:"device_info"`
	IPAddress    string     `                                                      json:"ip_address"`
	Location     string     `                                                      json:"location"`
	JwtJti       string     `gorm:"uniqueIndex;not null"                           json:"-"`
	LastActiveAt time.Time  `gorm:"default:now()"                                  json:"last_active_at"`
	CreatedAt    time.Time  `                                                      json:"created_at"`
	RevokedAt    *time.Time `gorm:"index"                                          json:"revoked_at,omitempty"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}
