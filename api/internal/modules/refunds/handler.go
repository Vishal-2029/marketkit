package refunds

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/payments"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandleRequest — any admin submits a refund request for a qualifying payment.
//
// Eligible ONLY when:
//   - payment.status = SUCCESS
//   - payment.gateway = RAZORPAY
//   - subscription is still ACTIVE (not expired/cancelled)
//   - user has NOT watched any video during subscription window
//   - no existing PENDING request for this payment
func HandleRequest(c *fiber.Ctx) error {
	adminID, _ := c.Locals("adminID").(string)

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil || body.Reason == "" {
		return response.BadRequest(c, "reason is required")
	}

	allowedReasons := map[string]bool{
		"Payment Successful But Access Not Given": true,
		"Technical Issue on Admin Side":           true,
	}
	if !allowedReasons[body.Reason] {
		return response.BadRequest(c, "Invalid refund reason. Allowed reasons: 'Payment Successful But Access Not Given', 'Technical Issue on Admin Side'. Refunds are not allowed if User Changed Their Mind.")
	}

	paymentID := c.Params("id")
	var p models.Payment
	if err := database.DB.Preload("User").Preload("Plan").
		First(&p, "id = ?", paymentID).Error; err != nil {
		return response.NotFound(c, "payment not found")
	}

	if p.Status != models.PaymentSuccess {
		return response.BadRequest(c, "only successful payments are eligible for refund")
	}
	// Manual (offline) payments have no gateway to refund through; anything
	// taken by a gateway can be refunded by that gateway.
	if p.Provider == models.ProviderManual {
		return response.BadRequest(c, "manual payments cannot be refunded through the gateway")
	}
	if p.ProviderPaymentID == nil || *p.ProviderPaymentID == "" {
		return response.BadRequest(c, "payment has no gateway payment ID on record")
	}

	// Subscription must still be ACTIVE
	var sub models.Subscription
	if err := database.DB.
		Where("user_id = ? AND plan_id = ? AND status = ?",
			p.UserID, p.PlanID, models.SubscriptionActive).
		Order("created_at DESC").First(&sub).Error; err != nil {
		return response.BadRequest(c, "subscription is no longer active — refund not applicable")
	}

	// Refunds apply only to paid subscriptions, never to admin-granted free plans.
	if sub.ActivatedBy == "free" {
		return response.BadRequest(c, "user is on a free (admin-granted) plan — refunds apply only to paid subscriptions")
	}

	// User must NOT have watched any video
	startAt := p.CreatedAt
	if p.PaidAt != nil {
		startAt = *p.PaidAt
	}
	var watched int64
	if err := database.DB.Model(&models.PlaybackLog{}).
		Where("user_id = ? AND played_at >= ? AND played_at <= ?",
			p.UserID, startAt, sub.ExpiryDate).
		Count(&watched).Error; err != nil {
		return response.InternalError(c, "could not verify watch history")
	}
	if watched > 0 {
		return response.BadRequest(c, fmt.Sprintf(
			"user has watched %d video(s) — subscription was used, refund not applicable", watched))
	}

	// No duplicate pending request
	var existing int64
	database.DB.Model(&models.RefundRequest{}).
		Where("payment_id = ? AND status = ?", paymentID, models.RefundPending).
		Count(&existing)
	if existing > 0 {
		return response.BadRequest(c, "a refund request is already pending for this payment")
	}

	req := models.RefundRequest{
		PaymentID:   paymentID,
		RequestedBy: adminID,
		Reason:      body.Reason,
		Status:      models.RefundPending,
	}
	if err := database.DB.Create(&req).Error; err != nil {
		return response.InternalError(c, "failed to create refund request")
	}

	database.DB.Preload("Payment.User").Preload("Payment.Plan").
		Preload("RequestedByAdmin").First(&req, "id = ?", req.ID)
	// Trim the preloaded end-user down to name/email (what the admin UI
	// shows) — phone/wallet balance/avatar/status don't belong in a refund
	// request response.
	req.Payment.User = models.User{ID: req.Payment.User.ID, Name: req.Payment.User.Name, Email: req.Payment.User.Email}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &adminID,
		TargetID:     &p.UserID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":     "refund_requested",
			"payment_id": paymentID,
			"reason":     body.Reason,
		},
	})

	return response.Created(c, req)
}

