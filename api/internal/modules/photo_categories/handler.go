package photo_categories

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleList godoc
// @Summary     List photo categories (admin)
// @Tags        Admin Photo Categories
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /photo-categories [get]
func HandleList(c *fiber.Ctx) error {
	var categories []models.PhotoCategory
	database.DB.Order("name ASC").Find(&categories)
	return response.OK(c, categories)
}

// HandleCreate godoc
// @Summary     Create a photo category (admin)
// @Tags        Admin Photo Categories
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       body  body  map[string]string  true  "Category name"
// @Success     201  {object}  models.PhotoCategory
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /photo-categories [post]
func HandleCreate(c *fiber.Ctx) error {
	var body struct {
		Name string `json:"name"`
	}
	c.BodyParser(&body)

	name := strings.TrimSpace(body.Name)
	if name == "" {
		return response.BadRequest(c, "name is required")
	}

	// FirstOrCreate keeps create idempotent — re-adding an existing name returns the
	// existing row instead of erroring on the unique index.
	var category models.PhotoCategory
	if err := database.DB.Where("name = ?", name).FirstOrCreate(&category, models.PhotoCategory{Name: name}).Error; err != nil {
		return response.InternalError(c, "failed to create category")
	}
	return response.Created(c, category)
}

// HandleDelete godoc
// @Summary     Delete a photo category (admin)
// @Tags        Admin Photo Categories
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Category ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /photo-categories/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	// Non-destructive to photos: only the selectable list entry is removed.
	// Photos that already stored this category text keep their value.
	res := database.DB.Delete(&models.PhotoCategory{}, "id = ?", c.Params("id"))
	if res.Error != nil {
		return response.InternalErrorWithLog(c, "photo_categories: delete", res.Error)
	}
	if res.RowsAffected == 0 {
		return response.NotFound(c, "category not found")
	}
	return response.OK(c, fiber.Map{"message": "category deleted"})
}
