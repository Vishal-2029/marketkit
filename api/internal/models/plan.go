package models

import "time"

type Plan struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string `gorm:"uniqueIndex;not null"                           json:"name"`
	Description string `gorm:"default:''"                                     json:"description"`
	PriceMinor  int64  `gorm:"not null"                                       json:"price_minor"`

	// Features lists the feature keys this plan grants. A user's entitlements
	// are the union of Features across all their active subscriptions, so
	// à-la-carte plans ("A only", "A + B") and ladder tiers both work without
	// a schema change — which is why this is a list rather than a tier number.
	//
	// Keys are free-form strings you define. The kit ships with the video
	// category keys from VideoCategory (CATEGORY_A/B/C) so that content gating
	// works out of the box: a video is unlocked when its Category appears in
	// the viewer's feature set. Add your own keys for anything else you sell.
	Features []string `gorm:"type:text;serializer:json" json:"features"`

	DurationDays int       `gorm:"default:365"   json:"duration_days"`
	IsActive     bool      `gorm:"default:true"  json:"is_active"`
	CreatedAt    time.Time `                     json:"created_at"`
	UpdatedAt    time.Time `                     json:"updated_at"`
}

// FeatureList returns Features, never nil. GORM leaves the slice nil when the
// stored JSON is null, which would serialize as `null` and break clients that
// expect an array.
func (p *Plan) FeatureList() []string {
	if p.Features == nil {
		return []string{}
	}
	return p.Features
}
