package notifications

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleBroadcast godoc
// @Summary     Broadcast push notification to users (admin)
// @Tags        Admin Notifications
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]string  true  "title, message, optional plan_id and sub_status"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/notifications/broadcast [post]
func HandleBroadcast(c *fiber.Ctx) error {
	var body struct {
		Title     string `json:"title"`
		Message   string `json:"message"`
		PlanID    string `json:"plan_id"`
		SubStatus string `json:"sub_status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.Title == "" || body.Message == "" {
		return response.BadRequest(c, "title and message are required")
	}

	// Records history synchronously; the actual push delivery is dispatched
	// asynchronously inside the fcm package, so this returns promptly.
	if body.PlanID != "" || body.SubStatus != "" {
		fcm.SendToSegment(body.Title, body.Message, body.PlanID, body.SubStatus)
	} else {
		fcm.SendToAll(body.Title, body.Message)
	}

	return response.OK(c, fiber.Map{"message": "notification sent"})
}

// HandleListNotifications godoc
// @Summary     List sent notification logs (admin)
// @Tags        Admin Notifications
// @Produce     json
// @Security    AdminAuth
// @Param       page  query  int  false  "Page number (default: 1)"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /admin/notifications [get]
func HandleListNotifications(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.NotificationLog{}).Count(&total)

	var logs []models.NotificationLog
	database.DB.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs)
	if logs == nil {
		logs = []models.NotificationLog{}
	}

	return response.OK(c, fiber.Map{"logs": logs, "total": total})
}

// HandleDeleteNotification godoc
// @Summary     Delete a single notification from history (admin)
// @Tags        Admin Notifications
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  int  true  "Notification log ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/notifications/{id} [delete]
func HandleDeleteNotification(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid notification id")
	}

	// Remove the per-user inbox rows first, then the history row.
	database.DB.Where("notification_log_id = ?", id).Delete(&models.UserNotification{})
	result := database.DB.Delete(&models.NotificationLog{}, id)
	if result.RowsAffected == 0 {
		return response.NotFound(c, "notification not found")
	}

	return response.OK(c, fiber.Map{"message": "notification deleted"})
}

// HandleClearNotifications godoc
// @Summary     Delete all notifications from history (admin)
// @Tags        Admin Notifications
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/notifications [delete]
func HandleClearNotifications(c *fiber.Ctx) error {
	// Wipe per-user inbox rows, then all history rows.
	database.DB.Where("1 = 1").Delete(&models.UserNotification{})
	database.DB.Where("1 = 1").Delete(&models.NotificationLog{})

	return response.OK(c, fiber.Map{"message": "all notifications cleared"})
}
