package playlists

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/imageutil"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

func thumbKey() string {
	return fmt.Sprintf("playlists/thumbnail/thumb_%s.jpg", uuid.New().String())
}

// uploadThumb saves the uploaded "thumbnail" file and returns the public URL.
// Returns "" if no file is attached, it isn't a decodable image, or the
// upload fails. Decodes+recompresses (like avatar/photo/community uploads)
// instead of trusting the claimed filename extension.
func uploadThumb(c *fiber.Ctx) string {
	f, err := c.FormFile("thumbnail")
	if err != nil {
		return ""
	}
	src, err := f.Open()
	if err != nil {
		return ""
	}
	defer src.Close()
	data, err := imageutil.CompressFast(src, imageutil.MaxPhotoPixels, imageutil.JPEGQuality)
	if err != nil {
		return ""
	}
	key := thumbKey()
	if err := storage.Store.Upload(context.Background(), key, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
		return ""
	}
	return storage.Store.PublicURL(key)
}

type playlistResult struct {
	models.Playlist
	VideoCount   int64  `json:"video_count"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// isValidPlaylistCategory accepts the three video categories ("" allowed on
// create for legacy compatibility; such playlists never receive videos
// automatically since assignment requires a category match).
func isValidPlaylistCategory(cat string) bool {
	switch models.VideoCategory(cat) {
	case "":
		return true
	}
	return models.IsValidVideoCategory(models.VideoCategory(cat))
}

// HandleList returns all playlists with their video counts and cover thumbnails.
// If a playlist has no custom thumbnail_url, it falls back to the first video's thumbnail.
func HandleList(c *fiber.Ctx) error {
	var playlists []models.Playlist
	if err := database.DB.Order("created_at DESC").Find(&playlists).Error; err != nil {
		return response.InternalError(c, "failed to fetch playlists")
	}

	result := make([]playlistResult, 0, len(playlists))
	for _, p := range playlists {
		var count int64
		database.DB.Model(&models.PlaylistVideo{}).Where("playlist_id = ?", p.ID).Count(&count)

		thumb := p.ThumbnailURL
		if thumb == "" {
			var v models.Video
			database.DB.
				Joins("INNER JOIN playlist_videos ON playlist_videos.video_id = videos.id").
				Where("playlist_videos.playlist_id = ?", p.ID).
				Order("playlist_videos.position ASC").
				Select("videos.thumbnail_url").
				First(&v)
			if v.ThumbnailURL != nil && *v.ThumbnailURL != "" {
				thumb = *v.ThumbnailURL
			}
		}

		result = append(result, playlistResult{Playlist: p, VideoCount: count, ThumbnailURL: thumb})
	}
	return response.OK(c, result)
}

// HandleCreate creates a new playlist.
// Accepts multipart/form-data (with optional "thumbnail" file) or JSON.
func HandleCreate(c *fiber.Ctx) error {
	var name, description, thumbnailURL, category string

	ct := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ct, "multipart/form-data") {
		name = strings.TrimSpace(c.FormValue("name"))
		description = c.FormValue("description")
		category = c.FormValue("category")
		if url := uploadThumb(c); url != "" {
			thumbnailURL = url
		}
	} else {
		var body struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			ThumbnailURL string `json:"thumbnail_url"`
			Category     string `json:"category"`
		}
		if err := c.BodyParser(&body); err != nil {
			return response.BadRequest(c, "invalid request body")
		}
		name = strings.TrimSpace(body.Name)
		description = body.Description
		thumbnailURL = body.ThumbnailURL
		category = body.Category
	}

	if name == "" {
		return response.BadRequest(c, "name is required")
	}
	if !isValidPlaylistCategory(category) {
		return response.BadRequest(c, "invalid category")
	}

	p := models.Playlist{
		Name:         name,
		Description:  description,
		ThumbnailURL: thumbnailURL,
		Category:     models.VideoCategory(category),
	}
	if err := database.DB.Create(&p).Error; err != nil {
		return response.InternalError(c, "failed to create playlist")
	}
	return response.Created(c, p)
}

// HandleUpdate updates a playlist's metadata.
// Accepts multipart/form-data (with optional "thumbnail" file) or JSON.
func HandleUpdate(c *fiber.Ctx) error {
	var p models.Playlist
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "playlist not found")
	}

	updates := map[string]interface{}{}

	ct := string(c.Request().Header.ContentType())
	if strings.HasPrefix(ct, "multipart/form-data") {
		if name := strings.TrimSpace(c.FormValue("name")); name != "" {
			updates["name"] = name
		}
		if desc := c.FormValue("description"); c.FormValue("description") != "" || desc == "" {
			updates["description"] = desc
		}
		if cat := c.FormValue("category"); cat != "" {
			if !isValidPlaylistCategory(cat) {
				return response.BadRequest(c, "invalid category")
			}
			updates["category"] = cat
		}
		if url := uploadThumb(c); url != "" {
			updates["thumbnail_url"] = url
		}
	} else {
		var body struct {
			Name         *string `json:"name"`
			Description  *string `json:"description"`
			ThumbnailURL *string `json:"thumbnail_url"`
			Category     *string `json:"category"`
		}
		if err := c.BodyParser(&body); err != nil {
			return response.BadRequest(c, "invalid request body")
		}
		if body.Name != nil {
			if *body.Name == "" {
				return response.BadRequest(c, "name cannot be empty")
			}
			updates["name"] = *body.Name
		}
		if body.Description != nil {
			updates["description"] = *body.Description
		}
		if body.ThumbnailURL != nil {
			updates["thumbnail_url"] = *body.ThumbnailURL
		}
		if body.Category != nil {
			if !isValidPlaylistCategory(*body.Category) {
				return response.BadRequest(c, "invalid category")
			}
			updates["category"] = *body.Category
		}
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&p).Updates(updates).Error; err != nil {
			return response.InternalError(c, "failed to update playlist")
		}
	}
	database.DB.First(&p, "id = ?", p.ID)
	return response.OK(c, p)
}

// HandleDelete deletes a playlist (cascade removes PlaylistVideo rows).
func HandleDelete(c *fiber.Ctx) error {
	var p models.Playlist
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "playlist not found")
	}
	if err := database.DB.Delete(&p).Error; err != nil {
		return response.InternalError(c, "failed to delete playlist")
	}
	return response.OK(c, fiber.Map{"message": "deleted"})
}

// HandleGetDetail returns a playlist with its ordered list of videos.
func HandleGetDetail(c *fiber.Ctx) error {
	var p models.Playlist
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "playlist not found")
	}

	var pvs []models.PlaylistVideo
	database.DB.Where("playlist_id = ?", p.ID).Order("position ASC").Find(&pvs)

	videoIDs := make([]string, 0, len(pvs))
	for _, pv := range pvs {
		videoIDs = append(videoIDs, pv.VideoID)
	}

	var videos []models.Video
	if len(videoIDs) > 0 {
		database.DB.Where("id IN ?", videoIDs).Find(&videos)
		byID := make(map[string]models.Video, len(videos))
		for _, v := range videos {
			byID[v.ID] = v
		}
		videos = videos[:0]
		for _, id := range videoIDs {
			if v, ok := byID[id]; ok {
				videos = append(videos, v)
			}
		}
	}

	type detail struct {
		models.Playlist
		VideoCount int            `json:"video_count"`
		Videos     []models.Video `json:"videos"`
	}

	return response.OK(c, detail{
		Playlist:   p,
		VideoCount: len(videos),
		Videos:     videos,
	})
}

// HandleSetVideos replaces all videos in a playlist, ordered by array index.
func HandleSetVideos(c *fiber.Ctx) error {
	var p models.Playlist
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "playlist not found")
	}

	var body struct {
		VideoIDs []string `json:"video_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	// A playlist only holds videos of its own category — drop mismatches
	// (the picker UI filters them out too; this is the backend guarantee).
	if p.Category != "" {
		var matching []string
		database.DB.Model(&models.Video{}).
			Where("id::text IN ? AND category = ?", body.VideoIDs, p.Category).
			Pluck("id", &matching)
		matchSet := make(map[string]bool, len(matching))
		for _, id := range matching {
			matchSet[id] = true
		}
		kept := make([]string, 0, len(body.VideoIDs))
		for _, id := range body.VideoIDs {
			if matchSet[id] {
				kept = append(kept, id)
			}
		}
		body.VideoIDs = kept
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("playlist_id = ?", p.ID).Delete(&models.PlaylistVideo{}).Error; err != nil {
			return err
		}
		for i, vid := range body.VideoIDs {
			if err := tx.Create(&models.PlaylistVideo{
				PlaylistID: p.ID,
				VideoID:    vid,
				Position:   i,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return response.InternalError(c, "failed to set playlist videos")
	}

	return response.OK(c, fiber.Map{"message": "updated", "count": len(body.VideoIDs)})
}
