package models

import (
	"time"

	"gorm.io/gorm"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
	UserStatusExpired   UserStatus = "EXPIRED"
)

// App mode values for User.CurrentAppMode — must match the strings the
// Flutter app's AppModeService already uses, so no translation layer is
// needed between client and server.
const (
	AppModeLearning = "learning"
	AppModeMarket   = "market"
)

type User struct {
	ID               string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name             string     `gorm:"not null"                                       json:"name"`
	Email            string     `gorm:"uniqueIndex;not null"                           json:"email"`
	Phone            string     `                                                      json:"phone"`
	PasswordHash     string     `gorm:"default:''"                                     json:"-"`
	Status           UserStatus `gorm:"type:varchar(20);default:'ACTIVE'"          json:"status"`
	CurrentSessionID string     `gorm:"type:varchar(36);default:''"                json:"-"`
	AvatarURL        *string    `gorm:"type:varchar(500)"                          json:"avatar_url"`
	// Cached wallet balance; source of truth is the wallet_transactions ledger.
	// Mutated only via wallet.Apply, which row-locks this record first.
	WalletBalanceMinor int64 `gorm:"default:0" json:"wallet_balance_minor"`
	// CurrentAppMode is the dual-mode app's active side for this user:
	// "" (legacy accounts predating this feature), "learning", or "market".
	// Set at registration now that mode is chosen before signup; changed later
	// only via the explicit mode-switch endpoint.
	CurrentAppMode   string         `gorm:"type:varchar(20);default:''"            json:"current_app_mode"`
	MarketJoinedAt   *time.Time     `                                              json:"market_joined_at,omitempty"`
	LearningJoinedAt *time.Time     `                                              json:"learning_joined_at,omitempty"`
	JoinedAt         time.Time      `gorm:"default:now()"                          json:"joined_at"`
	UpdatedAt        time.Time      `                                              json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index"                                  json:"-"`

	Subscriptions []Subscription `gorm:"foreignKey:UserID" json:"subscriptions,omitempty"`
	Sessions      []UserSession  `gorm:"foreignKey:UserID" json:"sessions,omitempty"`
	Payments      []Payment      `gorm:"foreignKey:UserID" json:"payments,omitempty"`
	PlaybackLogs  []PlaybackLog  `gorm:"foreignKey:UserID" json:"playback_logs,omitempty"`
}
