package market

import (
	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// buyersPurchase looks up the requesting user's successful purchase of a
// design — the thread is keyed by purchase internally, but the app only
// ever has the design ID handy (a design can only be bought once per buyer).
func buyersPurchase(userID, designID string) (models.DesignPurchase, error) {
	var purchase models.DesignPurchase
	err := database.DB.
		Where("design_id = ? AND buyer_id = ? AND status = ?", designID, userID, models.PaymentSuccess).
		Order("created_at DESC").
		First(&purchase).Error
	return purchase, err
}

// HandleListDesignMessages godoc
// @Summary     List the buyer's private support thread for a purchased design
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  []models.DesignPurchaseMessage
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/designs/{id}/messages [get]
func HandleListDesignMessages(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	purchase, err := buyersPurchase(userID, c.Params("id"))
	if err != nil {
		return response.NotFound(c, "you haven't purchased this design")
	}

	var messages []models.DesignPurchaseMessage
	database.DB.
		Where("purchase_id = ? AND thread_user_id = ?", purchase.ID, userID).
		Order("created_at ASC").
		Find(&messages)
	if messages == nil {
		messages = []models.DesignPurchaseMessage{}
	}
	return response.OK(c, messages)
}

// HandlePostDesignMessage godoc
// @Summary     Send a message on the buyer's private support thread for a purchased design
// @Tags        User Market
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       id    path  string             true  "Design ID"
// @Param       body  body  map[string]string  true  "content"
// @Success     201  {object}  models.DesignPurchaseMessage
// @Failure     400  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/designs/{id}/messages [post]
func HandlePostDesignMessage(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	purchase, err := buyersPurchase(userID, c.Params("id"))
	if err != nil {
		return response.NotFound(c, "you haven't purchased this design")
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return response.BadRequest(c, "content is required")
	}

	var user models.User
	userName := "User"
	if err := database.DB.Select("name").First(&user, "id = ?", userID).Error; err == nil && user.Name != "" {
		userName = user.Name
	}

	message := models.DesignPurchaseMessage{
		PurchaseID:   purchase.ID,
		UserID:       userID,
		ThreadUserID: userID,
		IsAdmin:      false,
		UserName:     userName,
		Content:      body.Content,
	}
	if err := database.DB.Create(&message).Error; err != nil {
		return response.InternalError(c, "failed to send message")
	}
	return response.Created(c, message)
}

// ---------------------------------------------------------------------------
// Admin handlers
// ---------------------------------------------------------------------------

// HandleAdminListPurchaseMessages godoc
// @Summary     List a purchase's private support thread (admin)
// @Tags        Admin Market
// @Produce     json
// @Security    BearerAuth
// @Param       id  path  string  true  "Purchase ID"
// @Success     200  {object}  []models.DesignPurchaseMessage
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/purchases/{id}/messages [get]
func HandleAdminListPurchaseMessages(c *fiber.Ctx) error {
	purchaseID := c.Params("id")

	var purchase models.DesignPurchase
	if err := database.DB.First(&purchase, "id = ?", purchaseID).Error; err != nil {
		return response.NotFound(c, "purchase not found")
	}

	var messages []models.DesignPurchaseMessage
	database.DB.
		Where("purchase_id = ?", purchaseID).
		Order("created_at ASC").
		Find(&messages)
	if messages == nil {
		messages = []models.DesignPurchaseMessage{}
	}
	return response.OK(c, messages)
}

// HandleAdminReplyPurchaseMessage godoc
// @Summary     Reply on a purchase's private support thread (admin)
// @Tags        Admin Market
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id    path  string             true  "Purchase ID"
// @Param       body  body  map[string]string  true  "content"
// @Success     201  {object}  models.DesignPurchaseMessage
// @Failure     400  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /market/purchases/{id}/messages [post]
func HandleAdminReplyPurchaseMessage(c *fiber.Ctx) error {
	purchaseID := c.Params("id")

	var purchase models.DesignPurchase
	if err := database.DB.First(&purchase, "id = ?", purchaseID).Error; err != nil {
		return response.NotFound(c, "purchase not found")
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return response.BadRequest(c, "content is required")
	}

	adminID, _ := c.Locals("adminID").(string)
	adminName := "Admin"
	var admin models.Admin
	if adminID != "" {
		if err := database.DB.Select("first_name, last_name").First(&admin, "id = ?", adminID).Error; err == nil {
			if full := admin.FullName(); full != "" {
				adminName = full
			}
		}
	}

	message := models.DesignPurchaseMessage{
		PurchaseID:   purchaseID,
		UserID:       "admin",
		ThreadUserID: purchase.BuyerID,
		IsAdmin:      true,
		UserName:     adminName,
		Content:      body.Content,
	}
	if err := database.DB.Create(&message).Error; err != nil {
		return response.InternalError(c, "failed to post reply")
	}

	preview := body.Content
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	go fcm.SendToUser(purchase.BuyerID, "Admin replied", preview)

	return response.Created(c, message)
}
