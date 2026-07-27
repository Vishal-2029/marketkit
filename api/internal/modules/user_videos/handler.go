package user_videos

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/cache"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/internal/subscriptions"
	"github.com/marketkit/api/pkg/response"
)

type videoWithAccess struct {
	models.Video
	Accessible      bool `json:"accessible"`
	ProgressSeconds int  `json:"progress_seconds"`
}

// HandleList godoc
// @Summary     List published videos with access flags
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       page      query  int     false  "Page number (default: 1)"
// @Param       category  query  string  false  "Category filter (CATEGORY_A, CATEGORY_B, CATEGORY_C)"
// @Success     200  {object}  []user_videos.videoWithAccess
// @Failure     401  {object}  map[string]string
// @Router      /user/videos [get]
func HandleList(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 20
	offset := (page - 1) * pageSize
	category := c.Query("category", "")

	// Fetch union of features across all active subscriptions
	features := subscriptions.UserFeatureAccess(userID)
	hasActiveSub := len(features) > 0

	// Build paginated video query with optional category filter.
	// The published video list is the same for all users, so cache it by page+category.
	var videos []models.Video
	pubCacheKey := fmt.Sprintf("videos:published:%d:%s", page, category)
	if hit, ok := cache.Get(c.Context(), pubCacheKey); ok {
		json.Unmarshal([]byte(hit), &videos)
	} else {
		q := database.DB.
			Where("status = ?", models.VideoStatusPublished).
			Order("uploaded_at DESC").
			Limit(pageSize).
			Offset(offset)
		if category != "" {
			q = q.Where("category = ?", category)
		}
		if err := q.Find(&videos).Error; err != nil {
			return response.InternalError(c, "failed to fetch videos")
		}
		if b, err := json.Marshal(videos); err == nil {
			cache.Set(c.Context(), pubCacheKey, string(b), 60*time.Second)
		}
	}
	storage.PopulateVideoMedia(c.Context(), videos)

	// Fetch all progress records for this user in one query
	var progresses []models.VideoProgress
	database.DB.Where("user_id = ?", userID).Find(&progresses)
	progressMap := make(map[string]int, len(progresses))
	for _, p := range progresses {
		progressMap[p.VideoID] = p.PositionSeconds
	}

	result := make([]videoWithAccess, 0, len(videos))
	for _, v := range videos {
		accessible := v.IsFree

		if !accessible && hasActiveSub {
			accessible = subscriptions.HasFeature(features, string(v.Category))
		}
		// A locked video must never carry a playable URL in the list
		// response — PopulateVideoMedia resolves one unconditionally, so
		// strip it here for anything the viewer isn't entitled to.
		if !accessible {
			v.PreviewURL = ""
		}

		result = append(result, videoWithAccess{
			Video:           v,
			Accessible:      accessible,
			ProgressSeconds: progressMap[v.ID],
		})
	}

	return response.OK(c, result)
}

// HandleGetIntro returns the single video flagged as is_intro, with access metadata.
// Returns {"data": null} when no intro video is configured.
func HandleGetIntro(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var v models.Video
	if err := database.DB.
		Where("is_intro = true AND status = ?", models.VideoStatusPublished).
		First(&v).Error; err != nil {
		return response.OK(c, nil)
	}
	storage.PopulateVideoMedia(c.Context(), []models.Video{v})

	var prog models.VideoProgress
	database.DB.Where("user_id = ? AND video_id = ?", userID, v.ID).First(&prog)

	// Intro videos are always accessible — they are the public teaser.
	return response.OK(c, videoWithAccess{
		Video:           v,
		Accessible:      true,
		ProgressSeconds: prog.PositionSeconds,
	})
}

