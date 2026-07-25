package platform_wallet

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// requireSuperAdmin writes a 403 and returns ok=false when the caller isn't a
// super-admin; handlers should `return err` immediately when ok is false.
// Mirrors the inline check in wallet.HandleAdminSettle / HandleUpdateSettings.
func requireSuperAdmin(c *fiber.Ctx) (ok bool, err error) {
	adminID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if dbErr := database.DB.First(&actor, "id = ?", adminID).Error; dbErr != nil {
		return false, response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return false, response.Forbidden(c, "only super-admins can access the platform wallet")
	}
	return true, nil
}

// HandleGet godoc
// @Summary     Platform wallet balance and per-source breakdown (super-admin)
// @Tags        Admin Platform Wallet
// @Produce     json
// @Security    BearerAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     403  {object}  map[string]string
// @Router      /platform-wallet [get]
func HandleGet(c *fiber.Ctx) error {
	if ok, err := requireSuperAdmin(c); !ok {
		return err
	}

	var w models.PlatformWallet
	if err := database.DB.First(&w, "id = ?", models.PlatformWalletSingletonID).Error; err != nil {
		return response.InternalErrorWithLog(c, "platform_wallet: load balance", err)
	}

	type sourceTotal struct {
		Source string
		Total  int64
	}
	var totals []sourceTotal
	if err := database.DB.Model(&models.PlatformLedger{}).
		Select("source, COALESCE(SUM(amount_in_paise), 0) AS total").
		Group("source").
		Scan(&totals).Error; err != nil {
		return response.InternalErrorWithLog(c, "platform_wallet: load breakdown", err)
	}

	breakdown := fiber.Map{
		"learning_plan_in_paise": int64(0),
		"market_plan_in_paise":   int64(0),
		"platform_fee_in_paise":  int64(0),
		"withdrawal_in_paise":    int64(0),
	}
	for _, t := range totals {
		switch t.Source {
		case models.PlatformSourceLearningPlan:
			breakdown["learning_plan_in_paise"] = t.Total
		case models.PlatformSourceMarketPlan:
			breakdown["market_plan_in_paise"] = t.Total
		case models.PlatformSourcePlatformFee:
			breakdown["platform_fee_in_paise"] = t.Total
		case models.PlatformSourceWithdrawal:
			breakdown["withdrawal_in_paise"] = t.Total
		}
	}

	return response.OK(c, fiber.Map{
		"balance_in_paise": w.BalanceInPaise,
		"breakdown":        breakdown,
	})
}

