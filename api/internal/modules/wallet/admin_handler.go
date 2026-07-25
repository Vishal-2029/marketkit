package wallet

import (
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleAdminListWithdrawals godoc
// @Summary     List withdrawal requests (admin)
// @Tags        Admin Wallet
// @Produce     json
// @Security    BearerAuth
// @Param       page    query  int     false  "Page"
// @Param       limit   query  int     false  "Limit"
// @Param       status  query  string  false  "APPROVED | SETTLED"
// @Success     200  {object}  []models.Withdrawal
// @Failure     401  {object}  map[string]string
// @Router      /wallet/withdrawals [get]
func HandleAdminListWithdrawals(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.Withdrawal{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Count(&total)

	var ws []models.Withdrawal
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&ws).Error; err != nil {
		return response.InternalError(c, "failed to fetch withdrawals")
	}
	if ws == nil {
		ws = []models.Withdrawal{}
	}

	// Populate requester name/email (same query-time pattern as the market
	// admin purchases list). Unscoped so soft-deleted users still resolve.
	userIDs := make([]string, 0, len(ws))
	for _, w := range ws {
		userIDs = append(userIDs, w.UserID)
	}
	if len(userIDs) > 0 {
		var users []models.User
		database.DB.Unscoped().Select("id", "name", "email").
			Where("id IN ?", userIDs).Find(&users)
		byID := make(map[string]models.User, len(users))
		for _, u := range users {
			byID[u.ID] = u
		}
		for i := range ws {
			if u, okUser := byID[ws[i].UserID]; okUser {
				ws[i].UserName = u.Name
				ws[i].UserEmail = u.Email
			}
		}
	}

	return response.Paginated(c, ws, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleAdminSettle godoc
// @Summary     Mark a withdrawal as settled after transferring the money manually
// @Tags        Admin Wallet
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id    path  string             true   "Withdrawal ID"
// @Param       body  body  map[string]string  false  "note"
// @Success     200  {object}  models.Withdrawal
// @Failure     400  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /wallet/withdrawals/{id}/settle [post]
func HandleAdminSettle(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", adminID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins can settle withdrawals")
	}

	var w models.Withdrawal
	if err := database.DB.First(&w, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "withdrawal not found")
	}
	if w.Status != models.WithdrawalStatusApproved {
		return response.BadRequest(c, "only approved withdrawals can be settled")
	}

	var body struct {
		Note string `json:"note"`
	}
	_ = c.BodyParser(&body)

	now := time.Now()
	if err := database.DB.Model(&w).Updates(map[string]interface{}{
		"status":     models.WithdrawalStatusSettled,
		"settled_by": adminID,
		"settled_at": now,
		"note":       body.Note,
	}).Error; err != nil {
		return response.InternalErrorWithLog(c, "wallet: settle withdrawal", err)
	}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &adminID,
		TargetID:     &w.UserID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":        "withdrawal_settled",
			"withdrawal_id": w.ID,
			"amount":        w.AmountInPaise,
			"method":        w.Method,
		},
	})

	go func(userID string, amount int64) {
		_ = fcm.SendToUser(userID, "Payout Sent",
			"Your withdrawal of ₹"+strconv.FormatInt(amount/100, 10)+" has been transferred.")
	}(w.UserID, w.AmountInPaise)

	return response.OK(c, w)
}

// HandleGetSettings godoc
// @Summary     Wallet platform settings (admin)
// @Tags        Admin Wallet
// @Produce     json
// @Security    BearerAuth
// @Success     200  {object}  map[string]int64
// @Router      /wallet/settings [get]
func HandleGetSettings(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"fee_percent":             FeePercent(),
		"min_withdrawal_in_paise": MinWithdrawal(),
	})
}

// HandleUpdateSettings godoc
// @Summary     Update wallet platform settings (super admin)
// @Tags        Admin Wallet
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body  body  map[string]int64  true  "fee_percent, min_withdrawal_in_paise"
// @Success     200  {object}  map[string]int64
// @Failure     400  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Router      /wallet/settings [put]
func HandleUpdateSettings(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins can change platform settings")
	}

	var body struct {
		FeePercent           int64 `json:"fee_percent"`
		MinWithdrawalInPaise int64 `json:"min_withdrawal_in_paise"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.FeePercent < 0 || body.FeePercent > 50 {
		return response.BadRequest(c, "fee_percent must be between 0 and 50")
	}
	if body.MinWithdrawalInPaise < 0 {
		return response.BadRequest(c, "min_withdrawal_in_paise must not be negative")
	}

	for key, value := range map[string]int64{
		models.SettingMarketFeePercent:   body.FeePercent,
		models.SettingMinWithdrawalPaise: body.MinWithdrawalInPaise,
	} {
		if err := database.DB.Model(&models.PlatformSetting{}).
			Where("key = ?", key).
			Update("value", strconv.FormatInt(value, 10)).Error; err != nil {
			return response.InternalErrorWithLog(c, "wallet: update settings", err)
		}
	}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":                  "wallet_settings_updated",
			"fee_percent":             body.FeePercent,
			"min_withdrawal_in_paise": body.MinWithdrawalInPaise,
		},
	})

	return response.OK(c, fiber.Map{
		"fee_percent":             body.FeePercent,
		"min_withdrawal_in_paise": body.MinWithdrawalInPaise,
	})
}

// HandleAdminUserTransactions godoc
// @Summary     Paginated wallet ledger for a specific user (admin)
// @Tags        Admin Wallet
// @Produce     json
// @Security    BearerAuth
// @Param       id     path   string  true   "User ID"
// @Param       page   query  int     false  "Page"
// @Param       limit  query  int     false  "Limit"
// @Success     200  {object}  []models.WalletTransaction
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /wallet/users/{id}/transactions [get]
func HandleAdminUserTransactions(c *fiber.Ctx) error {
	userID := c.Params("id")
	var user models.User
	if err := database.DB.Unscoped().Select("id").First(&user, "id = ?", userID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}
	return listUserTransactions(c, userID)
}
