package models

import "time"

// UserNotification records that a specific user received a notification.
// One row is created per user per broadcast at send time.
type UserNotification struct {
	ID                uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID            string          `json:"user_id" gorm:"index;not null"`
	NotificationLogID uint            `json:"notification_log_id" gorm:"index;not null"`
	NotificationLog   NotificationLog `json:"-" gorm:"foreignKey:NotificationLogID"`
	Read              bool            `json:"read" gorm:"default:false"`
	CreatedAt         time.Time       `json:"created_at"`
}