// HandleGetLatest returns the 5 most recently uploaded published videos with
// access metadata. The intro video is excluded — it has its own dashboard
// section and must not repeat in "What's New".
func HandleGetLatest(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	features := subscriptions.UserFeatureAccess(userID)

	var videos []models.Video
	if err := database.DB.
		Distinct("videos.*").
		Joins("INNER JOIN playlist_videos ON playlist_videos.video_id = videos.id").
		Where("videos.status = ? AND videos.is_intro = false", models.VideoStatusPublished).
		Order("videos.uploaded_at DESC").
		Limit(5).
		Find(&videos).Error; err != nil {
		return response.InternalError(c, "failed to fetch latest videos")
	}
	storage.PopulateVideoMedia(c.Context(), videos)

	var progresses []models.VideoProgress
	database.DB.Where("user_id = ?", userID).Find(&progresses)
	progressMap := make(map[string]int, len(progresses))
	for _, p := range progresses {
		progressMap[p.VideoID] = p.PositionSeconds
	}

	result := make([]videoWithAccess, 0, len(videos))
	for _, v := range videos {
		accessible := v.IsFree
		if !accessible {
			accessible = subscriptions.HasFeature(features, string(v.Category))
		}
		if !accessible {
			v.PreviewURL = ""
		}
		result = append(result, videoWithAccess{
			Video:           v,
			Accessible:      accessible,
			ProgressSeconds: progressMap[v.ID],
		})
	}
	return response.OK(c, result)
}

// HandleLogPlayback godoc
// @Summary     Log a video playback event
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/playback-log [post]
func HandleLogPlayback(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")

	log := models.PlaybackLog{
		UserID:  userID,
		VideoID: videoID,
	}
	if err := database.DB.Create(&log).Error; err != nil {
		slog.Error("user_videos: failed to log playback", "user", userID, "video", videoID, "error", err)
	}
	return response.OK(c, fiber.Map{"message": "logged"})
}

