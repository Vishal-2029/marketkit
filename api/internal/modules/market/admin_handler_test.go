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

// TestHandleAdminMarketUserProducts_ShowsPurchasesForBuyerOnlyUser covers
// Task 2's acceptance criteria: a user who has never listed a product still
// gets their full purchase history back (previously the UI's own
// product_count > 0 guard meant this drill-down was unreachable, but the
// endpoint itself already returned an empty products list either way).
func TestHandleAdminMarketUserProducts_ShowsPurchasesForBuyerOnlyUser(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB

		seller := testutil.MustCreateUser(t, tx)
		buyer := testutil.MustCreateUser(t, tx) // never lists a product
		product := testutil.MustCreateProduct(t, tx, seller.ID, 50000)
		testutil.MustCreateProductPurchase(t, tx, product.ID, buyer.ID, seller.ID, 50000, 5000, models.PaymentSuccess)

		app := testutil.FiberApp(nil)
		app.Get("/x/:id/products", HandleAdminMarketUserProducts)

		resp, err := app.Test(httptest.NewRequest("GET", "/x/"+buyer.ID+"/products", nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data struct {
				ProductsSold []MarketUserProductRow  `json:"products_sold"`
				Purchases    []MarketUserPurchaseRow `json:"purchases"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))

		assert.Empty(t, parsed.Data.ProductsSold, "buyer never listed a product")
		require.Len(t, parsed.Data.Purchases, 1)
		assert.Equal(t, product.Title, parsed.Data.Purchases[0].ProductTitle)
		assert.Equal(t, int64(50000), parsed.Data.Purchases[0].AmountInPaise)
		assert.Equal(t, int64(5000), parsed.Data.Purchases[0].FeeInPaise)
		assert.Equal(t, seller.Name, parsed.Data.Purchases[0].SellerName)
	})
}
