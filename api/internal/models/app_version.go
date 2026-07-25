package models

import "time"

type AppVersion struct {
	ID           string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	VersionName  string    `gorm:"not null"      json:"version_name"`
	BuildNumber  int       `gorm:"not null"      json:"build_number"`
	DownloadURL  string    `gorm:"not null"      json:"download_url"`
	ReleaseNotes string    `gorm:"default:''"   json:"release_notes"`
	IsMandatory  bool      `gorm:"default:false" json:"is_mandatory"`
	CreatedAt    time.Time `                     json:"created_at"`
	UpdatedAt    time.Time `                     json:"updated_at"`
}