// HandleSaveProgress godoc
// @Summary     Save video playback progress
// @Tags        User Videos
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       id    path  string              true  "Video ID"
// @Param       body  body  map[string]int      true  "position_seconds"
// @Success     200  {object}  models.VideoProgress
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/progress [post]
func HandleSaveProgress(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")

	var body struct {
		PositionSeconds int `json:"position_seconds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	// Upsert: insert first visit, update on subsequent calls
	if err := database.DB.Exec(`
		INSERT INTO video_progress (user_id, video_id, position_seconds, updated_at)
		VALUES (?, ?, ?, NOW())
		ON CONFLICT (user_id, video_id) DO UPDATE SET
			position_seconds = EXCLUDED.position_seconds,
			updated_at = EXCLUDED.updated_at
	`, userID, videoID, body.PositionSeconds).Error; err != nil {
		slog.Error("user_videos: failed to save progress", "user", userID, "video", videoID, "error", err)
	}
	progress := models.VideoProgress{
		UserID:          userID,
		VideoID:         videoID,
		PositionSeconds: body.PositionSeconds,
		UpdatedAt:       time.Now(),
	}

	return response.OK(c, progress)
}

// HandleDeleteProgress godoc
// @Summary     Remove a video from continue watching (clears saved progress)
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/progress [delete]
func HandleDeleteProgress(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	database.DB.Where("user_id = ? AND video_id = ?", userID, c.Params("id")).
		Delete(&models.VideoProgress{})
	return response.OK(c, fiber.Map{"message": "progress cleared"})
}

// HandleStream godoc
// @Summary     Stream a video (redirect to signed URL)
// @Tags        User Videos
// @Produce     octet-stream
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     307  "Redirect to signed video URL"
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/stream [get]
func HandleStream(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var v models.Video
	if err := database.DB.First(&v, "id = ? AND status = ?", c.Params("id"), models.VideoStatusPublished).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	if v.FileKey == "" && v.HLSKey == "" {
		return response.NotFound(c, "video file not available")
	}

	// Free and preview videos are accessible to any authenticated user.
	// All other videos require an active subscription that covers the category.
	if !v.IsFree && !v.IsPreview {
		features := subscriptions.UserFeatureAccess(userID)
		if len(features) == 0 {
			return response.Forbidden(c, "subscription required to watch this video")
		}

		// A category outside the known set stays accessible to any subscriber,

		// matching the behaviour before feature keys replaced the flags.

		hasAccess := true

		if models.IsValidVideoCategory(v.Category) {

			hasAccess = subscriptions.HasFeature(features, string(v.Category))

		}
		if !hasAccess {
			return response.Forbidden(c, "your plan does not include access to this content")
		}
	}

	streamKey := v.FileKey
	if v.HLSKey != "" {
		streamKey = v.HLSKey
	}
	url, err := storage.Store.SignedURL(context.Background(), streamKey, 2*time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate stream URL")
	}

	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// HandleStreamURL godoc
// @Summary     Get signed video stream URL as JSON
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/stream-url [get]
func HandleStreamURL(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var v models.Video
	if err := database.DB.First(&v, "id = ? AND status = ?", c.Params("id"), models.VideoStatusPublished).Error; err != nil {
		return response.NotFound(c, "video not found")
	}
	if v.FileKey == "" && v.HLSKey == "" {
		return response.NotFound(c, "video file not available")
	}

	if !v.IsFree && !v.IsPreview {
		features := subscriptions.UserFeatureAccess(userID)
		if len(features) == 0 {
			return response.Forbidden(c, "subscription required to watch this video")
		}

		// A category outside the known set stays accessible to any subscriber,

		// matching the behaviour before feature keys replaced the flags.

		hasAccess := true

		if models.IsValidVideoCategory(v.Category) {

			hasAccess = subscriptions.HasFeature(features, string(v.Category))

		}
		if !hasAccess {
			return response.Forbidden(c, "your plan does not include access to this content")
		}
	}

	// Optional: ?variant=v1 returns a signed URL for a specific HLS variant playlist.
	// Omit or "auto" to get the master adaptive playlist.
	variant := c.Query("variant", "")
	var streamKey string
	if variant != "" && variant != "auto" && v.HLSKey != "" {
		streamKey = fmt.Sprintf("videos/hls/%s/%s/index.m3u8", v.ID, variant)
		if _, exists, err := storage.Store.Size(context.Background(), streamKey); !exists {
			if err != nil {
				slog.Error("[stream] storage lookup failed", "video_id", v.ID, "key", streamKey, "error", err)
			}
			return response.NotFound(c, "this quality is not available for this video")
		}
	} else {
		streamKey = v.FileKey
		if v.HLSKey != "" {
			streamKey = v.HLSKey
		}
	}

	// Cache signed URLs so repeated plays skip the presign operation entirely.
	cacheKey := fmt.Sprintf("stream:%s:%s", v.ID, variant)
	if cached, ok := cache.Get(c.Context(), cacheKey); ok && cached != "" {
		return response.OK(c, fiber.Map{"url": cached})
	}

	url, err := storage.Store.SignedURL(context.Background(), streamKey, 2*time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate stream URL")
	}
	cache.Set(c.Context(), cacheKey, url, 110*time.Minute)

	return response.OK(c, fiber.Map{"url": url})
}

// HandleDownloadURL godoc
// @Summary     Get a signed download URL for a specific quality MP4
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id      path   string  true   "Video ID"
// @Param       quality query  string  false  "Quality: 240p | 360p | 480p | 720p | 1080p (default 720p)"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/download-url [get]
func HandleDownloadURL(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	quality := c.Query("quality", "720p")

	var v models.Video
	if err := database.DB.First(&v, "id = ? AND status = ?", c.Params("id"), models.VideoStatusPublished).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	if !v.IsFree && !v.IsPreview {
		features := subscriptions.UserFeatureAccess(userID)
		if len(features) == 0 {
			return response.Forbidden(c, "subscription required to download this video")
		}
		// A category outside the known set stays accessible to any subscriber,

		// matching the behaviour before feature keys replaced the flags.

		hasAccess := true

		if models.IsValidVideoCategory(v.Category) {

			hasAccess = subscriptions.HasFeature(features, string(v.Category))

		}
		if !hasAccess {
			return response.Forbidden(c, "your plan does not include access to this content")
		}
	}

	// Resolve per-quality MP4, falling back through lower qualities then original file.
	fallbackOrder := []string{quality, "720p", "480p", "360p", "240p", "1080p"}
	var downloadKey string
	for _, q := range fallbackOrder {
		if k := v.GetMP4Key(q); k != "" {
			downloadKey = k
			break
		}
	}
	if downloadKey == "" {
		downloadKey = v.FileKey
	}
	if downloadKey == "" {
		return response.NotFound(c, "video file not available")
	}

	url, err := storage.Store.SignedURL(context.Background(), downloadKey, 2*time.Hour)
	if err != nil {
		return response.InternalError(c, "failed to generate download URL")
	}

	return response.OK(c, fiber.Map{"url": url})
}

// HandleGetQualities godoc
// @Summary     List available download qualities with real file sizes
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/qualities [get]
func HandleGetQualities(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var v models.Video
	if err := database.DB.First(&v, "id = ? AND status = ?", c.Params("id"), models.VideoStatusPublished).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	if !v.IsFree && !v.IsPreview {
		features := subscriptions.UserFeatureAccess(userID)
		if len(features) == 0 {
			return response.Forbidden(c, "subscription required to download this video")
		}
		// A category outside the known set stays accessible to any subscriber,

		// matching the behaviour before feature keys replaced the flags.

		hasAccess := true

		if models.IsValidVideoCategory(v.Category) {

			hasAccess = subscriptions.HasFeature(features, string(v.Category))

		}
		if !hasAccess {
			return response.Forbidden(c, "your plan does not include access to this content")
		}
	}

	type qualityOption struct {
		Quality   string `json:"quality"`
		SizeBytes int64  `json:"size_bytes"`
	}

	var qualities []qualityOption
	updates := map[string]interface{}{}
	// Legacy videos published while the transcoder's publish UPDATE was broken
	// can have per-quality MP4s sitting in storage with empty DB key columns.
	// Probe storage once per video (negative result cached) and persist what's
	// found. Videos that genuinely have no MP4s stop being re-probed.
	healCacheKey := "mp4heal:" + v.ID
	_, healChecked := cache.Get(c.Context(), healCacheKey)
	for _, q := range []string{"1080p", "720p", "480p", "360p", "240p"} {
		key := v.GetMP4Key(q)
		size := v.GetMP4Size(q)
		if key == "" {
			if healChecked {
				continue
			}
			candidate := fmt.Sprintf("videos/mp4/%s/%s.mp4", v.ID, q)
			fetched, exists, err := storage.Store.Size(context.Background(), candidate)
			if err != nil || !exists {
				continue
			}
			key, size = candidate, fetched
			// GORM's column for MP4Key1080p is mp4_key1080p (no underscore
			// before the tier) — the underscored spelling makes Postgres
			// reject the whole update.
			updates["mp4_key"+q] = candidate
			updates["mp4_size"+q] = fetched
		}
		if size <= 0 {
			// Self-healing: legacy videos transcoded before per-quality sizes were
			// tracked don't have this cached — look it up once and persist it.
			if fetched, exists, err := storage.Store.Size(context.Background(), key); err == nil && exists {
				size = fetched
				updates["mp4_size"+q] = fetched
			}
		}
		qualities = append(qualities, qualityOption{Quality: q, SizeBytes: size})
	}
	if !healChecked {
		cache.Set(c.Context(), healCacheKey, "1", 6*time.Hour)
	}
	if len(updates) > 0 {
		database.DB.Model(&v).Updates(updates)
	}

	// Streaming qualities come from the HLS ladder, not the MP4 downloads.
	// uploadHLS writes every variant playlist in the same job that sets
	// hls_key, so a published video with an HLS master always has the full
	// ladder — even when its per-quality MP4s (download-only) are missing.
	streamQualities := []string{}
	if v.HLSKey != "" {
		streamQualities = []string{"1080p", "720p", "480p", "360p", "240p"}
	}

	return response.OK(c, fiber.Map{
		"qualities":        qualities,
		"stream_qualities": streamQualities,
	})
}

// HandleGetPhotos godoc
// @Summary     List photos attached to a video
// @Tags        User Videos
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  []models.Photo
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/photos [get]
func HandleGetPhotos(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var v models.Video
	if err := database.DB.First(&v, "id = ? AND status = ?", c.Params("id"), models.VideoStatusPublished).Error; err != nil {
		return response.NotFound(c, "video not found")
	}

	// Bonus photos are part of the lesson's paid content — same entitlement
	// rule as streaming/downloading the video itself.
	if !v.IsFree && !v.IsPreview {
		features := subscriptions.UserFeatureAccess(userID)
		// A category outside the known set stays accessible to any subscriber,

		// matching the behaviour before feature keys replaced the flags.

		hasAccess := true

		if models.IsValidVideoCategory(v.Category) {

			hasAccess = subscriptions.HasFeature(features, string(v.Category))

		}
		if !hasAccess {
			return response.Forbidden(c, "your plan does not include access to this content")
		}
	}

	var list []models.Photo
	database.DB.
		Where("video_id = ? AND is_published = ?", v.ID, true).
		Order("uploaded_at ASC").
		Find(&list)
	for i := range list {
		list[i].URL = storage.Store.PublicURL(list[i].FileKey)
	}
	return response.OK(c, list)
}
