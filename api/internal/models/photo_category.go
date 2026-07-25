package models

import "time"

type PhotoCategory struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"not null;uniqueIndex"                          json:"name"`
	CreatedAt time.Time `gorm:"default:now()"                                  json:"created_at"`
}
