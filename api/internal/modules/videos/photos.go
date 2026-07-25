package videos

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/photos"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/sync/errgroup"
)

const maxPhotosPerVideoUpload = 20

// HandleUploadPhotos godoc
// @Summary     Attach photos to a video (admin)
// @Tags        Admin Videos
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       id      path      string  true   "Video ID"
// @Param       title   formData  string  false  "Base title for the photos (defaults to the video title)"
// @Param       photos  formData  file    true   "One or more image files"
// @Success     201  {object}  []models.Photo
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /videos/{id}/photos [post]
func HandleUploadPhotos(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "photos are required")
	}
	fhs := form.File["photos"]
	if len(fhs) == 0 {
		return response.BadRequest(c, "at least one photo is required")
	}
	if len(fhs) > maxPhotosPerVideoUpload {
		fhs = fhs[:maxPhotosPerVideoUpload]
	}

	baseTitle := c.FormValue("title")
	if baseTitle == "" {
		baseTitle = v.Title
	}

	type uploaded struct {
		fileKey, mimeType string
		size              int64
	}
	results := make([]uploaded, len(fhs))
	var g errgroup.Group
	for i, fh := range fhs {
		i, fh := i, fh
		g.Go(func() error {
			key, mime, size, err := photos.UploadPhotoFile(fh)
			if err != nil {
				return err
			}
			results[i] = uploaded{fileKey: key, mimeType: mime, size: size}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		for _, r := range results {
			if r.fileKey == "" {
				continue
			}
			if delErr := storage.Store.Delete(context.Background(), r.fileKey); delErr != nil {
				slog.Error("videos: orphan photo cleanup failed", "key", r.fileKey, "error", delErr)
			}
		}
		return response.BadRequest(c, "invalid photo: "+err.Error())
	}

	created := make([]models.Photo, 0, len(results))
	for i, r := range results {
		title := baseTitle
		if len(results) > 1 {
			title = fmt.Sprintf("%s (%d)", baseTitle, i+1)
		}
		photo := models.Photo{
			Title:       title,
			Category:    string(v.Category),
			FileKey:     r.fileKey,
			MimeType:    r.mimeType,
			SizeBytes:   r.size,
			IsPublished: true,
			VideoID:     &v.ID,
		}
		if err := database.DB.Create(&photo).Error; err != nil {
			slog.Error("videos: failed to save photo row", "key", r.fileKey, "error", err)
			if delErr := storage.Store.Delete(context.Background(), r.fileKey); delErr != nil {
				slog.Error("videos: orphan photo cleanup failed", "key", r.fileKey, "error", delErr)
			}
			continue
		}
		photo.URL = storage.Store.PublicURL(r.fileKey)
		created = append(created, photo)
	}

	if len(created) > 0 {
		go fcm.SendToAllRich("📸 New Photos Added",
			fmt.Sprintf("%d new photo(s) added to \"%s\"", len(created), v.Title),
			created[0].URL, "gallery")
	}

	return response.Created(c, created)
}
