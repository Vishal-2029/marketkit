package market

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/imageutil"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
)

// populateCategoryPhotoURLs resolves each section's PhotoKey to a public URL
// in place. Sections with no admin-set photo are left with an empty URL —
// the app falls back to the first design's own preview in that case.
func populateCategoryPhotoURLs(cats []models.DesignCategory) {
	for i := range cats {
		if cats[i].PhotoKey != nil && *cats[i].PhotoKey != "" {
			cats[i].PhotoURL = storage.Store.PublicURL(*cats[i].PhotoKey)
		}
	}
}

// categoryWithCount is a DesignCategory row annotated with how many designs
// currently point at it — lets the admin UI disable/explain a blocked delete
// without a second round trip per row.
type categoryWithCount struct {
	models.DesignCategory
	DesignCount int64 `json:"design_count"`
}

// HandleAdminListCategories godoc
// @Summary     List all Design Market sections, flat, with design counts (admin)
// @Tags        Admin Market Categories
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []categoryWithCount
// @Router      /market/categories [get]
func HandleAdminListCategories(c *fiber.Ctx) error {
	var categories []models.DesignCategory
	if err := database.DB.
		Order("display_order ASC, created_at ASC").
		Find(&categories).Error; err != nil {
		return response.InternalError(c, "failed to fetch sections")
	}

	var counts []struct {
		CategoryID string `json:"category_id"`
		Count      int64  `json:"count"`
	}
	if err := database.DB.Model(&models.Design{}).
		Select("category_id, COUNT(*) as count").
		Where("category_id IS NOT NULL").
		Group("category_id").
		Scan(&counts).Error; err != nil {
		return response.InternalError(c, "failed to fetch section design counts")
	}
	countByID := make(map[string]int64, len(counts))
	for _, row := range counts {
		countByID[row.CategoryID] = row.Count
	}

	populateCategoryPhotoURLs(categories)
	result := make([]categoryWithCount, 0, len(categories))
	for _, cat := range categories {
		result = append(result, categoryWithCount{
			DesignCategory: cat,
			DesignCount:    countByID[cat.ID],
		})
	}
	return response.OK(c, result)
}

