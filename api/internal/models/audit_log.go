package models

import "time"

type AuditEventType string

const (
	EventLogin            AuditEventType = "LOGIN"
	EventLogout           AuditEventType = "LOGOUT"
	EventForceLogout      AuditEventType = "FORCE_LOGOUT"
	EventVideoPlay        AuditEventType = "VIDEO_PLAY"
	EventPlanPurchase     AuditEventType = "PLAN_PURCHASE"
	EventAdminAction      AuditEventType = "ADMIN_ACTION"
	EventPaymentFailed    AuditEventType = "PAYMENT_FAILED"
	EventUserSuspended    AuditEventType = "USER_SUSPENDED"
	EventUserReactivated  AuditEventType = "USER_REACTIVATED"
	EventPlanChanged      AuditEventType = "PLAN_CHANGED"
	EventManualActivation AuditEventType = "MANUAL_ACTIVATION"
	EventVideoDeleted     AuditEventType = "VIDEO_DELETED"
	EventUserDeleted      AuditEventType = "USER_DELETED"
	EventAppModeSwitched  AuditEventType = "APP_MODE_SWITCHED"
)

// AuditLog is append-only — never update or delete rows
type AuditLog struct {
	ID           string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	EventType    AuditEventType `gorm:"type:varchar(30);index;not null"                json:"event_type"`
	ActorUserID  *string        `gorm:"index"                                          json:"actor_user_id,omitempty"`
	ActorAdminID *string        `gorm:"index"                                          json:"actor_admin_id,omitempty"`
	TargetID     *string        `                                                      json:"target_id,omitempty"`
	IPAddress    string         `                                                      json:"ip_address"`
	Device       string         `                                                      json:"device"`
	Details      JSONMap        `gorm:"type:jsonb"                                     json:"details,omitempty"`
	CreatedAt    time.Time      `gorm:"index"                                          json:"created_at"`

	// -:migration prevents GORM from creating FK constraints — logs must outlive actors
	ActorUser  *User  `gorm:"-:migration;foreignKey:ActorUserID"  json:"actor_user,omitempty"`
	ActorAdmin *Admin `gorm:"-:migration;foreignKey:ActorAdminID" json:"actor_admin,omitempty"`
}
