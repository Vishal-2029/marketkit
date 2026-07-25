package models

import "time"

// Keys in platform_settings.
const (
	SettingMarketFeePercent   = "market_fee_percent"
	SettingMinWithdrawalPaise = "min_withdrawal_in_paise"
)

// PlatformSetting is a generic admin-tunable key/value row.
type PlatformSetting struct {
	Key       string    `gorm:"primaryKey;type:varchar(50)" json:"key"`
	Value     string    `gorm:"not null"                    json:"value"`
	UpdatedAt time.Time `                                   json:"updated_at"`
}
