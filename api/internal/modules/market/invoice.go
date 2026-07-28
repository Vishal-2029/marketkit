package market

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/money"
	"github.com/marketkit/api/pkg/response"
)

// formatMinor renders an amount for the PDF. It uses the ISO code rather than
// the currency symbol because fpdf's core fonts are Latin-1 and cannot draw ₹.
func formatMinor(minor int64) string {
	return money.FormatASCII(minor, config.App.PaymentCurrency)
}

// fetchPreviewImageBytes downloads a product's first preview image over HTTP
// from its public URL so it can be embedded in the invoice PDF. Preview
// images are always stored as JPEG (see uploadPreviewImage).
func fetchPreviewImageBytes(url string) ([]byte, error) {
	client := httpClient()
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d fetching preview image", resp.StatusCode)
	}
	return readAllLimited(resp.Body, 5*1024*1024)
}

// buildInvoicePDF renders the purchase invoice PDF. Used by the download
// endpoint and the post-purchase thank-you email.
func buildInvoicePDF(purchase *models.ProductPurchase) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 12, "Purchase Invoice", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 6, fmt.Sprintf("Invoice ID: %s", purchase.ID), "", 1, "L", false, 0, "")
	paidAt := purchase.CreatedAt
	if purchase.PaidAt != nil {
		paidAt = *purchase.PaidAt
	}
	pdf.CellFormat(0, 6, fmt.Sprintf("Date: %s", paidAt.Format("02 Jan 2006, 15:04")), "", 1, "L", false, 0, "")
	pdf.Ln(6)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(95, 8, "Buyer", "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 8, "Seller", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(95, 6, purchase.Buyer.Name, "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 6, purchase.Seller.Name, "", 1, "L", false, 0, "")
	pdf.CellFormat(95, 6, purchase.Buyer.Email, "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 6, purchase.Seller.Email, "", 1, "L", false, 0, "")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Product", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, purchase.Product.Title, "", 1, "L", false, 0, "")

	if len(purchase.Product.PreviewKeys) > 0 {
		previewURL := storage.Store.PublicURL(purchase.Product.PreviewKeys[0])
		if imgBytes, err := fetchPreviewImageBytes(previewURL); err == nil {
			imgName := "preview"
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "JPEG"}, bytes.NewReader(imgBytes))
			pdf.ImageOptions(imgName, 0, pdf.GetY()+4, 60, 0, false, fpdf.ImageOptions{ImageType: "JPEG"}, 0, "")
			pdf.Ln(50)
		}
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Payment", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(95, 6, "Amount Paid", "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 6, formatMinor(purchase.AmountMinor), "", 1, "L", false, 0, "")
	pdf.CellFormat(95, 6, "Payment Method", "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 6, purchase.PaidVia, "", 1, "L", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(120, 120, 120)
	// fpdf renders text as Latin-1, so keep this string ASCII-only — a
	// multi-byte em-dash (—) would come out as garbage ("â€"") in the PDF.
	pdf.MultiCell(0, 5,
		"This sale is final - no returns or refunds are processed through the app. "+
			"If there's an issue with the product you purchased, please contact support "+
			"from the product's page instead of requesting a return.",
		"", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// HandleDownloadInvoice godoc
// @Summary     Download a purchase invoice PDF
// @Tags        User Market
// @Produce     application/pdf
// @Security    UserAuth
// @Param       id  path  string  true  "Purchase ID"
// @Success     200  {file}    binary
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/purchases/{id}/invoice [get]
func HandleDownloadInvoice(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var purchase models.ProductPurchase
	if err := database.DB.
		Preload("Product").
		Preload("Buyer").
		Preload("Seller").
		Where("id = ? AND buyer_id = ? AND status = ?", c.Params("id"), userID, models.PaymentSuccess).
		First(&purchase).Error; err != nil {
		return response.NotFound(c, "purchase not found")
	}

	pdfBytes, err := buildInvoicePDF(&purchase)
	if err != nil {
		return response.InternalError(c, "failed to generate invoice")
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, purchase.ID))
	return c.Send(pdfBytes)
}
