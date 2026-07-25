package market

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

// TestHandleAdminMarketUserDesigns_ShowsPurchasesForBuyerOnlyUser covers
// Task 2's acceptance criteria: a user who has never listed a design still
// gets their full purchase history back (previously the UI's own
// design_count > 0 guard meant this drill-down was unreachable, but the
// endpoint itself already returned an empty designs list either way).
func TestHandleAdminMarketUserDesigns_ShowsPurchasesForBuyerOnlyUser(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx) // never lists a design
		design := testutil.MustCreateDesign(t, tx, seller.ID, 50000)
		testutil.MustCreateDesignPurchase(t, tx, design.ID, buyer.ID, seller.ID, 50000, 5000, models.PaymentSuccess)

		app := testutil.FiberApp(nil)
		app.Get("/x/:id/designs", HandleAdminMarketUserDesigns)

		resp, err := app.Test(httptest.NewRequest("GET", "/x/"+buyer.ID+"/designs", nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data struct {
				DesignsSold []MarketUserDesignRow   `json:"designs_sold"`
				Purchases   []MarketUserPurchaseRow `json:"purchases"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))

		assert.Empty(t, parsed.Data.DesignsSold, "buyer never listed a design")
		require.Len(t, parsed.Data.Purchases, 1)
		assert.Equal(t, design.Title, parsed.Data.Purchases[0].DesignTitle)
		assert.Equal(t, int64(50000), parsed.Data.Purchases[0].AmountInPaise)
		assert.Equal(t, int64(5000), parsed.Data.Purchases[0].FeeInPaise)
		assert.Equal(t, seller.Name, parsed.Data.Purchases[0].SellerName)
	})
}
