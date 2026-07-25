package models

import "time"

type NotificationLog struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Audience    string    `json:"audience"` // "all" or a specific user_id
	SentCount   int       `json:"sent_count"`
	FailedCount int       `json:"failed_count"`
	CreatedAt   time.Time `json:"created_at"`
}
