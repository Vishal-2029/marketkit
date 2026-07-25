package models

import "time"

type PlaybackLog struct {
	ID             string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID         string    `gorm:"index;not null"                                 json:"user_id"`
	VideoID        string    `gorm:"index;not null"                                 json:"video_id"`
	WatchedSeconds int       `gorm:"default:0"                                      json:"watched_seconds"`
	Completed      bool      `gorm:"default:false"                                  json:"completed"`
	WatermarkID    string    `                                                      json:"watermark_id"`
	PlayedAt       time.Time `gorm:"default:now()"                                  json:"played_at"`

	User  User  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"  json:"user,omitempty"`
	Video Video `gorm:"foreignKey:VideoID;constraint:OnDelete:CASCADE" json:"video,omitempty"`
}
