package subscriptions

import (
	"slices"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ActivateOrExtend creates a new active subscription or extends an existing one
// for the same user+plan without cancelling other plans.
// Callers are expected to run this inside a transaction (tx) — the row lock
// below only serializes concurrent calls against an *existing* subscription
// row; it doesn't hold beyond the caller's transaction.
func ActivateOrExtend(tx *gorm.DB, userID, planID string, newExpiry time.Time, activatedBy string) error {
	var existing models.Subscription
	err := tx.Preload("Plan").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND plan_id = ? AND status = ? AND expiry_date > ?",
			userID, planID, models.SubscriptionActive, time.Now()).
		First(&existing).Error
	if err == nil {
		base := existing.ExpiryDate
		if base.Before(time.Now()) {
			base = time.Now()
		}
		var plan models.Plan
		if err := tx.First(&plan, "id = ?", planID).Error; err != nil {
			return err
		}
		extended := base.AddDate(0, 0, plan.DurationDays)
		if extended.After(newExpiry) {
			newExpiry = extended
		}
		return tx.Model(&existing).Updates(map[string]interface{}{
			"expiry_date":  newExpiry,
			"activated_by": activatedBy,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return tx.Create(&models.Subscription{
		UserID:      userID,
		PlanID:      planID,
		ExpiryDate:  newExpiry,
		ActivatedBy: activatedBy,
	}).Error
}

// UserFeatureAccess returns the union of feature keys across all active,
// unexpired subscriptions, de-duplicated and never nil. An empty result means
// the user has no active entitlement.
//
// Holding two plans is additive: "A only" plus "B only" grants both, the same
// as a single "A + B" plan.
func UserFeatureAccess(userID string) []string {
	var subs []models.Subscription
	database.DB.
		Preload("Plan").
		Where("user_id = ? AND status = ? AND expiry_date > ?", userID, models.SubscriptionActive, time.Now()).
		Find(&subs)

	seen := make(map[string]bool)
	features := make([]string, 0)
	for _, s := range subs {
		for _, f := range s.Plan.Features {
			if !seen[f] {
				seen[f] = true
				features = append(features, f)
			}
		}
	}
	return features
}

// HasFeature reports whether key is present in features. Use it for any
// entitlement check so the comparison stays in one place.
func HasFeature(features []string, key string) bool {
	return slices.Contains(features, key)
}

// SubscriptionData builds a JSON-friendly map for a subscription with its plan.
func SubscriptionData(sub *models.Subscription, plan *models.Plan) map[string]interface{} {
	return map[string]interface{}{
		"id":         sub.ID,
		"plan_id":    sub.PlanID,
		"plan_name":  plan.Name,
		"status":     string(sub.Status),
		"expires_at": sub.ExpiryDate,
		"features":   plan.FeatureList(),
	}
}
