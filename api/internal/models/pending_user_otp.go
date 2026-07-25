package models

import "time"

// PendingUserOTP holds a short-lived OTP for admin-initiated user creation —
// verifying an email address before the User row exists yet, so it can't use
// UserOtpCode (which requires an existing UserID foreign key). Rows are
// deleted once consumed or superseded by a resend; never queried by anything
// outside this flow.
type PendingUserOTP struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"-"`
	Email     string    `gorm:"index;not null"                                 json:"-"`
	CodeHash  string    `gorm:"not null"                                       json:"-"`
	ExpiresAt time.Time `gorm:"not null"                                       json:"-"`
	CreatedAt time.Time `                                                      json:"-"`
}
