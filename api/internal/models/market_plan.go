package models

import "time"

// MarketPlan is a Design Market seller subscription tier.
// Completely separate from learning Plan (plan.go).
//
// Placeholder perks (do not invent more):
//   - FeeDiscountPct (0-100): knocks off market_fee_percent when calculating seller fee
//   - FeaturedSeller: badge for subscribers
type MarketPlan struct {
	ID             string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name           string    `gorm:"uniqueIndex;not null"                           json:"name"`
	Description    string    `gorm:"default:''"                                     json:"description"`
	PriceInPaise   int64     `gorm:"not null"                                       json:"price_in_paise"`
	DurationDays   int       `gorm:"default:30"                                     json:"duration_days"`
	FeeDiscountPct int       `gorm:"default:0"                                      json:"fee_discount_pct"` // 0-100; placeholder perk
	FeaturedSeller bool      `gorm:"default:false"                                  json:"featured_seller"`  // placeholder perk
	IsActive       bool      `gorm:"default:true"                                   json:"is_active"`
	CreatedAt      time.Time `                                                      json:"created_at"`
	UpdatedAt      time.Time `                                                      json:"updated_at"`
}