// HandleTransactions godoc
// @Summary     Paginated platform ledger, or a full CSV export (super-admin)
// @Tags        Admin Platform Wallet
// @Produce     json
// @Security    BearerAuth
// @Param       page    query  int     false  "Page"
// @Param       limit   query  int     false  "Limit"
// @Param       format  query  string  false  "csv for a full unpaginated export"
// @Success     200  {object}  []models.PlatformLedger
// @Failure     403  {object}  map[string]string
// @Router      /platform-wallet/transactions [get]
func HandleTransactions(c *fiber.Ctx) error {
	if ok, err := requireSuperAdmin(c); !ok {
		return err
	}

	switch c.Query("format") {
	case "csv":
		return exportCSV(c)
	case "pdf":
		return exportPDF(c)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.PlatformLedger{})
	var total int64
	q.Count(&total)

	var rows []models.PlatformLedger
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error; err != nil {
		return response.InternalError(c, "failed to fetch platform ledger")
	}
	if rows == nil {
		rows = []models.PlatformLedger{}
	}

	return response.Paginated(c, rows, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// exportCSV writes the full, unpaginated ledger as a downloadable CSV.
func exportCSV(c *fiber.Ctx) error {
	var rows []models.PlatformLedger
	if err := database.DB.Order("created_at DESC").Find(&rows).Error; err != nil {
		return response.InternalError(c, "failed to export platform ledger")
	}

	var buf bytes.Buffer
	wr := csv.NewWriter(&buf)
	_ = wr.Write([]string{"date", "type", "source", "reference_id", "amount_in_paise", "balance_after_in_paise"})
	for _, r := range rows {
		ref := ""
		if r.ReferenceID != nil {
			ref = *r.ReferenceID
		}
		_ = wr.Write([]string{
			r.CreatedAt.Format(time.RFC3339),
			r.Type,
			r.Source,
			ref,
			strconv.FormatInt(r.AmountInPaise, 10),
			strconv.FormatInt(r.BalanceAfterInPaise, 10),
		})
	}
	wr.Flush()

	filename := fmt.Sprintf("platform-wallet-transactions-%s.csv", time.Now().Format("20060102"))
	c.Set(fiber.HeaderContentType, "text/csv")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.SendString(buf.String())
}

func formatPaise(paise int64) string {
	rupees := float64(paise) / 100
	sign := ""
	if rupees < 0 {
		sign = "-"
		rupees = -rupees
	}
	if rupees == float64(int64(rupees)) {
		return fmt.Sprintf("%sRs. %d", sign, int64(rupees))
	}
	return fmt.Sprintf("%sRs. %.2f", sign, rupees)
}

var sourceLabel = map[string]string{
	models.PlatformSourceLearningPlan: "Learning plan",
	models.PlatformSourceMarketPlan:   "Market plan",
	models.PlatformSourcePlatformFee:  "Product sale fee",
	models.PlatformSourceWithdrawal:   "Withdrawal",
}

// exportPDF writes the full, unpaginated ledger as a formatted statement PDF,
// reusing the same fpdf library the market module already uses for invoices.
func exportPDF(c *fiber.Ctx) error {
	var w models.PlatformWallet
	if err := database.DB.First(&w, "id = ?", models.PlatformWalletSingletonID).Error; err != nil {
		return response.InternalError(c, "failed to load platform wallet")
	}
	var rows []models.PlatformLedger
	if err := database.DB.Order("created_at DESC").Find(&rows).Error; err != nil {
		return response.InternalError(c, "failed to export platform ledger")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 12, "Platform Wallet Statement", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated: %s", time.Now().Format("02 Jan 2006, 15:04")), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Current Balance: %s", formatPaise(w.BalanceInPaise)), "", 1, "L", false, 0, "")
	pdf.Ln(6)

	colWidths := []float64{28, 40, 40, 40, 40}
	headers := []string{"Date", "Type", "Source", "Amount", "Balance"}
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 8)
	for _, r := range rows {
		label := sourceLabel[r.Source]
		if label == "" {
			label = r.Source
		}
		pdf.CellFormat(colWidths[0], 6, r.CreatedAt.Format("02 Jan 2006"), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 6, r.Type, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 6, label, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[3], 6, formatPaise(r.AmountInPaise), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[4], 6, formatPaise(r.BalanceAfterInPaise), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return response.InternalErrorWithLog(c, "platform_wallet: render pdf", err)
	}

	filename := fmt.Sprintf("platform-wallet-transactions-%s.pdf", time.Now().Format("20060102"))
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

// HandleCreateWithdrawal godoc
// @Summary     Record a super-admin withdrawal from the platform wallet
// @Tags        Admin Platform Wallet
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body  body  map[string]interface{}  true  "amount_in_paise, note"
// @Success     201  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Router      /platform-wallet/withdrawals [post]
func HandleCreateWithdrawal(c *fiber.Ctx) error {
	ok, err := requireSuperAdmin(c)
	if !ok {
		return err
	}
	adminID, _ := c.Locals("adminID").(string)

	var body struct {
		AmountInPaise int64  `json:"amount_in_paise"`
		Note          string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.AmountInPaise <= 0 {
		return response.BadRequest(c, "amount must be greater than 0")
	}

	var newBalance int64
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		var applyErr error
		newBalance, applyErr = Apply(tx, models.PlatformSourceWithdrawal, -body.AmountInPaise, nil,
			models.JSONMap{"withdrawn_by": adminID, "note": body.Note})
		return applyErr
	})
	if txErr == ErrInsufficientBalance {
		return response.BadRequest(c, "insufficient platform wallet balance")
	}
	if txErr != nil {
		return response.InternalErrorWithLog(c, "platform_wallet: withdraw", txErr)
	}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &adminID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":          "platform_wallet_withdrawal",
			"amount_in_paise": body.AmountInPaise,
			"note":            body.Note,
		},
	})

	return response.Created(c, fiber.Map{
		"message":          "withdrawal recorded",
		"amount_in_paise":  body.AmountInPaise,
		"balance_in_paise": newBalance,
	})
}
