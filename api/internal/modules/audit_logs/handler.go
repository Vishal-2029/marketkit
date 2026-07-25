package audit_logs

import (
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleList godoc
// @Summary     List audit logs (admin)
// @Tags        Admin Audit Logs
// @Produce     json
// @Security    AdminAuth
// @Param       page        query  int  false  "Page number (default: 1)"
// @Param       limit       query  int  false  "Items per page (default: 50)"
// @Param       event_type  query  string  false  "Filter by event type"
// @Param       start_date  query  string  false  "Filter by start date (YYYY-MM-DD)"
// @Param       end_date    query  string  false  "Filter by end date (YYYY-MM-DD)"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /audit-logs [get]
func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}

	q := database.DB.Model(&models.AuditLog{}).Where("event_type IN ?", []models.AuditEventType{
		models.EventLogin, models.EventLogout, models.EventForceLogout,
		models.EventPlanPurchase, models.EventAdminAction, models.EventPaymentFailed,
		models.EventUserSuspended, models.EventUserReactivated, models.EventPlanChanged,
		models.EventManualActivation, models.EventVideoDeleted, models.EventUserDeleted,
	})
	if et := c.Query("event_type"); et != "" {
		q = q.Where("event_type = ?", et)
	}
	if sd := c.Query("start_date"); sd != "" {
		q = q.Where("created_at >= ?", sd)
	}
	if ed := c.Query("end_date"); ed != "" {
		q = q.Where("created_at <= ?", ed)
	}

	var total int64
	q.Count(&total)

	var logs []models.AuditLog
	q.Offset((page - 1) * limit).Limit(limit).Order("created_at DESC").Find(&logs)

	return response.Paginated(c, logs, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}
