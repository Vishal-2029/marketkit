package videos

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/cache"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/imageutil"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
)

// generateLQIP encodes a tiny (20 px wide) blurry JPEG of the thumbnail as a
// base64 data URL. Returns "" if the image cannot be decoded.
func generateLQIP(data []byte) string {
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	small := imaging.Resize(img, 20, 0, imaging.Box)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, small, imaging.JPEG, imaging.JPEGQuality(10)); err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

var videoMIMETypes = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".mkv":  "video/x-matroska",
	".ogg":  "video/ogg",
	".m4v":  "video/mp4",
}

// assignPlaylist moves a video into the given playlist (or removes it from all
// playlists when playlistID is empty). A video belongs to at most one playlist
// in the admin UI; calling this replaces any existing assignment.
//
// A video may only live in a playlist of its own category — mismatched
// assignments are refused (the admin UI also filters the dropdown by the
// selected category, so this is a backend safety net). Free videos follow the
// same rule: matching category → added.
func assignPlaylist(videoID, playlistID string) {
	if playlistID == "" {
		database.DB.Where("video_id = ?", videoID).Delete(&models.PlaylistVideo{})
		return
	}
	var pl models.Playlist
	if err := database.DB.First(&pl, "id = ?", playlistID).Error; err != nil {
		return
	}
	var v models.Video
	if err := database.DB.First(&v, "id = ?", videoID).Error; err != nil {
		return
	}
	if pl.Category != v.Category {
		slog.Warn("[playlist] refused assignment — category mismatch",
			"video", videoID, "video_category", v.Category,
			"playlist", playlistID, "playlist_category", pl.Category)
		return
	}
	var maxPos int
	database.DB.Model(&models.PlaylistVideo{}).
		Where("playlist_id = ?", playlistID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)
	database.DB.Where("video_id = ?", videoID).Delete(&models.PlaylistVideo{})
	database.DB.Create(&models.PlaylistVideo{
		PlaylistID: playlistID,
		VideoID:    videoID,
		Position:   maxPos + 1,
	})
}

func videoObjectKey(ext string) string {
	return fmt.Sprintf("videos/%s%s", uuid.New().String(), ext)
}

func videoThumbObjectKey(ext string) string {
	return fmt.Sprintf("videos/thumbnail/thumb_%s%s", uuid.New().String(), ext)
}

// uploadValidatedThumbnail decodes+recompresses the uploaded file as an image
// (rejecting anything that isn't actually a decodable image, regardless of
// its claimed filename extension) before uploading. Returns the public URL
// and the recompressed bytes (callers may want the bytes for generateLQIP).
func uploadValidatedThumbnail(ctx context.Context, thumbFile *multipart.FileHeader) (url string, data []byte, err error) {
	src, err := thumbFile.Open()
	if err != nil {
		return "", nil, err
	}
	defer src.Close()

	data, err = imageutil.CompressFast(src, imageutil.MaxPhotoPixels, imageutil.JPEGQuality)
	if err != nil {
		return "", nil, err
	}

	key := videoThumbObjectKey(".jpg")
	if err := storage.Store.Upload(ctx, key, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
		return "", nil, err
	}
	return storage.Store.PublicURL(key), data, nil
}

func deleteStoredThumbnail(ctx context.Context, thumbnailURL *string) {
	if thumbnailURL == nil || *thumbnailURL == "" {
		return
	}
	storage.DeleteByMediaURL(ctx, *thumbnailURL)
}

type videoListCache struct {
	Videos []models.Video `json:"v"`
	Total  int64          `json:"t"`
}

