package market

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
)

const maxEmailAttachmentBytes = 20 * 1024 * 1024 // stay under common SMTP limits

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("content exceeds %d bytes", limit)
	}
	return data, nil
}

// sendPurchaseEmailAsync loads the purchase with relations, builds the invoice
// PDF, optionally attaches the design file (or falls back to a signed link),
// and emails the buyer. Runs in a goroutine — never blocks the purchase response.
func sendPurchaseEmailAsync(purchaseID string) {
	go func() {
		var purchase models.DesignPurchase
		if err := database.DB.
			Preload("Design").
			Preload("Buyer").
			Preload("Seller").
			First(&purchase, "id = ?", purchaseID).Error; err != nil {
			slog.Error("market: purchase email load failed", "purchase_id", purchaseID, "error", err)
			return
		}
		if purchase.Buyer.Email == "" {
			return
		}

		pdfBytes, err := buildInvoicePDF(&purchase)
		if err != nil {
			slog.Error("market: purchase email invoice failed", "purchase_id", purchaseID, "error", err)
			return
		}

		paidAt := purchase.CreatedAt
		if purchase.PaidAt != nil {
			paidAt = *purchase.PaidAt
		}
		previewURL := ""
		if len(purchase.Design.PreviewKeys) > 0 {
			previewURL = storage.Store.PublicURL(purchase.Design.PreviewKeys[0])
		}

		data := email.MarketPurchaseEmailData{
			Name:         purchase.Buyer.Name,
			DesignTitle:  purchase.Design.Title,
			SellerName:   purchase.Seller.Name,
			Amount:       email.FormatAmount(purchase.AmountInPaise),
			PaidVia:      purchase.PaidVia,
			PaidAt:       email.FormatDate(paidAt),
			PurchaseID:   purchase.ID,
			PreviewURL:   previewURL,
			DownloadLink: "",
		}

		var designBytes []byte
		designFileName := purchase.Design.FileName
		if designFileName == "" {
			designFileName = "design-file"
		}

		url, err := storage.Store.SignedURL(context.Background(), purchase.Design.FileKey, 7*24*time.Hour)
		if err != nil {
			slog.Error("market: purchase email signed url failed", "purchase_id", purchaseID, "error", err)
		} else {
			resp, err := httpClient().Get(url)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					body, err := readAllLimited(resp.Body, maxEmailAttachmentBytes)
					if err == nil {
						designBytes = body
					} else {
						// Too large (or read failed) — put a recoverable link in the email.
						data.DownloadLink = url
					}
				} else {
					data.DownloadLink = url
				}
			} else {
				data.DownloadLink = url
			}
		}

		if err := email.SendMarketPurchaseEmail(
			purchase.Buyer.Email, data, pdfBytes, designBytes, designFileName,
		); err != nil {
			slog.Error("market: purchase email send failed", "purchase_id", purchaseID, "error", err)
		}
	}()
}
