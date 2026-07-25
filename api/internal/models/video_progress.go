package models

import "time"

// VideoProgress stores the last playback position for a user+video pair.
// Upserted on every progress save — one row per (user_id, video_id).
type VideoProgress struct {
	UserID          string    `gorm:"primaryKey"      json:"user_id"`
	VideoID         string    `gorm:"primaryKey"      json:"video_id"`
	PositionSeconds int       `gorm:"default:0"       json:"position_seconds"`
	UpdatedAt       time.Time `gorm:"default:now();index" json:"updated_at"`
}