// HandleList godoc
// @Summary     List all videos
// @Tags        Videos
// @Produce     json
// @Security    AdminAuth
// @Param       page  query  int  false  "Page number"
// @Param       limit  query  int  false  "Items per page"
// @Param       search  query  string  false  "Search term"
// @Param       category  query  string  false  "Category"
// @Param       status  query  string  false  "Status"
// @Param       is_free  query  string  false  "Is free (true/false)"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/videos [get]
func HandleList(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	cacheKey := fmt.Sprintf("videos:admin:%s/%s/%s/%s/%s/%s",
		c.Query("page", "1"), c.Query("limit", "20"),
		c.Query("search"), c.Query("category"),
		c.Query("status"), c.Query("is_free"),
	)
	if hit, ok := cache.Get(c.Context(), cacheKey); ok {
		var lc videoListCache
		if json.Unmarshal([]byte(hit), &lc) == nil {
			storage.PopulateVideoMedia(c.Context(), lc.Videos)
			return response.Paginated(c, lc.Videos, response.Meta{
				Page: page, Limit: limit, Total: lc.Total,
				Pages: int(math.Ceil(float64(lc.Total) / float64(limit))),
			})
		}
	}

	q := database.DB.Model(&models.Video{})
	if s := c.Query("search"); s != "" {
		q = q.Where("title ILIKE ?", "%"+s+"%")
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if isF := c.Query("is_free"); isF == "true" {
		q = q.Where("is_free = ?", true)
	} else if isF == "false" {
		q = q.Where("is_free = ?", false)
	}

	var total int64
	q.Count(&total)

	var videos []models.Video
	q.Offset((page - 1) * limit).Limit(limit).Order("uploaded_at DESC").Find(&videos)
	storage.PopulateVideoMedia(c.Context(), videos)

	if b, err := json.Marshal(videoListCache{Videos: videos, Total: total}); err == nil {
		cache.Set(c.Context(), cacheKey, string(b), 60*time.Second)
	}

	return response.Paginated(c, videos, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: int(math.Ceil(float64(total) / float64(limit))),
	})
}

// HandleGet godoc
// @Summary     Get video by ID
// @Tags        Videos
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  models.Video
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/videos/{id} [get]
func HandleGet(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	storage.PopulateVideoMedia(c.Context(), []models.Video{v})
	return response.OK(c, v)
}

// HandleCreate godoc
// @Summary     Create a new video
// @Tags        Videos
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       title  formData  string  true  "Video title"
// @Param       description  formData  string  false  "Video description"
// @Param       category  formData  string  true  "Video category"
// @Param       is_free  formData  bool  false  "Is free (default: false)"
// @Param       is_preview  formData  bool  false  "Is preview (default: false)"
// @Param       file  formData  file  true  "Video file"
// @Param       thumbnail  formData  file  false  "Thumbnail image"
// @Param       thumbnail_url  formData  string  false  "Thumbnail URL"
// @Success     201  {object}  models.Video
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /admin/videos [post]
func HandleCreate(c *fiber.Ctx) error {
	title := c.FormValue("title")
	description := c.FormValue("description")
	category := models.VideoCategory(c.FormValue("category"))

	if title == "" || category == "" {
		return response.BadRequest(c, "title and category are required")
	}

	// Paid by default; admins opt a video into free access explicitly.
	isFree := false
	if isFreeStr := c.FormValue("is_free"); isFreeStr != "" {
		if parsed, err := strconv.ParseBool(isFreeStr); err == nil {
			isFree = parsed
		}
	}
	isPreview, _ := strconv.ParseBool(c.FormValue("is_preview"))

	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "video file is required")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	mimeType, ok := videoMIMETypes[ext]
	if !ok {
		return response.BadRequest(c, "unsupported video format")
	}

	fileKey := videoObjectKey(ext)

	src, err := file.Open()
	if err != nil {
		return response.InternalError(c, "failed to read uploaded file")
	}
	defer src.Close()

	if err := storage.Store.Upload(context.Background(), fileKey, mimeType, src, file.Size); err != nil {
		slog.Error("[upload] failed to store video", "key", fileKey, "error", err)
		return response.InternalError(c, "failed to save video file")
	}

	var thumbnailURL *string
	var lqip string
	if thumbFile, err := c.FormFile("thumbnail"); err == nil {
		if url, data, err := uploadValidatedThumbnail(context.Background(), thumbFile); err != nil {
			slog.Warn("[upload] rejected invalid thumbnail", "filename", thumbFile.Filename, "error", err)
		} else {
			thumbnailURL = &url
			lqip = generateLQIP(data)
		}
	} else if t := c.FormValue("thumbnail_url"); t != "" {
		thumbnailURL = &t
	}

	video := models.Video{
		Title:        title,
		Description:  description,
		Category:     category,
		IsPreview:    isPreview,
		IsFree:       isFree,
		FileKey:      fileKey,
		ThumbnailURL: thumbnailURL,
		LQIP:         lqip,
		Status:       models.VideoStatusProcessing,
	}
	database.DB.Create(&video)
	if pid := c.FormValue("playlist_id"); pid != "" {
		go assignPlaylist(video.ID, pid)
	}
	storage.PopulateVideoMedia(c.Context(), []models.Video{video})
	go cache.DeletePattern(context.Background(), "videos:admin:*")
	go cache.DeletePattern(context.Background(), "videos:published:*")
	return response.Created(c, video)
}

