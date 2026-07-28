package models

import "time"

// Keys in platform_settings.
const (
	SettingMarketFeePercent   = "market_fee_percent"
	SettingMinWithdrawalMinor = "min_withdrawal_minor"
)

// PlatformSetting is a generic admin-tunable key/value row.
type PlatformSetting struct {
	Key       string    `gorm:"primaryKey;type:varchar(50)" json:"key"`
	Value     string    `gorm:"not null"                    json:"value"`
	UpdatedAt time.Time `                                   json:"updated_at"`
}
