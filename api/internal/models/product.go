package models

import "time"

type Product struct {
	ID            string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	SellerID      string    `gorm:"index;not null"                                 json:"-"`
	Title         string    `gorm:"not null"                                       json:"title"`
	Description   string    `gorm:"type:text;default:''"                           json:"description"`
	PriceMinor    int64     `gorm:"not null"                                       json:"price_minor"`
	FileKey       string    `gorm:"not null"                                       json:"-"`
	FileName      string    `gorm:"not null"                                       json:"file_name"`
	FileSizeBytes int64     `                                                      json:"file_size_bytes"`
	FileFormat    string    `gorm:"type:varchar(10)"                               json:"file_format"`
	IsActive      bool      `gorm:"default:true;index"                             json:"is_active"`
	SalesCount    int       `gorm:"default:0"                                      json:"sales_count"`
	ViewCount     int       `gorm:"default:0"                                      json:"view_count"`
	CreatedAt     time.Time `                                                      json:"created_at"`
	UpdatedAt     time.Time `                                                      json:"updated_at"`

	// CategoryID is nullable so pre-existing products aren't broken by this
	// field's addition. CategoryOther holds the seller's free-text
	// description when CategoryID points at the "Other" category.
	CategoryID    *string `gorm:"index"     json:"category_id,omitempty"`
	CategoryOther *string `                 json:"category_other,omitempty"`

	Seller   User             `gorm:"foreignKey:SellerID;constraint:OnDelete:CASCADE"   json:"-"`
	Category *ProductCategory `gorm:"foreignKey:CategoryID;constraint:OnDelete:SET NULL" json:"category,omitempty"`

	// Preview image keys stored as a JSON array; URLs resolved at query time.
	PreviewKeys []string `gorm:"type:text;serializer:json" json:"-"`
	PreviewURLs []string `gorm:"-"                         json:"preview_urls"`

	// Populated at query time; not stored in DB.
	SellerName     string `gorm:"-" json:"seller_name"`
	FeaturedSeller bool   `gorm:"-" json:"featured_seller"` // true when seller has active market plan with FeaturedSeller perk
	IsMine         bool   `gorm:"-" json:"is_mine"`
	IsPurchased    bool   `gorm:"-" json:"is_purchased"`
	SellerEmail    string `gorm:"-" json:"seller_email,omitempty"`
}
