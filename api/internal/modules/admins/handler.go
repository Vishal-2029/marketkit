package admins

import (
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/crypto/bcrypt"
)

// HandleList godoc
// @Summary     List all admin accounts
// @Tags        Admins
// @Produce     json
// @Security    AdminAuth
// @Param       page  query  int  false  "Page number"
// @Param       limit  query  int  false  "Items per page"
// @Param       search  query  string  false  "Search term"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /admins [get]
func HandleList(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins may list admin accounts")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Admin{})
	if search := c.Query("search"); search != "" {
		query = query.Where(
			"first_name ILIKE ? OR last_name ILIKE ? OR email ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%",
		)
	}

	var total int64
	query.Count(&total)

	var admins []models.Admin
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&admins).Error; err != nil {
		return response.InternalError(c, "failed to fetch admins")
	}

	return response.Paginated(c, admins, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleDelete godoc
// @Summary     Permanently delete an admin (super-admin only)
// @Tags        Admins
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Admin ID"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Router      /admins/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins may delete admin accounts")
	}

	id := c.Params("id")
	var target models.Admin
	if err := database.DB.First(&target, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "admin not found")
	}

	// Prevent removing the last super-admin
	if target.IsSuper {
		var cnt int64
		database.DB.Model(&models.Admin{}).Where("is_super = true").Count(&cnt)
		if cnt <= 1 {
			return response.BadRequest(c, "cannot delete the last super-admin")
		}
	}

	if err := database.DB.Delete(&models.Admin{}, "id = ?", id).Error; err != nil {
		return response.InternalError(c, "failed to delete admin")
	}

	// Audit
	actorRef := actorID
	targetRef := id
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorRef,
		TargetID:     &targetRef,
		IPAddress:    c.IP(),
		Device:       c.Get("User-Agent"),
		Details:      models.JSONMap{"action": "delete_admin", "email": target.Email},
	})

	return response.OK(c, fiber.Map{"message": "admin deleted"})
}

// HandleSetSuper godoc
// @Summary     Grant or revoke super-admin status (super-admin only)
// @Tags        Admins
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Admin ID"
// @Param       body  body  map[string]bool  true  "{\"is_super\": true|false}"
// @Success     200  {object}  models.Admin
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Router      /admins/{id}/super [patch]
func HandleCreate(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins may create admin accounts")
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		IsSuper   bool   `json:"is_super"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if req.FirstName == "" || req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "first name, email and password are required")
	}
	if len(req.Password) < 6 {
		return response.BadRequest(c, "password must be at least 6 characters")
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	var existingCount int64
	database.DB.Model(&models.Admin{}).Where("LOWER(email) = ?", req.Email).Count(&existingCount)
	if existingCount > 0 {
		return response.BadRequest(c, "an account with this email already exists")
	}
	var userCount int64
	database.DB.Model(&models.User{}).Where("LOWER(email) = ?", req.Email).Count(&userCount)
	if userCount > 0 {
		return response.BadRequest(c, "an account with this email already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return response.InternalError(c, "failed to create admin")
	}

	admin := models.Admin{
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Phone:        req.Phone,
		Email:        req.Email,
		PasswordHash: string(hash),
		IsActive:     true,
		IsSuper:      req.IsSuper,
	}
	if err := database.DB.Create(&admin).Error; err != nil {
		return response.InternalError(c, "failed to create admin")
	}

	actorRef := actorID
	targetRef := admin.ID
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorRef,
		TargetID:     &targetRef,
		IPAddress:    c.IP(),
		Device:       c.Get("User-Agent"),
		Details:      models.JSONMap{"action": "create_admin", "email": admin.Email, "is_super": admin.IsSuper},
	})

	return response.Created(c, admin)
}

func HandleSetSuper(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins may change super status")
	}

	id := c.Params("id")
	var body struct {
		IsSuper *bool `json:"is_super"`
	}
	if err := c.BodyParser(&body); err != nil || body.IsSuper == nil {
		return response.BadRequest(c, "invalid body")
	}

	var target models.Admin
	if err := database.DB.First(&target, "id = ?", id).Error; err != nil {
		return response.NotFound(c, "admin not found")
	}

	// If demoting a super, ensure at least one super remains
	if target.IsSuper && !*body.IsSuper {
		var cnt int64
		database.DB.Model(&models.Admin{}).Where("is_super = true").Count(&cnt)
		if cnt <= 1 {
			return response.BadRequest(c, "cannot remove super status from the last super-admin")
		}
	}

	if err := database.DB.Model(&models.Admin{}).Where("id = ?", id).Update("is_super", *body.IsSuper).Error; err != nil {
		return response.InternalError(c, "failed to update super status")
	}

	// Return updated admin
	if err := database.DB.First(&target, "id = ?", id).Error; err != nil {
		return response.InternalError(c, "failed to fetch admin")
	}

	// Audit
	actorRef := actorID
	targetRef := id
	action := "grant_super"
	if !*body.IsSuper {
		action = "revoke_super"
	}
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventAdminAction,
		ActorAdminID: &actorRef,
		TargetID:     &targetRef,
		IPAddress:    c.IP(),
		Device:       c.Get("User-Agent"),
		Details:      models.JSONMap{"action": action, "email": target.Email},
	})

	return response.OK(c, target)
}
