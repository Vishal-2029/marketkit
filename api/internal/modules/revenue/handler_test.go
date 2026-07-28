package revenue

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.RunMain(m) }

// TestHandleSummary_ExcludesProductMarketMoney is the cross-cutting isolation
// guarantee: no product-market money (product sale, market-plan payment) may
// ever leak into the learning Revenue page.
func TestHandleSummary_ExcludesProductMarketMoney(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		user := testutil.MustCreateUser(t, tx)
		plan := testutil.MustCreatePlan(t, tx, 99900)
		testutil.MustCreatePayment(t, tx, user.ID, plan.ID, 99900, models.PaymentSuccess)

		marketPlan := testutil.MustCreateMarketPlan(t, tx, 49900)
		testutil.MustCreateMarketPlanSub(t, tx, user.ID, marketPlan.ID, 49900, true)

		product := testutil.MustCreateProduct(t, tx, user.ID, 20000)
		testutil.MustCreateProductPurchase(t, tx, product.ID, user.ID, user.ID, 20000, 2000, models.PaymentSuccess)

		app := testutil.FiberApp(nil)
		app.Get("/x", HandleSummary)

		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data struct {
				TotalRevenueMinor int64 `json:"total_revenue_minor"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))
		assert.Equal(t, int64(99900), parsed.Data.TotalRevenueMinor,
			"learning revenue must equal only the learning payment — market-plan and product-sale money must not leak in")
	})
}
