package wallet

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

const (
	minTopupInPaise = 1000     // ₹10
	maxTopupInPaise = 10000000 // ₹1,00,000
)

var (
	upiRegex     = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,}@[a-zA-Z]{2,}$`)
	ifscRegex    = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
	accountRegex = regexp.MustCompile(`^[0-9]{9,18}$`)
)

// HandleSummary godoc
// @Summary     Wallet balance and payout-method availability
// @Tags        User Wallet
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet [get]
func HandleSummary(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var user models.User
	if err := database.DB.Select("id", "wallet_balance_in_paise").
		First(&user, "id = ?", userID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	var pd models.PayoutDetail
	database.DB.Where("user_id = ?", userID).First(&pd)

	return response.OK(c, fiber.Map{
		"balance_in_paise":        user.WalletBalanceInPaise,
		"has_upi":                 pd.UpiID != "",
		"has_bank":                pd.BankAccountNumber != "",
		"min_withdrawal_in_paise": MinWithdrawal(),
	})
}

// HandleTransactions godoc
// @Summary     Paginated wallet ledger for the current user
// @Tags        User Wallet
// @Produce     json
// @Security    UserAuth
// @Param       page   query  int  false  "Page"
// @Param       limit  query  int  false  "Limit"
// @Success     200  {object}  []models.WalletTransaction
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/transactions [get]
func HandleTransactions(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	return listUserTransactions(c, userID)
}

// listUserTransactions returns a paginated wallet ledger for the given user.
func listUserTransactions(c *fiber.Ctx, userID string) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.WalletTransaction{}).Where("user_id = ?", userID)

	var total int64
	q.Count(&total)

	var txs []models.WalletTransaction
	if err := q.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&txs).Error; err != nil {
		return response.InternalError(c, "failed to fetch transactions")
	}
	if txs == nil {
		txs = []models.WalletTransaction{}
	}

	return response.Paginated(c, txs, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleTopupOrder godoc
// @Summary     Create a Razorpay order to add money to the wallet
// @Tags        User Wallet
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]int  true  "amount_in_paise"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/topup/order [post]
func HandleTopupOrder(c *fiber.Ctx) error {
	var body struct {
		AmountInPaise int64 `json:"amount_in_paise"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.AmountInPaise < minTopupInPaise || body.AmountInPaise > maxTopupInPaise {
		return response.BadRequest(c, "amount must be between ₹10 and ₹1,00,000")
	}

	userID, _ := c.Locals("userID").(string)

	if config.App.RazorpayKeyID == "" || config.App.RazorpayKeySecret == "" ||
		config.App.RazorpayKeyID == "rzp_test_xxxx" || config.App.RazorpayKeySecret == "xxxx" {
		return response.InternalError(c, "razorpay is not configured on the server")
	}

	rzpOrderID, err := createRazorpayOrder(body.AmountInPaise, userID)
	if err != nil {
		slog.Error("wallet: razorpay topup order creation failed", "error", err, "user_id", userID)
		return response.InternalError(c, "failed to create payment order")
	}

	topup := models.WalletTopup{
		UserID:          userID,
		AmountInPaise:   body.AmountInPaise,
		Status:          models.PaymentPending,
		RazorpayOrderID: &rzpOrderID,
	}
	database.DB.Create(&topup)

	// Same response shape as the market/plans order endpoints so the app
	// checkout code is shared.
	return response.OK(c, fiber.Map{
		"order_id": rzpOrderID,
		"amount":   body.AmountInPaise,
		"currency": "INR",
		"key_id":   config.App.RazorpayKeyID,
	})
}

