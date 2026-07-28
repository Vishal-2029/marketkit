package subscriptions

import (
	"testing"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.RunMain(m) }

// Plan.Features is a []string persisted through GORM's json serializer. A map
// -based Updates() call has to route through that serializer too, and a nil
// slice must never reach clients as a JSON null — both are easy to break and
// neither shows up at compile time.
func TestPlanFeatures_RoundTrip(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		plan := models.Plan{
			Name:         "Round Trip " + time.Now().Format("150405.000000"),
			PriceMinor:   99900,
			Features:     []string{string(models.CategoryA), string(models.CategoryB)},
			DurationDays: 365,
		}
		require.NoError(t, tx.Create(&plan).Error)

		var got models.Plan
		require.NoError(t, tx.First(&got, "id = ?", plan.ID).Error)
		assert.Equal(t, []string{"CATEGORY_A", "CATEGORY_B"}, got.Features,
			"features must survive a Create/read round trip")

		// This mirrors what PATCH /plans/:id does. It must go through the
		// struct field: a map-based Updates({"features": []string{...}})
		// bypasses the serializer and writes Go slice syntax ("[CATEGORY_C]"),
		// which then fails to unmarshal on the next read.
		got.Features = []string{string(models.CategoryC)}
		require.NoError(t, tx.Model(&got).Select("Features").Updates(&got).Error)

		var updated models.Plan
		require.NoError(t, tx.First(&updated, "id = ?", plan.ID).Error)
		assert.Equal(t, []string{"CATEGORY_C"}, updated.Features,
			"updating features must apply the json serializer")
	})
}

func TestPlanFeatureList_NeverNil(t *testing.T) {
	var p models.Plan
	assert.NotNil(t, p.FeatureList(), "must be an empty array, never nil")
	assert.Empty(t, p.FeatureList())

	p.Features = []string{"X"}
	assert.Equal(t, []string{"X"}, p.FeatureList())
}

func TestHasFeature(t *testing.T) {
	features := []string{"CATEGORY_A", "CATEGORY_C"}
	assert.True(t, HasFeature(features, "CATEGORY_A"))
	assert.True(t, HasFeature(features, "CATEGORY_C"))
	assert.False(t, HasFeature(features, "CATEGORY_B"))
	assert.False(t, HasFeature(nil, "CATEGORY_A"))
	assert.False(t, HasFeature([]string{}, ""))
}

// Entitlements are the union across active subscriptions, so two à-la-carte
// plans must grant the same access as one bundled plan — the property that
// justified a list over a tier integer.
func TestUserFeatureAccess_UnionAndExpiry(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		stamp := time.Now().Format("150405.000000")

		planA := models.Plan{Name: "A " + stamp, PriceMinor: 1000, Features: []string{"CATEGORY_A"}, DurationDays: 365}
		planB := models.Plan{Name: "B " + stamp, PriceMinor: 1000, Features: []string{"CATEGORY_B"}, DurationDays: 365}
		planC := models.Plan{Name: "C " + stamp, PriceMinor: 1000, Features: []string{"CATEGORY_C"}, DurationDays: 365}
		require.NoError(t, tx.Create(&planA).Error)
		require.NoError(t, tx.Create(&planB).Error)
		require.NoError(t, tx.Create(&planC).Error)

		user := models.User{Name: "Feature User", Email: "features-" + stamp + "@example.com"}
		require.NoError(t, tx.Create(&user).Error)

		future := time.Now().Add(30 * 24 * time.Hour)
		past := time.Now().Add(-24 * time.Hour)

		require.NoError(t, tx.Create(&models.Subscription{
			UserID: user.ID, PlanID: planA.ID, ExpiryDate: future, Status: models.SubscriptionActive,
		}).Error)
		require.NoError(t, tx.Create(&models.Subscription{
			UserID: user.ID, PlanID: planB.ID, ExpiryDate: future, Status: models.SubscriptionActive,
		}).Error)
		// Expired — must contribute nothing.
		require.NoError(t, tx.Create(&models.Subscription{
			UserID: user.ID, PlanID: planC.ID, ExpiryDate: past, Status: models.SubscriptionActive,
		}).Error)

		features := UserFeatureAccess(user.ID)
		assert.ElementsMatch(t, []string{"CATEGORY_A", "CATEGORY_B"}, features)
		assert.False(t, HasFeature(features, "CATEGORY_C"), "expired subscription must not grant access")
	})
}

func TestUserFeatureAccess_NoSubscriptions(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		stamp := time.Now().Format("150405.000000")

		user := models.User{Name: "No Sub", Email: "nosub-" + stamp + "@example.com"}
		require.NoError(t, tx.Create(&user).Error)

		features := UserFeatureAccess(user.ID)
		assert.NotNil(t, features, "must be an empty slice, never nil")
		assert.Empty(t, features)
	})
}

func TestSubscriptionData_EmitsFeaturesArray(t *testing.T) {
	sub := &models.Subscription{ID: "sub-1", PlanID: "plan-1", Status: models.SubscriptionActive}
	plan := &models.Plan{Name: "All Access", Features: []string{"CATEGORY_A", "CATEGORY_B"}}

	data := SubscriptionData(sub, plan)
	assert.Equal(t, []string{"CATEGORY_A", "CATEGORY_B"}, data["features"])

	// A plan with no features must serialize as [] so clients can iterate it.
	empty := SubscriptionData(sub, &models.Plan{Name: "Free"})
	assert.Equal(t, []string{}, empty["features"])
	assert.NotNil(t, empty["features"])
}
