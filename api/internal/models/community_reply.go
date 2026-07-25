package models

import "time"

type CommunityReply struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	PostID    string    `gorm:"index;not null"                                 json:"post_id"`
	UserID    string    `gorm:"index;not null"                                 json:"-"`
	AnonName  string    `gorm:"not null"                                       json:"anon_name"`
	Content   string    `gorm:"type:text;not null"                             json:"content"`
	IsFlagged bool      `gorm:"default:false"                                  json:"is_flagged"`
	CreatedAt time.Time `                                                      json:"created_at"`

	// Optional single image; URL resolved at query time.
	ImageKey *string `gorm:"type:text" json:"-"`
	ImageURL *string `gorm:"-"         json:"image_url"`
}
