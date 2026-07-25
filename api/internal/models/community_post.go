package models

import "time"

type CommunityPost struct {
	ID         string           `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID     string           `gorm:"index;not null"                                 json:"-"`
	AnonName   string           `gorm:"not null"                                       json:"anon_name"`
	Category   string           `gorm:"type:varchar(20);default:'GENERAL'"             json:"category"`
	Title      string           `gorm:"not null"                                       json:"title"`
	Content    string           `gorm:"type:text;not null"                             json:"content"`
	ReplyCount int              `gorm:"default:0"                                      json:"reply_count"`
	IsFlagged  bool             `gorm:"default:false"                                  json:"is_flagged"`
	CreatedAt  time.Time        `                                                      json:"created_at"`
	Replies    []CommunityReply `gorm:"foreignKey:PostID"                              json:"replies,omitempty"`

	// Up to 3 image keys stored as a JSON array; URLs resolved at query time.
	ImageKeys []string `gorm:"type:text;serializer:json" json:"-"`
	ImageURLs []string `gorm:"-"                         json:"image_urls"`

	// Populated at query time for admin endpoints only; not stored in DB.
	UserName  string `gorm:"-" json:"user_name,omitempty"`
	UserEmail string `gorm:"-" json:"user_email,omitempty"`
}
