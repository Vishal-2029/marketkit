package playback

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleTopVideos godoc
// @Summary     Top videos by play count (admin)
// @Tags        Playback Analytics
// @Produce     json
// @Security    AdminAuth
// @Param       period  query  string  false  "Time period: '7d' or '30d' (default: 30d)"
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /playback/top-videos [get]
func HandleTopVideos(c *fiber.Ctx) error {
	period := c.Query("period", "30d")
	cutoff := time.Now().AddDate(0, 0, -30)
	if period == "7d" {
		cutoff = time.Now().AddDate(0, 0, -7)
	}

	type topRow struct {
		VideoID         string `json:"video_id"`
		Title           string `json:"title"`
		PlayCount       int64  `json:"play_count"`
		AvgWatchSeconds int64  `json:"avg_watch_seconds"`
	}
	var rows []topRow
	database.DB.Raw(`
		SELECT v.id AS video_id, v.title,
		       COUNT(pl.id) AS play_count,
		       COALESCE(AVG(pl.watched_seconds), 0)::int AS avg_watch_seconds
		FROM playback_logs pl
		JOIN videos v ON v.id = pl.video_id
		WHERE pl.played_at >= ?
		GROUP BY v.id, v.title
		ORDER BY play_count DESC
		LIMIT 20
	`, cutoff).Scan(&rows)
	return response.OK(c, rows)
}

// HandleByUser godoc
// @Summary     Playback stats grouped by user (admin)
// @Tags        Playback Analytics
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /playback/by-user [get]
func HandleByUser(c *fiber.Ctx) error {
	type userRow struct {
		UserID       string `json:"user_id"`
		UserName     string `json:"user_name"`
		TotalVideos  int64  `json:"total_videos"`
		TotalSeconds int64  `json:"total_seconds"`
	}
	var rows []userRow
	database.DB.Raw(`
		SELECT u.id AS user_id, u.name AS user_name,
		       COUNT(DISTINCT pl.video_id) AS total_videos,
		       COALESCE(SUM(pl.watched_seconds), 0) AS total_seconds
		FROM playback_logs pl
		JOIN users u ON u.id = pl.user_id
		GROUP BY u.id, u.name
		ORDER BY total_seconds DESC
		LIMIT 50
	`).Scan(&rows)
	return response.OK(c, rows)
}

// HandleDailyTrend godoc
// @Summary     Daily playback trend over last 30 days (admin)
// @Tags        Playback Analytics
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  []map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /playback/daily-trend [get]
func HandleDailyTrend(c *fiber.Ctx) error {
	type dayRow struct {
		Date      string `json:"date"`
		PlayCount int64  `json:"play_count"`
	}
	var rows []dayRow
	database.DB.Raw(`
		SELECT TO_CHAR(played_at::date, 'YYYY-MM-DD') AS date,
		       COUNT(*) AS play_count
		FROM playback_logs
		WHERE played_at >= NOW() - INTERVAL '30 days'
		GROUP BY played_at::date
		ORDER BY played_at::date ASC
	`).Scan(&rows)
	return response.OK(c, rows)
}

// HandleSummary godoc
// @Summary     Overall playback summary (admin)
// @Tags        Playback Analytics
// @Produce     json
// @Security    AdminAuth
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /playback/summary [get]
func HandleSummary(c *fiber.Ctx) error {
	var totalSessions, uniqueViewers int64
	var avgWatchSeconds float64

	database.DB.Model(&models.PlaybackLog{}).Count(&totalSessions)
	database.DB.Model(&models.PlaybackLog{}).Distinct("user_id").Count(&uniqueViewers)
	database.DB.Model(&models.PlaybackLog{}).Select("COALESCE(AVG(watched_seconds), 0)").Scan(&avgWatchSeconds)

	return response.OK(c, fiber.Map{
		"total_sessions":    totalSessions,
		"unique_viewers":    uniqueViewers,
		"avg_watch_seconds": int64(avgWatchSeconds),
	})
}
