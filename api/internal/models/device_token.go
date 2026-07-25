package models

import "time"

// DeviceToken stores FCM push notification tokens per user device.
// One user can have multiple tokens (multiple devices).
type DeviceToken struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    string    `gorm:"index;not null"                                 json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null"                           json:"token"`
	Platform  string    `gorm:"type:varchar(10);default:'android'"             json:"platform"`
	UpdatedAt time.Time `gorm:"default:now()"                                  json:"updated_at"`
}
