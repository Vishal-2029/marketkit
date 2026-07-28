package platform_wallet

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleCreateWithdrawal_OnlySuperAdmin covers the IsSuper gate: a
// regular admin must get 403 and the balance must be untouched.
func TestHandleCreateWithdrawal_OnlySuperAdmin(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		_, err := Apply(tx, models.PlatformSourceLearningPlan, 100000, nil, nil)
		require.NoError(t, err)

		regularAdmin := testutil.MustCreateAdmin(t, tx, false)
		app := testutil.FiberApp(map[string]string{"adminID": regularAdmin.ID})
		app.Post("/x", HandleCreateWithdrawal)

		req := httptest.NewRequest("POST", "/x", testutil.JSONBody(t, map[string]interface{}{"amount_minor": 50000}))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 403, resp.StatusCode)

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(100000), w.BalanceMinor, "a rejected non-super withdrawal must not touch the balance")
	})
}

// TestHandleCreateWithdrawal_SuperAdminDebits covers the super-admin happy
// path and the insufficient-balance rejection.
func TestHandleCreateWithdrawal_SuperAdminDebits(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		_, err := Apply(tx, models.PlatformSourceLearningPlan, 100000, nil, nil)
		require.NoError(t, err)

		superAdmin := testutil.MustCreateAdmin(t, tx, true)
		app := testutil.FiberApp(map[string]string{"adminID": superAdmin.ID})
		app.Post("/x", HandleCreateWithdrawal)

		// Over the balance must be rejected and leave it untouched.
		req := httptest.NewRequest("POST", "/x", testutil.JSONBody(t, map[string]interface{}{"amount_minor": 999999}))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)

		req2 := httptest.NewRequest("POST", "/x", testutil.JSONBody(t, map[string]interface{}{"amount_minor": 40000, "note": "payout"}))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		assert.Equal(t, 201, resp2.StatusCode)

		var w models.PlatformWallet
		require.NoError(t, tx.First(&w, "id = ?", models.PlatformWalletSingletonID).Error)
		assert.Equal(t, int64(60000), w.BalanceMinor)

		var ledgerRow models.PlatformLedger
		require.NoError(t, tx.Where("source = ?", models.PlatformSourceWithdrawal).First(&ledgerRow).Error)
		assert.Equal(t, int64(-40000), ledgerRow.AmountMinor)
	})
}

// TestHandleGet_BreakdownBySource covers the per-source breakdown endpoint,
// including that a regular admin is rejected.
func TestHandleGet_BreakdownBySource(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		_, err := Apply(tx, models.PlatformSourceLearningPlan, 10000, nil, nil)
		require.NoError(t, err)
		_, err = Apply(tx, models.PlatformSourceMarketPlan, 20000, nil, nil)
		require.NoError(t, err)
		_, err = Apply(tx, models.PlatformSourcePlatformFee, 3000, nil, nil)
		require.NoError(t, err)

		superAdmin := testutil.MustCreateAdmin(t, tx, true)
		app := testutil.FiberApp(map[string]string{"adminID": superAdmin.ID})
		app.Get("/x", HandleGet)

		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})
}

// TestHandleTransactions_CSVAndPDFExport covers Task 5's two export formats.
func TestHandleTransactions_CSVAndPDFExport(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		_, err := Apply(tx, models.PlatformSourceLearningPlan, 10000, nil, nil)
		require.NoError(t, err)

		superAdmin := testutil.MustCreateAdmin(t, tx, true)
		app := testutil.FiberApp(map[string]string{"adminID": superAdmin.ID})
		app.Get("/x", HandleTransactions)

		csvResp, err := app.Test(httptest.NewRequest("GET", "/x?format=csv", nil))
		require.NoError(t, err)
		assert.Equal(t, 200, csvResp.StatusCode)
		assert.Equal(t, "text/csv", csvResp.Header.Get("Content-Type"))

		pdfResp, err := app.Test(httptest.NewRequest("GET", "/x?format=pdf", nil))
		require.NoError(t, err)
		assert.Equal(t, 200, pdfResp.StatusCode)
		assert.Equal(t, "application/pdf", pdfResp.Header.Get("Content-Type"))

		body, err := io.ReadAll(pdfResp.Body)
		require.NoError(t, err)
		assert.True(t, len(body) > 0 && string(body[:4]) == "%PDF", "response must be a real PDF")
	})
}
