package sessions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

func HandleList(c *fiber.Ctx) error {
	var sessions []models.UserSession
	database.DB.Preload("User").
		Where("revoked_at IS NULL").
		Order("last_active_at DESC").
		Find(&sessions)

	// Strip the preloaded User down to just what the admin UI shows
	// (name/email) — the full model also carries phone, wallet balance, and
	// avatar for every user with an active session, none of which belongs in
	// a list of sessions.
	for i := range sessions {
		sessions[i].User = models.User{
			ID:    sessions[i].User.ID,
			Name:  sessions[i].User.Name,
			Email: sessions[i].User.Email,
		}
	}
	return response.OK(c, sessions)
}

func HandleRevoke(c *fiber.Ctx) error {
	now := time.Now()
	result := database.DB.Model(&models.UserSession{}).
		Where("id = ? AND revoked_at IS NULL", c.Params("id")).
		Update("revoked_at", now)

	if result.RowsAffected == 0 {
		return response.NotFound(c, "active session not found")
	}
	return response.OK(c, fiber.Map{"message": "session revoked"})
}

func HandleRevokeAll(c *fiber.Ctx) error {
	actorID, _ := c.Locals("adminID").(string)
	var actor models.Admin
	if err := database.DB.First(&actor, "id = ?", actorID).Error; err != nil {
		return response.Forbidden(c, "forbidden")
	}
	if !actor.IsSuper {
		return response.Forbidden(c, "only super-admins can force-logout all users")
	}

	now := time.Now()
	database.DB.Model(&models.UserSession{}).
		Where("revoked_at IS NULL").
		Update("revoked_at", now)
	database.DB.Where("1 = 1").Delete(&models.UserRefreshToken{})

	database.DB.Create(&models.AuditLog{
		EventType: models.EventForceLogout, ActorAdminID: &actorID,
		IPAddress: c.IP(), Details: models.JSONMap{"scope": "all_sessions"},
	})

	return response.OK(c, fiber.Map{"message": "all sessions revoked"})
}

// HandleBackfillLocations fills in location for active sessions that have an IP
// but no location yet. Calls ipapi.co for each. Safe to call multiple times.
func HandleBackfillLocations(c *fiber.Ctx) error {
	var sessions []models.UserSession
	database.DB.
		Where("revoked_at IS NULL AND location = '' AND ip_address <> ''").
		Find(&sessions)

	updated := 0
	for _, s := range sessions {
		loc := geolocateIP(s.IPAddress)
		if loc != "" {
			database.DB.Model(&s).Update("location", loc)
			updated++
		}
	}

	return response.OK(c, fiber.Map{"message": fmt.Sprintf("updated %d sessions", updated)})
}

type geoResponse struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country_name"`
	Error   bool   `json:"error"`
}

func geolocateIP(ipAddress string) string {
	ipAddress = strings.TrimSpace(ipAddress)
	if strings.HasPrefix(ipAddress, "::ffff:") {
		ipAddress = strings.TrimPrefix(ipAddress, "::ffff:")
	}
	if ipAddress == "" || ipAddress == "127.0.0.1" || ipAddress == "::1" || strings.HasPrefix(ipAddress, "192.168.") || strings.HasPrefix(ipAddress, "10.") {
		return ""
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://ipapi.co/%s/json/", ipAddress))
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	var geo geoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil || geo.Error {
		return ""
	}

	parts := []string{}
	if geo.City != "" {
		parts = append(parts, geo.City)
	}
	if geo.Region != "" {
		parts = append(parts, geo.Region)
	}
	return strings.Join(parts, ", ")
}
