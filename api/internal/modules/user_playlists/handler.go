package user_playlists

import (
	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/internal/subscriptions"
	"github.com/marketkit/api/pkg/response"
)

type playlistSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnail_url"`
	VideoCount   int64  `json:"video_count"`
}

type videoWithAccess struct {
	models.Video
	Accessible      bool `json:"accessible"`
	ProgressSeconds int  `json:"progress_seconds"`
}

type playlistDetail struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	ThumbnailURL string            `json:"thumbnail_url"`
	Videos       []videoWithAccess `json:"videos"`
}

// HandleList returns all playlists with video counts (no video data).
func HandleList(c *fiber.Ctx) error {
	var playlists []models.Playlist
	if err := database.DB.Order("created_at DESC").Find(&playlists).Error; err != nil {
		return response.InternalError(c, "failed to fetch playlists")
	}

	result := make([]playlistSummary, 0, len(playlists))
	for _, p := range playlists {
		var count int64
		database.DB.Model(&models.PlaylistVideo{}).Where("playlist_id = ?", p.ID).Count(&count)
		result = append(result, playlistSummary{
			ID:           p.ID,
			Name:         p.Name,
			Description:  p.Description,
			ThumbnailURL: p.ThumbnailURL,
			VideoCount:   count,
		})
	}
	return response.OK(c, result)
}

// HandleGetDetail returns a playlist with its full video list including access flags.
func HandleGetDetail(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var p models.Playlist
	if err := database.DB.First(&p, "id = ?", c.Params("id")).Error; err != nil {
		return response.NotFound(c, "playlist not found")
	}

	// Load videos in playlist order
	var pvs []models.PlaylistVideo
	database.DB.
		Where("playlist_id = ?", p.ID).
		Order("position ASC").
		Find(&pvs)

	if len(pvs) == 0 {
		return response.OK(c, playlistDetail{
			ID: p.ID, Name: p.Name, Description: p.Description,
			ThumbnailURL: p.ThumbnailURL, Videos: []videoWithAccess{},
		})
	}

	videoIDs := make([]string, len(pvs))
	for i, pv := range pvs {
		videoIDs[i] = pv.VideoID
	}

	var videos []models.Video
	database.DB.
		Where("id IN ? AND status = ?", videoIDs, models.VideoStatusPublished).
		Find(&videos)
	storage.PopulateVideoMedia(c.Context(), videos)

	// Map videos by ID to preserve playlist order
	videoMap := make(map[string]models.Video, len(videos))
	for _, v := range videos {
		videoMap[v.ID] = v
	}

	// User subscription access
	features := subscriptions.UserFeatureAccess(userID)

	// Progress
	var progresses []models.VideoProgress
	database.DB.Where("user_id = ? AND video_id IN ?", userID, videoIDs).Find(&progresses)
	progressMap := make(map[string]int, len(progresses))
	for _, pr := range progresses {
		progressMap[pr.VideoID] = pr.PositionSeconds
	}

	result := make([]videoWithAccess, 0, len(pvs))
	for _, pv := range pvs {
		v, ok := videoMap[pv.VideoID]
		if !ok {
			continue
		}
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

	return response.OK(c, playlistDetail{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		ThumbnailURL: p.ThumbnailURL,
		Videos:       result,
	})
}