// HandleAdminCreateCategory godoc
// @Summary     Create a Design Market section or sub-section (admin)
// @Tags        Admin Market Categories
// @Produce     json
// @Security    AdminAuth
// @Router      /market/categories [post]
func HandleAdminCreateCategory(c *fiber.Ctx) error {
	var body struct {
		Name         string  `json:"name"`
		ParentID     *string `json:"parent_id"`
		DisplayOrder int     `json:"display_order"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		return response.BadRequest(c, "name is required")
	}

	if body.ParentID != nil {
		var parent models.DesignCategory
		if err := database.DB.First(&parent, "id = ?", *body.ParentID).Error; err != nil {
			return response.BadRequest(c, "parent section not found")
		}
		if parent.IsOther {
			return response.BadRequest(c, "cannot create a section under Other")
		}
		if parent.ParentID != nil {
			return response.BadRequest(c, "sections can only be nested one level deep")
		}
	}

	category := models.DesignCategory{
		Name:         body.Name,
		ParentID:     body.ParentID,
		DisplayOrder: body.DisplayOrder,
	}
	if err := database.DB.Create(&category).Error; err != nil {
		return response.InternalError(c, "failed to create section")
	}
	return response.Created(c, categoryWithCount{DesignCategory: category, DesignCount: 0})
}

// HandleAdminUploadCategoryPhoto godoc
// @Summary     Set (or replace) a section's banner photo (admin)
// @Tags        Admin Market Categories
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       id     path  string  true  "Category ID"
// @Param       photo  formData  file  true  "Section banner photo"
// @Router      /market/categories/{id}/photo [post]
func HandleAdminUploadCategoryPhoto(c *fiber.Ctx) error {
	var cat models.DesignCategory
	if err := database.DB.First(&cat, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "section not found")
	}
	if cat.IsOther {
		return response.BadRequest(c, "the Other section cannot be edited")
	}

	fileHeader, err := c.FormFile("photo")
	if err != nil {
		return response.BadRequest(c, "photo file is required")
	}

	photoKey, err := uploadCategoryPhoto(fileHeader)
	if err != nil {
		return response.BadRequest(c, "invalid photo: "+err.Error())
	}

	oldKey := cat.PhotoKey
	if err := database.DB.Model(&cat).Update("photo_key", photoKey).Error; err != nil {
		cleanupKeys([]string{photoKey})
		return response.InternalError(c, "failed to save section photo")
	}
	if oldKey != nil && *oldKey != "" {
		cleanupKeys([]string{*oldKey})
	}

	cat.PhotoKey = &photoKey
	cat.PhotoURL = storage.Store.PublicURL(photoKey)
	var designCount int64
	database.DB.Model(&models.Design{}).Where("category_id = ?", cat.ID).Count(&designCount)
	return response.OK(c, categoryWithCount{DesignCategory: cat, DesignCount: designCount})
}

// uploadCategoryPhoto validates, compresses, and stores a section banner
// photo — mirrors uploadPreviewImage but under its own key prefix.
func uploadCategoryPhoto(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > maxPreviewImageBytes {
		return "", fmt.Errorf("image must be under 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if _, ok := imageMIMETypes[ext]; !ok {
		return "", fmt.Errorf("unsupported image format %s", ext)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	compressed, err := imageutil.CompressFast(src, imageutil.MaxCommunityPixels, imageutil.JPEGQuality)
	if err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	fileKey := fmt.Sprintf("market/categories/%s.jpg", uuid.New().String())
	if err := storage.Store.Upload(context.Background(), fileKey, "image/jpeg", bytes.NewReader(compressed), int64(len(compressed))); err != nil {
		return "", err
	}
	return fileKey, nil
}

// HandleAdminUpdateCategory godoc
// @Summary     Rename or reorder a Design Market section (admin)
// @Tags        Admin Market Categories
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Category ID"
// @Router      /market/categories/{id} [put]
func HandleAdminUpdateCategory(c *fiber.Ctx) error {
	var cat models.DesignCategory
	if err := database.DB.First(&cat, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "section not found")
	}
	if cat.IsOther {
		return response.BadRequest(c, "the Other section cannot be edited")
	}

	var body struct {
		Name         *string `json:"name"`
		DisplayOrder *int    `json:"display_order"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	updates := map[string]interface{}{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return response.BadRequest(c, "name cannot be empty")
		}
		updates["name"] = name
	}
	if body.DisplayOrder != nil {
		updates["display_order"] = *body.DisplayOrder
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&cat).Updates(updates).Error; err != nil {
			return response.InternalError(c, "failed to update section")
		}
		database.DB.First(&cat, "id = ?", cat.ID)
	}

	if cat.PhotoKey != nil && *cat.PhotoKey != "" {
		cat.PhotoURL = storage.Store.PublicURL(*cat.PhotoKey)
	}
	var designCount int64
	database.DB.Model(&models.Design{}).Where("category_id = ?", cat.ID).Count(&designCount)
	return response.OK(c, categoryWithCount{DesignCategory: cat, DesignCount: designCount})
}

// HandleAdminDeleteCategory godoc
// @Summary     Delete a Design Market section (admin)
// @Tags        Admin Market Categories
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Category ID"
// @Router      /market/categories/{id} [delete]
func HandleAdminDeleteCategory(c *fiber.Ctx) error {
	var cat models.DesignCategory
	if err := database.DB.First(&cat, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "section not found")
	}
	if cat.IsOther {
		return response.BadRequest(c, "the Other section cannot be deleted")
	}

	if cat.ParentID == nil {
		var childCount int64
		database.DB.Model(&models.DesignCategory{}).Where("parent_id = ?", cat.ID).Count(&childCount)
		if childCount > 0 {
			return response.BadRequest(c, "cannot delete a section that still has sub-sections — delete or move them first")
		}
	}

	var designCount int64
	database.DB.Model(&models.Design{}).Where("category_id = ?", cat.ID).Count(&designCount)
	if designCount > 0 {
		return response.BadRequest(c, "cannot delete a section that still has designs assigned to it")
	}

	if err := database.DB.Delete(&models.DesignCategory{}, "id = ?", cat.ID).Error; err != nil {
		return response.InternalError(c, "failed to delete section")
	}
	if cat.PhotoKey != nil && *cat.PhotoKey != "" {
		cleanupKeys([]string{*cat.PhotoKey})
	}
	return response.OK(c, fiber.Map{"message": "section deleted"})
}
