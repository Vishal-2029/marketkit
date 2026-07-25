package models

import (
	"time"

	"gorm.io/gorm"
)

type VideoCategory string
type VideoStatus string

const (
	CategoryWillcom VideoCategory = "WILLCOM"
	CategoryE4      VideoCategory = "E4"
	CategoryMecad   VideoCategory = "MECAD"

	VideoStatusDraft      VideoStatus = "DRAFT"
	VideoStatusProcessing VideoStatus = "PROCESSING"
	VideoStatusPublished  VideoStatus = "PUBLISHED"
	VideoStatusError      VideoStatus = "ERROR"
)

type Video struct {
	ID          string        `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Title       string        `gorm:"not null"                                       json:"title"`
	Description string        `                                                      json:"description"`
	Category    VideoCategory `gorm:"type:varchar(20);not null"                      json:"category"`
	Status      VideoStatus   `gorm:"type:varchar(20);default:'DRAFT';index"         json:"status"`
	// Raw storage keys are never serialized to any client (admin or user) —
	// they resolve to permanent, unauthenticated URLs on local storage, so
	// exposing them would let anyone with the JSON bypass the subscription
	// paywall (and the signed-URL/entitlement flow) entirely. Clients only
	// ever get a resolved PreviewURL (entitlement-gated) or a time-limited
	// signed stream URL via the dedicated /stream-url endpoint.
	FileKey         string  `                                                      json:"-"`
	DurationSeconds int     `                                                      json:"duration_seconds"`
	ThumbnailURL    *string `                                                      json:"thumbnail_url"`
	PreviewURL      string  `gorm:"-"                                               json:"preview_url,omitempty"`
	HLSKey          string  `                                                      json:"-"`
	MP4Key1080p     string  `                                                      json:"-"`
	MP4Key720p      string  `                                                      json:"-"`
	MP4Key480p      string  `                                                      json:"-"`
	MP4Key360p      string  `                                                      json:"-"`
	MP4Key240p      string  `                                                      json:"-"`
	MP4Size1080p    int64   `                                                      json:"mp4_size_1080p,omitempty"`
	MP4Size720p     int64   `                                                      json:"mp4_size_720p,omitempty"`
	MP4Size480p     int64   `                                                      json:"mp4_size_480p,omitempty"`
	MP4Size360p     int64   `                                                      json:"mp4_size_360p,omitempty"`
	MP4Size240p     int64   `                                                      json:"mp4_size_240p,omitempty"`
	LQIP            string  `gorm:"type:text"                                      json:"lqip,omitempty"`
	TranscodeError  string  `                                                      json:"transcode_error,omitempty"`
	// Transcode timing, shown in the admin panel (how long each video took).
	TranscodeStartedAt  *time.Time `                                                     json:"transcode_started_at,omitempty"`
	TranscodeFinishedAt *time.Time `                                                     json:"transcode_finished_at,omitempty"`
	// Safe "is transcoded" flag for clients — HLSKey itself is never
	// serialized (raw storage key), so the admin UI needs this to know
	// whether the HLS ladder exists. Set by the AfterFind hook.
	HasHLS     bool      `gorm:"-"                                              json:"has_hls"`
	IsPreview  bool      `gorm:"default:false"                                  json:"is_preview"`
	IsFree     bool      `gorm:"default:false"                                  json:"is_free"`
	IsIntro    bool      `gorm:"default:false;index"                            json:"is_intro"`
	UploadedAt time.Time `gorm:"default:now()"                                  json:"uploaded_at"`
	UpdatedAt  time.Time `                                                      json:"updated_at"`

	PlaybackLogs []PlaybackLog `gorm:"foreignKey:VideoID" json:"playback_logs,omitempty"`
}

// AfterFind derives HasHLS on every load so it's always correct for any
// handler that returns videos, without each one having to remember to set it.
func (v *Video) AfterFind(_ *gorm.DB) error {
	v.HasHLS = v.HLSKey != ""
	return nil
}

// GetMP4Key returns the storage key for the given quality label (e.g. "720p").
func (v *Video) GetMP4Key(quality string) string {
	switch quality {
	case "1080p":
		return v.MP4Key1080p
	case "720p":
		return v.MP4Key720p
	case "480p":
		return v.MP4Key480p
	case "360p":
		return v.MP4Key360p
	case "240p":
		return v.MP4Key240p
	}
	return ""
}

// GetMP4Size returns the byte size of the given quality's MP4, or 0 if unknown.
func (v *Video) GetMP4Size(quality string) int64 {
	switch quality {
	case "1080p":
		return v.MP4Size1080p
	case "720p":
		return v.MP4Size720p
	case "480p":
		return v.MP4Size480p
	case "360p":
		return v.MP4Size360p
	case "240p":
		return v.MP4Size240p
	}
	return 0
}