// HandleList — list refund requests. Supports ?status=PENDING|APPROVED|REJECTED
func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	q := database.DB.Model(&models.RefundRequest{}).
		Preload("Payment.User").Preload("Payment.Plan").
		Preload("RequestedByAdmin").Preload("ReviewedByAdmin")

	if s := c.Query("status"); s != "" {
		q = q.Where("refund_requests.status = ?", s)
	}

	var total int64
	q.Count(&total)

	var reqs []models.RefundRequest
	if err := q.Offset((page - 1) * limit).Limit(limit).
		Order("refund_requests.created_at DESC").Find(&reqs).Error; err != nil {
		return response.InternalError(c, "failed to fetch refund requests")
	}
	for i := range reqs {
		reqs[i].Payment.User = models.User{ID: reqs[i].Payment.User.ID, Name: reqs[i].Payment.User.Name, Email: reqs[i].Payment.User.Email}
	}

	return response.Paginated(c, reqs, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleApprove — super admin only. Calls the gateway, marks APPROVED, cancels subscription.
func HandleApprove(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins can approve refunds")
	}

	var req models.RefundRequest
	if err := database.DB.Preload("Payment.User").Preload("Payment.Plan").
		First(&req, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "refund request not found")
	}
	if req.Status != models.RefundPending {
		return response.BadRequest(c, "only pending requests can be approved")
	}

	p := req.Payment
	if p.ProviderPaymentID == nil {
		return response.BadRequest(c, "no gateway payment ID on record")
	}

	var body struct {
		Note string `json:"note"`
	}
	_ = c.BodyParser(&body)

	refundID, err := callGatewayRefund(*p.ProviderPaymentID, p.AmountInPaise, req.Reason)
	if err != nil {
		return response.InternalErrorWithLog(c, "gateway refund failed", err)
	}

	now := time.Now()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&req).Updates(map[string]interface{}{
			"status":      models.RefundApproved,
			"reviewed_by": actorID,
			"review_note": body.Note,
			"reviewed_at": now,
			"refund_id":   refundID,
		}).Error; err != nil {
			return err
		}

		gr := p.GatewayResponse
		if gr == nil {
			gr = models.JSONMap{}
		}
		gr["refund_id"] = refundID
		gr["refunded_at"] = now.Format(time.RFC3339)
		gr["refund_reason"] = req.Reason

		if err := tx.Model(&p).Updates(map[string]interface{}{
			"status":           models.PaymentRefunded,
			"notes":            req.Reason,
			"gateway_response": gr,
		}).Error; err != nil {
			return err
		}

		return tx.Model(&models.Subscription{}).
			Where("user_id = ? AND plan_id = ? AND status = ?",
				p.UserID, p.PlanID, models.SubscriptionActive).
			Update("status", models.SubscriptionCancelled).Error
	}); err != nil {
		// The gateway already issued the refund — make sure the id is not lost so the
		// out-of-sync state can be reconciled manually.
		slog.Error("refund: gateway refund SUCCEEDED but DB update failed — manual reconciliation needed",
			"refund_id", refundID, "request_id", req.ID, "payment_id", p.ID,
			"user_id", p.UserID, "amount", p.AmountInPaise, "error", err)
		database.DB.Model(&req).Update("refund_id", refundID)
		return response.InternalError(c,
			"refund issued by the gateway (id: "+refundID+") but recording it failed — reconcile manually")
	}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorID,
		TargetID:     &p.UserID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":     "refund_approved",
			"request_id": req.ID,
			"payment_id": p.ID,
			"refund_id":  refundID,
			"amount":     p.AmountInPaise,
		},
	})

	go email.SendRefundEmail(p.User.Email, email.RefundEmailData{
		Name:          p.User.Name,
		PlanName:      p.Plan.Name,
		Amount:        email.FormatAmount(p.AmountInPaise),
		TransactionID: *p.ProviderPaymentID,
		RefundID:      refundID,
		Reason:        req.Reason,
		RefundedAt:    email.FormatDate(now),
	})

	return response.OK(c, fiber.Map{
		"message":   "refund approved and processed",
		"refund_id": refundID,
		"amount":    email.FormatAmount(p.AmountInPaise),
	})
}

// HandleReject — super admin only. Marks REJECTED with a note.
func HandleReject(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins can reject refunds")
	}

	var req models.RefundRequest
	if err := database.DB.Preload("Payment.User").Preload("Payment.Plan").
		First(&req, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "refund request not found")
	}
	if req.Status != models.RefundPending {
		return response.BadRequest(c, "only pending requests can be rejected")
	}

	var body struct {
		Note string `json:"note"`
	}
	_ = c.BodyParser(&body)
	if body.Note == "" {
		body.Note = "Rejected by super admin"
	}

	now := time.Now()
	if err := database.DB.Model(&req).Updates(map[string]interface{}{
		"status":      models.RefundRejected,
		"reviewed_by": actorID,
		"review_note": body.Note,
		"reviewed_at": now,
	}).Error; err != nil {
		return response.InternalError(c, "failed to reject refund request")
	}

	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorID,
		TargetID:     &req.PaymentID,
		IPAddress:    c.IP(),
		Details: models.JSONMap{
			"action":     "refund_rejected",
			"request_id": req.ID,
			"note":       body.Note,
		},
	})

	if req.Payment.User.Email != "" {
		go email.SendRefundRejectionEmail(req.Payment.User.Email, email.RefundRejectionData{
			Name:     req.Payment.User.Name,
			PlanName: req.Payment.Plan.Name,
			Reason:   req.Reason,
			Note:     body.Note,
		})
	}

	return response.OK(c, fiber.Map{"message": "refund request rejected"})
}

// callGatewayRefund refunds a captured payment through the active gateway.
func callGatewayRefund(providerPaymentID string, amountPaise int64, reason string) (string, error) {
	refund, err := payments.Refund(context.Background(), providerPaymentID, amountPaise, reason)
	if err != nil {
		return "", err
	}
	return refund.ID, nil
}
