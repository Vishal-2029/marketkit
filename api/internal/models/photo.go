package models

import "time"

type Photo struct {
	ID          string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Title       string    `gorm:"not null"                                       json:"title"`
	Category    string    `gorm:"not null;default:'GENERAL'"                     json:"category"`
	FileKey     string    `gorm:"not null"                                       json:"file_key"`
	MimeType    string    `                                                      json:"mime_type"`
	SizeBytes   int64     `                                                      json:"size_bytes"`
	IsPublished bool      `gorm:"default:false"                                  json:"is_published"`
	VideoID     *string   `gorm:"index"                                          json:"video_id,omitempty"`
	UploadedAt  time.Time `gorm:"default:now()"                                  json:"uploaded_at"`
	UpdatedAt   time.Time `                                                      json:"updated_at"`

	// URL is computed at request time from FileKey — not stored in DB.
	URL string `gorm:"-" json:"url"`
}
