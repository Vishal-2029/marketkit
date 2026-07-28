package market

import (
	"testing"
	"time"

	"github.com/marketkit/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildInvoicePDF_ProducesValidPDF proves the bill/invoice generated on
// every product purchase (wallet, Razorpay verify, Razorpay webhook all call
// sendPurchaseEmailAsync -> buildInvoicePDF, and the download endpoint
// HandleDownloadInvoice calls it too) actually renders without error and
// yields a real PDF.
func TestBuildInvoicePDF_ProducesValidPDF(t *testing.T) {
	now := time.Now()
	purchase := &models.ProductPurchase{
		ID:             "test-purchase-id",
		AmountMinor:    400000,
		FeeMinor:       40000,
		SellerNetMinor: 360000,
		PaidVia:        "WALLET",
		Status:         models.PaymentSuccess,
		PaidAt:         &now,
		CreatedAt:      now,
		Buyer:          models.User{Name: "Test Buyer", Email: "buyer@example.com"},
		Seller:         models.User{Name: "Test Seller", Email: "seller@example.com"},
		Product: models.Product{
			Title:    "Sample Product Pack",
			FileName: "sample-pack.zip",
			// No PreviewKeys, so buildInvoicePDF skips the network image fetch.
		},
	}

	pdf, err := buildInvoicePDF(purchase)
	require.NoError(t, err, "invoice PDF generation must not error")
	require.NotEmpty(t, pdf)
	// A real PDF stream begins with the %PDF- magic bytes.
	assert.Equal(t, "%PDF-", string(pdf[:5]), "output must be a valid PDF")
	assert.Greater(t, len(pdf), 500, "a rendered invoice should be more than a stub")
}