// HandleUpdate godoc
// @Summary     Update video details
// @Tags        Videos
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Video ID"
// @Param       title  formData  string  false  "Video title"
// @Param       description  formData  string  false  "Video description"
// @Param       category  formData  string  false  "Video category"
// @Param       is_free  formData  bool  false  "Is free (default: false)"
// @Param       is_preview  formData  bool  false  "Is preview (default: false)"
// @Param       thumbnail  formData  file  false  "Thumbnail image"
// @Param       thumbnail_url  formData  string  false  "Thumbnail URL"
// @Success     200  {object}  models.Video
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/videos/{id} [put]
func HandleUpdate(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	oldStatus := v.Status
	updates := map[string]interface{}{}

	ct := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ct, "multipart/form-data") {
		// Edit-modal path: title, description, optional thumbnail file upload.
		if t := c.FormValue("title"); t != "" {
			updates["title"] = t
		}
		if d := c.FormValue("description"); d != "" {
			updates["description"] = d
		} else if c.FormValue("description") == "" && c.FormValue("clear_description") == "1" {
			updates["description"] = ""
		}
		if cat := models.VideoCategory(c.FormValue("category")); cat != "" {
			if !models.IsValidVideoCategory(cat) {
				return response.BadRequest(c, "invalid category")
			}
			if cat != v.Category {
				updates["category"] = cat
			}
		}

		if thumbFile, err := c.FormFile("thumbnail"); err == nil {
			if url, _, err := uploadValidatedThumbnail(context.Background(), thumbFile); err != nil {
				slog.Warn("[upload] rejected invalid thumbnail", "filename", thumbFile.Filename, "error", err)
			} else {
				deleteStoredThumbnail(context.Background(), v.ThumbnailURL)
				updates["thumbnail_url"] = url
			}
		}
		if pid := c.FormValue("playlist_id"); pid != "" || c.FormValue("clear_playlist") == "1" {
			go assignPlaylist(v.ID, pid)
		}
	} else {
		// Existing JSON path: is_preview, is_free, status, thumbnail_url string, etc.
		var body struct {
			Title        *string             `json:"title"`
			Description  *string             `json:"description"`
			Status       *models.VideoStatus `json:"status"`
			IsPreview    *bool               `json:"is_preview"`
			IsFree       *bool               `json:"is_free"`
			IsIntro      *bool               `json:"is_intro"`
			ThumbnailURL *string             `json:"thumbnail_url"`
		}
		c.BodyParser(&body)
		validStatuses := map[string]bool{
			"PUBLISHED": true, "DRAFT": true, "PROCESSING": true, "ERROR": true,
		}
		if body.Status != nil && !validStatuses[string(*body.Status)] {
			return response.BadRequest(c, "invalid status value")
		}
		// A video is only publishable once transcoding produced its HLS
		// ladder — publishing a raw upload would stream the original file
		// with no quality options (and PROCESSING/ERROR videos aren't ready).
		if body.Status != nil && *body.Status == models.VideoStatusPublished && v.HLSKey == "" {
			switch v.Status {
			case models.VideoStatusError:
				return response.BadRequest(c, "transcoding failed for this video — retry the transcode before publishing")
			default:
				return response.BadRequest(c, "video is still transcoding — it can be published once transcoding completes")
			}
		}
		if body.Title != nil && strings.TrimSpace(*body.Title) == "" {
			return response.BadRequest(c, "title cannot be empty")
		}
		if body.Title != nil {
			updates["title"] = *body.Title
		}
		if body.Description != nil {
			updates["description"] = *body.Description
		}
		if body.Status != nil {
			updates["status"] = *body.Status
		}
		if body.IsPreview != nil {
			updates["is_preview"] = *body.IsPreview
		}
		if body.IsFree != nil {
			updates["is_free"] = *body.IsFree
		}
		if body.IsIntro != nil {
			updates["is_intro"] = *body.IsIntro
			// Only one video can be the intro — clear all others first.
			if *body.IsIntro {
				database.DB.Model(&models.Video{}).Where("id != ?", v.ID).Update("is_intro", false)
			}
		}
		if body.ThumbnailURL != nil {
			newURL := strings.TrimSpace(*body.ThumbnailURL)
			oldURL := ""
			if v.ThumbnailURL != nil {
				oldURL = *v.ThumbnailURL
			}
			if newURL != oldURL {
				deleteStoredThumbnail(c.Context(), v.ThumbnailURL)
			}
			if newURL == "" {
				updates["thumbnail_url"] = nil
			} else {
				updates["thumbnail_url"] = newURL
			}
		}

		if body.Status != nil && *body.Status == models.VideoStatusPublished && oldStatus != models.VideoStatusPublished {
			thumb := ""
			if body.ThumbnailURL != nil {
				thumb = strings.TrimSpace(*body.ThumbnailURL)
			}
			if thumb == "" && v.ThumbnailURL != nil {
				thumb = *v.ThumbnailURL
			}
			title := v.Title
			defer func() {
				go fcm.SendToAllRich("🎬 New Video", title, thumb, "home")
			}()
		}
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&v).Updates(updates).Error; err != nil {
			return response.InternalError(c, "failed to update video")
		}
	}
	// Re-fetch so the response reflects the saved state, not the pre-update snapshot.
	database.DB.First(&v, "id = ?", v.ID)

	if _, changed := updates["category"]; changed {
		go database.ReassignVideoDefaultPlaylist(&v)
	}

	go cache.DeletePattern(context.Background(), "videos:admin:*")
	go cache.DeletePattern(context.Background(), "videos:published:*")
	return response.OK(c, v)
}

