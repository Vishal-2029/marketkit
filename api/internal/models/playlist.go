package models

import "time"

type Playlist struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string `gorm:"not null"                                       json:"name"`
	Description string `gorm:"default:''"                                     json:"description"`
	// A playlist belongs to one video category; videos are only ever added to
	// a playlist whose category matches the video's ("" = legacy/uncategorized,
	// accepts nothing automatically).
	Category     VideoCategory   `gorm:"type:varchar(20);default:''"                    json:"category"`
	ThumbnailURL string          `gorm:"default:''"                                     json:"thumbnail_url"`
	CreatedAt    time.Time       `                                                      json:"created_at"`
	UpdatedAt    time.Time       `                                                      json:"updated_at"`
	Videos       []PlaylistVideo `gorm:"foreignKey:PlaylistID;constraint:OnDelete:CASCADE" json:"-"`
}

type PlaylistVideo struct {
	PlaylistID string `gorm:"primaryKey"  json:"playlist_id"`
	VideoID    string `gorm:"primaryKey"  json:"video_id"`
	Position   int    `gorm:"default:0"   json:"position"`
}
