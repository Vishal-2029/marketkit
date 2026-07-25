package models

import "time"

type Plan struct {
	ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"uniqueIndex;not null"                           json:"name"`
	Description  string    `gorm:"default:''"                                     json:"description"`
	PriceInPaise int64     `gorm:"not null"                                       json:"price_in_paise"`
	HasWillcom   bool      `gorm:"default:false"                                  json:"has_willcom"`
	HasE4        bool      `gorm:"default:false"                                  json:"has_e4"`
	HasMecad     bool      `gorm:"default:false"                                  json:"has_mecad"`
	DurationDays int       `gorm:"default:365"                                    json:"duration_days"`
	IsActive     bool      `gorm:"default:true"                                   json:"is_active"`
	CreatedAt    time.Time `                                                      json:"created_at"`
	UpdatedAt    time.Time `                                                      json:"updated_at"`
}
