package testutil

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marketkit/api/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seq gives every fixture a unique suffix within a test run so uniqueIndex
// columns (email, plan name, ...) never collide across fixtures created in
// the same rolled-back transaction.
var seq int64

func nextSeq() int64 { return atomic.AddInt64(&seq, 1) }

func MustCreateUser(t *testing.T, tx *gorm.DB) models.User {
	t.Helper()
	n := nextSeq()
	u := models.User{
		Name:  "Test User",
		Email: "test-user-" + strconv.FormatInt(n, 10) + "@example.com",
	}
	require.NoError(t, tx.Create(&u).Error)
	return u
}

func MustCreateAdmin(t *testing.T, tx *gorm.DB, isSuper bool) models.Admin {
	t.Helper()
	n := nextSeq()
	a := models.Admin{
		FirstName: "Test",
		LastName:  "Admin",
		Email:     "test-admin-" + strconv.FormatInt(n, 10) + "@example.com",
		IsActive:  true,
		IsSuper:   isSuper,
	}
	require.NoError(t, tx.Create(&a).Error)
	return a
}

func MustCreatePlan(t *testing.T, tx *gorm.DB, priceInPaise int64) models.Plan {
	t.Helper()
	n := nextSeq()
	p := models.Plan{
		Name:         "Test Plan " + strconv.FormatInt(n, 10),
		PriceInPaise: priceInPaise,
		DurationDays: 365,
		IsActive:     true,
	}
	require.NoError(t, tx.Create(&p).Error)
	return p
}

func MustCreatePayment(t *testing.T, tx *gorm.DB, userID, planID string, amountInPaise int64, status models.PaymentStatus) models.Payment {
	t.Helper()
	now := time.Now()
	p := models.Payment{
		UserID:        userID,
		PlanID:        planID,
		AmountInPaise: amountInPaise,
		Gateway:       models.GatewayRazorpay,
		Status:        status,
	}
	if status == models.PaymentSuccess {
		p.PaidAt = &now
	}
	require.NoError(t, tx.Create(&p).Error)
	return p
}

func MustCreateMarketPlan(t *testing.T, tx *gorm.DB, priceInPaise int64) models.MarketPlan {
	t.Helper()
	n := nextSeq()
	p := models.MarketPlan{
		Name:         "Test Market Plan " + strconv.FormatInt(n, 10),
		PriceInPaise: priceInPaise,
		DurationDays: 30,
		IsActive:     true,
	}
	require.NoError(t, tx.Create(&p).Error)
	return p
}

// MustCreateMarketPlanSub creates a market plan subscription. When paid is
// true it's ACTIVE with PaidAt set (as activateMarketPlanSub/the wallet path
// would leave it); otherwise it's an unpaid PENDING row.
func MustCreateMarketPlanSub(t *testing.T, tx *gorm.DB, userID, planID string, amountInPaise int64, paid bool) models.MarketPlanSubscription {
	t.Helper()
	now := time.Now()
	sub := models.MarketPlanSubscription{
		UserID:        userID,
		PlanID:        planID,
		AmountInPaise: amountInPaise,
		StartDate:     now,
		ExpiryDate:    now.AddDate(0, 0, 30),
	}
	if paid {
		sub.Status = models.SubscriptionActive
		sub.PaidAt = &now
	} else {
		sub.Status = models.SubscriptionPending
	}
	require.NoError(t, tx.Create(&sub).Error)
	return sub
}

func MustCreateProduct(t *testing.T, tx *gorm.DB, sellerID string, priceInPaise int64) models.Product {
	t.Helper()
	n := nextSeq()
	d := models.Product{
		SellerID:     sellerID,
		Title:        "Test Product " + strconv.FormatInt(n, 10),
		PriceInPaise: priceInPaise,
		FileKey:      "products/test-" + strconv.FormatInt(n, 10) + ".zip",
		FileName:     "test-" + strconv.FormatInt(n, 10) + ".zip",
		IsActive:     true,
	}
	require.NoError(t, tx.Create(&d).Error)
	return d
}

func MustCreateProductPurchase(t *testing.T, tx *gorm.DB, productID, buyerID, sellerID string, amountInPaise, feeInPaise int64, status models.PaymentStatus) models.ProductPurchase {
	t.Helper()
	now := time.Now()
	p := models.ProductPurchase{
		ProductID:     productID,
		BuyerID:       buyerID,
		SellerID:      sellerID,
		AmountInPaise: amountInPaise,
		Status:        status,
	}
	if status == models.PaymentSuccess {
		p.FeeInPaise = feeInPaise
		p.SellerNetInPaise = amountInPaise - feeInPaise
		p.PaidAt = &now
	}
	require.NoError(t, tx.Create(&p).Error)
	return p
}
