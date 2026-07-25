package models

import "time"

// DesignCategory is a 2-level tree (parent section + child sub-section) used
// to group marketplace designs on the Design Market home page. Names start
// as seeded placeholders ("Section 1", "Section 1.1", ...) and are renamed
// later directly in the DB — no rename UI yet.
type DesignCategory struct {
	ID           string  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ParentID     *string `gorm:"index"                                          json:"parent_id,omitempty"`
	Name         string  `gorm:"not null"                                       json:"name"`
	DisplayOrder int     `gorm:"default:0"                                      json:"display_order"`
	// IsOther marks the single catch-all category sellers pick when nothing
	// else fits; selecting it reveals Design.CategoryOther for a free-text
	// description instead of relying on a fragile name match.
	IsOther   bool      `gorm:"default:false" json:"is_other"`
	CreatedAt time.Time `                     json:"created_at"`

	// PhotoKey is the admin-set banner photo for this section's row on the
	// Design Market home page. Nil until an admin uploads one, in which case
	// the app falls back to the first design's own preview image instead.
	PhotoKey *string `                     json:"-"`
	PhotoURL string  `gorm:"-"             json:"photo_url,omitempty"`
}