// HandleTopupVerify godoc
// @Summary     Verify a Razorpay wallet topup signature and credit the wallet
// @Tags        User Wallet
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]string  true  "razorpay_order_id, razorpay_payment_id, razorpay_signature"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/wallet/topup/verify [post]
func HandleTopupVerify(c *fiber.Ctx) error {
	var body struct {
		RazorpayOrderID   string `json:"razorpay_order_id"`
		RazorpayPaymentID string `json:"razorpay_payment_id"`
		RazorpaySignature string `json:"razorpay_signature"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.RazorpayOrderID == "" || body.RazorpayPaymentID == "" || body.RazorpaySignature == "" {
		return response.BadRequest(c, "razorpay_order_id, razorpay_payment_id, and razorpay_signature are required")
	}

	userID, _ := c.Locals("userID").(string)

	// Verify signature: HMAC_SHA256(order_id|payment_id, key_secret)
	msg := body.RazorpayOrderID + "|" + body.RazorpayPaymentID
	mac := hmac.New(sha256.New, []byte(config.App.RazorpayKeySecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(body.RazorpaySignature), []byte(expected)) {
		return response.BadRequest(c, "invalid razorpay signature")
	}

	var t models.WalletTopup
	if err := database.DB.
		Where("user_id = ? AND razorpay_order_id = ?", userID, body.RazorpayOrderID).
		First(&t).Error; err != nil {
		return response.NotFound(c, "topup not found")
	}
	if t.Status == models.PaymentSuccess {
		return response.OK(c, fiber.Map{"message": "topup already verified"})
	}

	captureTopup(t.ID, body.RazorpayPaymentID, nil)

	return response.OK(c, fiber.Map{"message": "topup verified"})
}

// captureTopup atomically flips a topup to SUCCESS and credits the wallet in
// the same transaction. The status != SUCCESS guard makes the credit
// exactly-once across the verify endpoint and the webhook racing each other.
func captureTopup(topupID, razorpayPaymentID string, gatewayResponse models.JSONMap) bool {
	captured := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":              models.PaymentSuccess,
			"razorpay_payment_id": razorpayPaymentID,
			"paid_at":             time.Now(),
		}
		if gatewayResponse != nil {
			updates["gateway_response"] = gatewayResponse
		}
		result := tx.Model(&models.WalletTopup{}).
			Where("id = ? AND status != ?", topupID, models.PaymentSuccess).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		var t models.WalletTopup
		if err := tx.First(&t, "id = ?", topupID).Error; err != nil {
			return err
		}
		if _, err := Apply(tx, t.UserID, models.WalletTxTopup, t.AmountInPaise, &t.ID, nil); err != nil {
			return err
		}
		captured = true
		return nil
	})
	if err != nil {
		slog.Error("wallet: failed to capture topup", "topup_id", topupID, "error", err)
		return false
	}
	return captured
}

// CaptureRazorpayOrder resolves a webhook payment.captured event against the
// wallet_topups table. Called by the payments webhook handler when neither a
// subscription payment nor a market purchase matches the order ID. Returns
// true if the order belongs to a wallet topup.
func CaptureRazorpayOrder(orderID, razorpayPaymentID string, entity models.JSONMap) bool {
	var t models.WalletTopup
	if err := database.DB.Where("razorpay_order_id = ?", orderID).First(&t).Error; err != nil {
		return false
	}
	if t.Status == models.PaymentSuccess {
		return true
	}
	captureTopup(t.ID, razorpayPaymentID, entity)
	return true
}

// HandleGetPayoutDetails godoc
// @Summary     Get saved payout details (UPI / bank)
// @Tags        User Wallet
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  models.PayoutDetail
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/payout-details [get]
func HandleGetPayoutDetails(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var pd models.PayoutDetail
	database.DB.Where("user_id = ?", userID).First(&pd)
	return response.OK(c, pd)
}

// HandleUpsertPayoutDetails godoc
// @Summary     Save payout details (UPI ID and/or bank account)
// @Tags        User Wallet
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  models.PayoutDetail  true  "payout details"
// @Success     200  {object}  models.PayoutDetail
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/payout-details [put]
func HandleUpsertPayoutDetails(c *fiber.Ctx) error {
	var body struct {
		UpiID             string `json:"upi_id"`
		BankAccountNumber string `json:"bank_account_number"`
		BankIfsc          string `json:"bank_ifsc"`
		BankHolderName    string `json:"bank_holder_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	body.UpiID = strings.TrimSpace(body.UpiID)
	body.BankAccountNumber = strings.TrimSpace(body.BankAccountNumber)
	body.BankIfsc = strings.ToUpper(strings.TrimSpace(body.BankIfsc))
	body.BankHolderName = strings.TrimSpace(body.BankHolderName)

	if body.UpiID == "" && body.BankAccountNumber == "" {
		return response.BadRequest(c, "provide a UPI ID or bank account details")
	}
	if body.UpiID != "" && !upiRegex.MatchString(body.UpiID) {
		return response.BadRequest(c, "invalid UPI ID")
	}
	// Bank fields are all-or-nothing.
	if body.BankAccountNumber != "" || body.BankIfsc != "" || body.BankHolderName != "" {
		if !accountRegex.MatchString(body.BankAccountNumber) {
			return response.BadRequest(c, "invalid bank account number")
		}
		if !ifscRegex.MatchString(body.BankIfsc) {
			return response.BadRequest(c, "invalid IFSC code")
		}
		if body.BankHolderName == "" {
			return response.BadRequest(c, "account holder name is required")
		}
	}

	userID, _ := c.Locals("userID").(string)

	var pd models.PayoutDetail
	database.DB.Where("user_id = ?", userID).First(&pd)
	pd.UserID = userID
	pd.UpiID = body.UpiID
	pd.BankAccountNumber = body.BankAccountNumber
	pd.BankIfsc = body.BankIfsc
	pd.BankHolderName = body.BankHolderName
	if err := database.DB.Save(&pd).Error; err != nil {
		return response.InternalErrorWithLog(c, "wallet: save payout details", err)
	}
	return response.OK(c, pd)
}

// HandleCreateWithdrawal godoc
// @Summary     Request a withdrawal (auto-approved; balance deducted immediately)
// @Tags        User Wallet
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       body  body  map[string]interface{}  true  "amount_in_paise, method (UPI|BANK)"
// @Success     201  {object}  models.Withdrawal
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/withdrawals [post]
func HandleCreateWithdrawal(c *fiber.Ctx) error {
	var body struct {
		AmountInPaise int64  `json:"amount_in_paise"`
		Method        string `json:"method"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	body.Method = strings.ToUpper(strings.TrimSpace(body.Method))
	if body.Method != models.WithdrawalMethodUPI && body.Method != models.WithdrawalMethodBank {
		return response.BadRequest(c, "method must be UPI or BANK")
	}
	if min := MinWithdrawal(); body.AmountInPaise < min {
		return response.BadRequest(c, fmt.Sprintf("minimum withdrawal is ₹%d", min/100))
	}

	userID, _ := c.Locals("userID").(string)

	var pd models.PayoutDetail
	if err := database.DB.Where("user_id = ?", userID).First(&pd).Error; err != nil {
		return response.BadRequest(c, "save your payout details first")
	}
	if body.Method == models.WithdrawalMethodUPI && pd.UpiID == "" {
		return response.BadRequest(c, "add your UPI ID first")
	}
	if body.Method == models.WithdrawalMethodBank && pd.BankAccountNumber == "" {
		return response.BadRequest(c, "add your bank account details first")
	}

	w := models.Withdrawal{
		UserID:        userID,
		AmountInPaise: body.AmountInPaise,
		Method:        body.Method,
		Status:        models.WithdrawalStatusApproved,
	}
	// Snapshot the destination as it is right now.
	if body.Method == models.WithdrawalMethodUPI {
		w.UpiID = pd.UpiID
	} else {
		w.BankAccountNumber = pd.BankAccountNumber
		w.BankIfsc = pd.BankIfsc
		w.BankHolderName = pd.BankHolderName
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&w).Error; err != nil {
			return err
		}
		_, err := Apply(tx, userID, models.WalletTxWithdrawal, -body.AmountInPaise, &w.ID,
			models.JSONMap{"method": body.Method})
		return err
	})
	if err == ErrInsufficientBalance {
		return response.BadRequest(c, "insufficient wallet balance")
	}
	if err != nil {
		return response.InternalErrorWithLog(c, "wallet: create withdrawal", err)
	}

	return response.Created(c, w)
}

// HandleMyWithdrawals godoc
// @Summary     List own withdrawal requests
// @Tags        User Wallet
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.Withdrawal
// @Failure     401  {object}  map[string]string
// @Router      /user/wallet/withdrawals [get]
func HandleMyWithdrawals(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var ws []models.Withdrawal
	if err := database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(100).
		Find(&ws).Error; err != nil {
		return response.InternalError(c, "failed to fetch withdrawals")
	}
	if ws == nil {
		ws = []models.Withdrawal{}
	}
	return response.OK(c, ws)
}

// HandleGetFee godoc
// @Summary     Current platform fee percent (for the upload form breakdown)
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]int64
// @Router      /user/market/fee [get]
func HandleGetFee(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{"fee_percent": FeePercent()})
}

// createRazorpayOrder creates a topup order via the Razorpay REST API (no
// SDK), mirroring the market module's helper.
func createRazorpayOrder(amountInPaise int64, userID string) (string, error) {
	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("wal_%s_%d", userID[:8], time.Now().Unix()),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(config.App.RazorpayKeyID, config.App.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("razorpay error: %s", string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	id, _ := result["id"].(string)
	if id == "" {
		return "", fmt.Errorf("razorpay returned empty order id")
	}
	return id, nil
}
