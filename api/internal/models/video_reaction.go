package models

import "time"

type VideoReaction struct {
	UserID    string    `gorm:"primaryKey"     json:"user_id"`
	VideoID   string    `gorm:"primaryKey"     json:"video_id"`
	Reaction  string    `gorm:"type:varchar(10);not null" json:"reaction"` // "like" or "dislike"
	CreatedAt time.Time `                      json:"created_at"`
}