// HandleDelete godoc
// @Summary     Delete video
// @Tags        Videos
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /admin/videos/{id} [delete]
func HandleDelete(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	if v.FileKey != "" {
		_ = storage.Store.Delete(context.Background(), v.FileKey)
	}
	deleteStoredThumbnail(context.Background(), v.ThumbnailURL)

	// Remove related rows first — the DB FK on playback_logs.video_id is NO ACTION
	// so the video delete would fail silently without this cleanup.
	database.DB.Where("video_id = ?", v.ID).Delete(&models.PlaybackLog{})
	database.DB.Where("video_id = ?", v.ID).Delete(&models.VideoProgress{})

	// Unlink (don't delete) photos attached during upload — they remain valid,
	// independently-visible Gallery content after the source video is gone.
	database.DB.Model(&models.Photo{}).Where("video_id = ?", v.ID).Update("video_id", nil)

	if err := database.DB.Delete(&models.Video{}, "id = ?", v.ID).Error; err != nil {
		return response.InternalError(c, "failed to delete video")
	}

	adminID, _ := c.Locals("adminID").(string)
	database.DB.Create(&models.AuditLog{
		EventType:    models.EventVideoDeleted,
		ActorAdminID: &adminID,
		TargetID:     &v.ID,
		IPAddress:    c.IP(),
		Details:      models.JSONMap{"title": v.Title},
	})

	go cache.DeletePattern(context.Background(), "videos:admin:*")
	go cache.DeletePattern(context.Background(), "videos:published:*")
	return response.OK(c, fiber.Map{"message": "video deleted"})
}

