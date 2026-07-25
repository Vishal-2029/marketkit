package cron

import (
	"log"
	"time"

	"fmt"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
)

// StartSubscriptionChecker runs daily checks for expiring and expired subscriptions.
// Call this once in a goroutine from main.go: go cron.StartSubscriptionChecker()
func StartSubscriptionChecker() {
	// Run once immediately on startup, then every 24 hours
	runChecks()
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		runChecks()
	}
}

func runChecks() {
	log.Println("[cron] running subscription expiry checks")
	sendExpiryWarnings()
	markAndNotifyExpired()
	markMarketPlanExpired()
}

// sendExpiryWarnings emails users whose active subscription expires in 6–8 days.
// The 6–8 day window prevents duplicate emails if the cron fires slightly late/early.
func sendExpiryWarnings() {
	now := time.Now()
	from := now.Add(6 * 24 * time.Hour)
	to := now.Add(8 * 24 * time.Hour)

	type row struct {
		models.Subscription
		UserName  string
		UserEmail string
		PlanName  string
	}

	var subs []row
	database.DB.
		Table("subscriptions").
		Select("subscriptions.*, users.name as user_name, users.email as user_email, plans.name as plan_name").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Joins("JOIN plans ON plans.id = subscriptions.plan_id").
		Where("subscriptions.status = ? AND subscriptions.expiry_date BETWEEN ? AND ? AND subscriptions.expiry_warning_sent = ?",
			models.SubscriptionActive, from, to, false).
		Scan(&subs)

	for _, s := range subs {
		daysLeft := int(time.Until(s.ExpiryDate).Hours() / 24)
		if daysLeft < 1 {
			daysLeft = 1
		}
		data := email.ExpiryEmailData{
			Name:      s.UserName,
			PlanName:  s.PlanName,
			ExpiresAt: email.FormatDate(s.ExpiryDate),
			DaysLeft:  daysLeft,
		}
		if err := email.SendExpiryWarningEmail(s.UserEmail, data); err != nil {
			log.Printf("[cron] expiry warning failed for %s: %v", mask.Email(s.UserEmail), err)
		} else {
			log.Printf("[cron] expiry warning sent to %s (plan: %s, days: %d)", mask.Email(s.UserEmail), s.PlanName, daysLeft)
			database.DB.Model(&models.Subscription{}).
				Where("id = ?", s.ID).
				Update("expiry_warning_sent", true)
			if err := fcm.SendToUser(s.UserID, "Subscription Expiring Soon",
				fmt.Sprintf("Your %s plan expires in %d day(s). Renew to keep access.", s.PlanName, daysLeft)); err != nil {
				log.Printf("[cron] fcm expiry warning failed for user %s: %v", s.UserID, err)
			}
		}
	}
}

// markAndNotifyExpired finds subscriptions that have passed their expiry date,
// updates their status to EXPIRED, and emails the users.
func markAndNotifyExpired() {
	now := time.Now()

	type row struct {
		models.Subscription
		UserName  string
		UserEmail string
		PlanName  string
	}

	var subs []row
	database.DB.
		Table("subscriptions").
		Select("subscriptions.*, users.name as user_name, users.email as user_email, plans.name as plan_name").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Joins("JOIN plans ON plans.id = subscriptions.plan_id").
		Where("subscriptions.status = ? AND subscriptions.expiry_date < ?",
			models.SubscriptionActive, now).
		Scan(&subs)

	for _, s := range subs {
		// Mark as expired
		database.DB.Model(&models.Subscription{}).
			Where("id = ?", s.ID).
			Update("status", models.SubscriptionExpired)

		data := email.ExpiryEmailData{
			Name:      s.UserName,
			PlanName:  s.PlanName,
			ExpiresAt: email.FormatDate(s.ExpiryDate),
			DaysLeft:  0,
		}
		if err := email.SendPlanExpiredEmail(s.UserEmail, data); err != nil {
			log.Printf("[cron] expired email failed for %s: %v", mask.Email(s.UserEmail), err)
		} else {
			log.Printf("[cron] expired email sent to %s (plan: %s)", mask.Email(s.UserEmail), s.PlanName)
		}
		if err := fcm.SendToUser(s.UserID, "Subscription Expired",
			fmt.Sprintf("Your %s plan has expired. Renew to regain access to videos.", s.PlanName)); err != nil {
			log.Printf("[cron] fcm expired failed for user %s: %v", s.UserID, err)
		}
	}
}

// markMarketPlanExpired flips ACTIVE Product Market plan subscriptions past
// their expiry date to EXPIRED. Separate from learning-plan subscriptions.
func markMarketPlanExpired() {
	now := time.Now()
	result := database.DB.Model(&models.MarketPlanSubscription{}).
		Where("status = ? AND expiry_date < ?", models.SubscriptionActive, now).
		Update("status", models.SubscriptionExpired)
	if result.Error != nil {
		log.Printf("[cron] market plan expiry failed: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[cron] marked %d market plan subscription(s) as EXPIRED", result.RowsAffected)
	}
}
