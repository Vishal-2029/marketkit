package photos

import (
	"context"
	"log/slog"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
)

var photoMIMETypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".png": "image/png", ".webp": "image/webp", ".gif": "image/gif",
}

func populateURLs(photos []models.Photo) {
	for i := range photos {
		photos[i].URL = storage.Store.PublicURL(photos[i].FileKey)
	}
}

// HandlePublic returns published photos — no auth required.
func HandlePublic(c *fiber.Ctx) error {
	var photos []models.Photo
	database.DB.
		Where("is_published = ?", true).
		Order("uploaded_at DESC").
		Limit(50).
		Find(&photos)
	populateURLs(photos)
	return response.OK(c, photos)
}

// HandleList godoc
// @Summary     List photos (admin)
// @Tags        Admin Photos
// @Produce     json
// @Security    AdminAuth
// @Param       page    query  int  false  "Page number (default: 1)"
// @Param       limit   query  int  false  "Items per page (default: 20)"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /photos [get]
func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var total int64
	database.DB.Model(&models.Photo{}).Count(&total)

	var photos []models.Photo
	database.DB.Offset((page - 1) * limit).Limit(limit).Order("uploaded_at DESC").Find(&photos)
	populateURLs(photos)

	return response.Paginated(c, photos, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

const maxPhotoBytes = 10 * 1024 * 1024 // 10 MB

// HandleCreate godoc
// @Summary     Create a new photo (admin)
// @Tags        Admin Photos
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       file  formData  file  true  "Photo file (max 10MB)"
// @Param       title  formData  string  true  "Photo title"
// @Param       category  formData  string  false  "Photo category (default: GENERAL)"
// @Success     201  {object}  models.Photo
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     500  {object}  map[string]string
// @Router      /photos [post]
func HandleCreate(c *fiber.Ctx) error {
	title := c.FormValue("title")
	if title == "" {
		return response.BadRequest(c, "title is required")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "image file is required")
	}

	fileKey, mimeType, compressedSize, err := UploadPhotoFile(file)
	if err != nil {
		slog.Error("[upload] failed to store photo", "error", err)
		return response.BadRequest(c, err.Error())
	}

	category := c.FormValue("category")
	if category == "" {
		category = "GENERAL"
	}

	photo := models.Photo{
		Title:     title,
		Category:  category,
		FileKey:   fileKey,
		MimeType:  mimeType,
		SizeBytes: compressedSize,
	}
	if err := database.DB.Create(&photo).Error; err != nil {
		if delErr := storage.Store.Delete(context.Background(), fileKey); delErr != nil {
			slog.Error("photos: orphan file cleanup failed", "key", fileKey, "error", delErr)
		}
		return response.InternalError(c, "failed to save photo")
	}
	photo.URL = storage.Store.PublicURL(fileKey)
	return response.Created(c, photo)
}

// HandleUpdate godoc
// @Summary     Update a photo (admin)
// @Tags        Admin Photos
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Photo ID"
// @Param       body  body  map[string]interface{}  true  "Title, category, and published status"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /photos/{id} [put]
func HandleUpdate(c *fiber.Ctx) error {
	var photo models.Photo
	if err := database.DB.First(&photo, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "photo not found")
	}

	var body struct {
		Title       *string `json:"title"`
		Category    *string `json:"category"`
		IsPublished *bool   `json:"is_published"`
	}
	c.BodyParser(&body)

	wasPublished := photo.IsPublished

	updates := map[string]interface{}{}
	if body.Title != nil {
		updates["title"] = *body.Title
	}
	if body.Category != nil {
		updates["category"] = *body.Category
	}
	if body.IsPublished != nil {
		updates["is_published"] = *body.IsPublished
	}

	database.DB.Model(&photo).Updates(updates)

	if body.IsPublished != nil && *body.IsPublished && !wasPublished {
		photoURL := storage.Store.PublicURL(photo.FileKey)
		go fcm.SendToAllRich("📸 New Photos Added", photo.Title, photoURL, "gallery")
	}

	photo.URL = storage.Store.PublicURL(photo.FileKey)
	return response.OK(c, photo)
}

// HandleDelete godoc
// @Summary     Delete a photo (admin)
// @Tags        Admin Photos
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Photo ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /photos/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	var photo models.Photo
	if err := database.DB.First(&photo, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "photo not found")
	}

	if photo.FileKey != "" {
		if err := storage.Store.Delete(context.Background(), photo.FileKey); err != nil {
			slog.Error("photos: failed to delete file", "key", photo.FileKey, "error", err)
		}
	}

	database.DB.Delete(&models.Photo{}, "id = ?", c.Params("id"))
	return response.OK(c, fiber.Map{"message": "photo deleted"})
}