// HandlePresignUpload returns a presigned R2 PUT URL so the browser can upload
// the video file directly to R2 without routing data through the API server.
// Returns {"direct":false} when local storage is active (fallback to multipart).
func HandlePresignUpload(c *fiber.Ctx) error {
	filename := c.Query("filename")
	contentType := c.Query("content_type")
	if filename == "" {
		return response.BadRequest(c, "filename is required")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := videoMIMETypes[ext]; !ok {
		return response.BadRequest(c, "unsupported video format")
	}
	if contentType == "" {
		contentType = videoMIMETypes[ext]
	}

	r2, ok := storage.Store.(*storage.R2Storage)
	if !ok {
		// Local storage — tell frontend to use regular multipart upload
		return response.OK(c, fiber.Map{"direct": false})
	}

	fileKey := videoObjectKey(ext)
	uploadURL, err := r2.PresignPutURL(context.Background(), fileKey, contentType, time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate upload URL")
	}

	return response.OK(c, fiber.Map{
		"direct":     true,
		"upload_url": uploadURL,
		"file_key":   fileKey,
	})
}

// HandleFinalize creates the video DB record after the client has uploaded the
// file directly to R2 via the presigned PUT URL from HandlePresignUpload.
func HandleFinalize(c *fiber.Ctx) error {
	fileKey := c.FormValue("file_key")
	title := c.FormValue("title")
	category := models.VideoCategory(c.FormValue("category"))

	if fileKey == "" || title == "" || category == "" {
		return response.BadRequest(c, "file_key, title and category are required")
	}
	// Prevent arbitrary storage key injection
	if !strings.HasPrefix(fileKey, "videos/") {
		return response.BadRequest(c, "invalid file_key")
	}
	ext := strings.ToLower(filepath.Ext(fileKey))
	if _, ok := videoMIMETypes[ext]; !ok {
		return response.BadRequest(c, "unsupported video format")
	}

	description := c.FormValue("description")
	// Paid by default; admins opt a video into free access explicitly.
	isFree := false
	if isFreeStr := c.FormValue("is_free"); isFreeStr != "" {
		if parsed, err := strconv.ParseBool(isFreeStr); err == nil {
			isFree = parsed
		}
	}
	isPreview, _ := strconv.ParseBool(c.FormValue("is_preview"))

	var thumbnailURL *string
	if thumbFile, err := c.FormFile("thumbnail"); err == nil {
		if url, _, err := uploadValidatedThumbnail(context.Background(), thumbFile); err != nil {
			slog.Warn("[finalize] rejected invalid thumbnail", "filename", thumbFile.Filename, "error", err)
		} else {
			thumbnailURL = &url
		}
	}

	video := models.Video{
		Title:        title,
		Description:  description,
		Category:     category,
		IsPreview:    isPreview,
		IsFree:       isFree,
		FileKey:      fileKey,
		ThumbnailURL: thumbnailURL,
		Status:       models.VideoStatusProcessing,
	}
	database.DB.Create(&video)
	if pid := c.FormValue("playlist_id"); pid != "" {
		go assignPlaylist(video.ID, pid)
	}
	storage.PopulateVideoMedia(c.Context(), []models.Video{video})
	go cache.DeletePattern(context.Background(), "videos:admin:*")
	go cache.DeletePattern(context.Background(), "videos:published:*")
	return response.Created(c, video)
}

func HandleRetry(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	// Requeue failed transcodes — and legacy videos that were published
	// without ever being transcoded (no HLS ladder), as long as the original
	// upload still exists to transcode from.
	if v.Status != models.VideoStatusError && v.HLSKey != "" {
		return response.BadRequest(c, "video is already transcoded")
	}
	if v.FileKey == "" {
		return response.BadRequest(c, "original file no longer exists — re-upload the video to transcode it")
	}
	database.DB.Model(&v).Update("status", models.VideoStatusProcessing)
	go cache.DeletePattern(context.Background(), "videos:admin:*")
	go cache.DeletePattern(context.Background(), "videos:published:*")
	return response.OK(c, fiber.Map{"message": "video requeued for processing"})
}

// HandleStream generates a time-limited URL for the video and redirects the
// client to stream directly from storage. Requires Authorization: Bearer header.
// @Summary     Stream video
// @Tags        Videos
// @Produce     octet-stream
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     307  "Redirect to video stream"
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /videos/{id}/stream [get]
func HandleStream(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	if v.FileKey == "" {
		return response.NotFound(c, "video file not available")
	}

	key := v.FileKey
	if v.HLSKey != "" {
		key = v.HLSKey
	}
	url, err := storage.Store.SignedURL(context.Background(), key, 2*time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate stream URL")
	}

	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// HandleStreamURL returns the time-limited storage URL as JSON so the web
// client can load it via Axios (Authorization: Bearer) and pass it to the
// <video> element without ever embedding the session token in the URL.
// @Summary     Get video stream URL
// @Tags        Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /videos/{id}/stream-url [get]
func HandleStreamURL(c *fiber.Ctx) error {
	var v models.Video
	if err := database.DB.First(&v, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	if v.FileKey == "" {
		return response.NotFound(c, "video file not available")
	}

	key := v.FileKey
	if v.HLSKey != "" {
		key = v.HLSKey
	}
	url, err := storage.Store.SignedURL(context.Background(), key, 2*time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate stream URL")
	}

	return response.OK(c, fiber.Map{"url": url})
}
