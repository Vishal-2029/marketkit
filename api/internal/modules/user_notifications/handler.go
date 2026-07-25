package user_notifications

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

type notifItem struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// HandleList godoc
// @Summary     List user notifications
// @Tags        User Notifications
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /user/notifications [get]
func HandleList(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var rows []models.UserNotification
	database.DB.Preload("NotificationLog").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(30).
		Find(&rows)

	var unreadCount int64
	database.DB.Model(&models.UserNotification{}).
		Where("user_id = ? AND read = false", userID).
		Count(&unreadCount)

	items := make([]notifItem, len(rows))
	for i, n := range rows {
		items[i] = notifItem{
			ID:        n.ID,
			Title:     n.NotificationLog.Title,
			Body:      n.NotificationLog.Body,
			Read:      n.Read,
			CreatedAt: n.CreatedAt,
		}
	}

	return response.OK(c, fiber.Map{
		"notifications": items,
		"unread_count":  unreadCount,
	})
}

// HandleMarkAllRead godoc
// @Summary     Mark all notifications as read
// @Tags        User Notifications
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/notifications/read [post]
func HandleMarkAllRead(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	database.DB.Model(&models.UserNotification{}).
		Where("user_id = ? AND read = false", userID).
		Update("read", true)
	return response.OK(c, fiber.Map{"message": "notifications marked as read"})
}

// HandleClearAll godoc
// @Summary     Clear all notifications for the current user
// @Tags        User Notifications
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/notifications [delete]
func HandleClearAll(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	database.DB.Where("user_id = ?", userID).Delete(&models.UserNotification{})
	return response.OK(c, fiber.Map{"message": "notifications cleared"})
}

// HandleDeleteOne godoc
// @Summary     Delete a single notification for the current user
// @Tags        User Notifications
// @Produce     json
// @Security    UserAuth
// @Param       id   path  int  true  "Notification ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/notifications/{id} [delete]
func HandleDeleteOne(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	id, err := c.ParamsInt("id")
	if err != nil {
		return response.BadRequest(c, "invalid notification id")
	}
	result := database.DB.Where("user_id = ? AND id = ?", userID, id).
		Delete(&models.UserNotification{})
	if result.RowsAffected == 0 {
		return response.NotFound(c, "notification not found")
	}
	return response.OK(c, fiber.Map{"message": "notification deleted"})
}
