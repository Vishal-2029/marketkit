package models

import "time"

type VideoComment struct {
	ID      string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	VideoID string `gorm:"index;not null"                                 json:"video_id"`
	UserID  string `gorm:"index;not null"                                 json:"-"`

	// ThreadUserID identifies which end user's private conversation this row
	// belongs to. For a user-authored row it equals UserID. For an
	// admin-authored reply, UserID is the "admin" sentinel while
	// ThreadUserID is the target user — so a single
	// Where("thread_user_id = ?") query returns a user's own messages plus
	// admin's replies to them, never another user's rows.
	ThreadUserID string `gorm:"index;not null;default:''" json:"thread_user_id"`
	IsAdmin      bool   `gorm:"not null;default:false"     json:"is_admin"`

	UserName  string    `gorm:"not null"           json:"user_name"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `                          json:"created_at"`
}
