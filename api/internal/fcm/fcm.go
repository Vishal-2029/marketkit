package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// ErrTokenUnregistered is returned when FCM reports the token is no longer valid.
var ErrTokenUnregistered = errors.New("fcm: token unregistered")

var (
	tokenSource oauth2.TokenSource
	projectID   string
	setupOnce   sync.Once
)

// setup initialises the FCM client lazily on the first broadcast call so that
// the .env file is already loaded by the time credentials are read.
func setup() {
	setupOnce.Do(func() {
		credFile := os.Getenv("FIREBASE_CREDENTIALS_FILE")
		if credFile == "" {
			credFile = "./firebase-service-account.json"
		}

		data, err := os.ReadFile(credFile)
		if err != nil {
			slog.Warn("fcm: credentials file not found — push notifications disabled", "path", credFile)
			return
		}

		var sa struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(data, &sa); err != nil || sa.ProjectID == "" {
			slog.Error("fcm: invalid service account JSON — push notifications disabled")
			return
		}
		projectID = sa.ProjectID

		creds, err := google.CredentialsFromJSON(context.Background(), data, fcmScope)
		if err != nil {
			slog.Error("fcm: credentials error — push notifications disabled", "error", err)
			return
		}
		tokenSource = creds.TokenSource
		slog.Info("fcm: initialized", "project", projectID)
	})
}

func sendOne(ctx context.Context, deviceToken, title, body, imageURL, screen string) error {
	t, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("fcm: get oauth token: %w", err)
	}

	notification := map[string]string{
		"title": title,
		"body":  body,
	}
	androidNotification := map[string]string{
		"channel_id": "default_channel",
		"sound":      "default",
	}
	aps := map[string]interface{}{
		"sound": "default",
		"badge": 1,
	}
	message := map[string]interface{}{
		"token":        deviceToken,
		"notification": notification,
		// High-priority Android delivery — bypasses Doze mode
		"android": map[string]interface{}{
			"priority":     "high",
			"notification": androidNotification,
		},
		// iOS sound + badge
		"apns": map[string]interface{}{
			"payload": map[string]interface{}{
				"aps": aps,
			},
		},
	}

	// Attach image and/or screen-routing data. The top-level image drives the
	// Android big-picture style automatically when the app is backgrounded; the
	// data copy lets the foreground handler render it. The screen key tells the
	// mobile app which route to open when the notification is tapped.
	data := map[string]string{}
	if imageURL != "" {
		notification["image"] = imageURL
		androidNotification["image"] = imageURL
		aps["mutable-content"] = 1
		data["image"] = imageURL
	}
	if screen != "" {
		data["screen"] = screen
	}
	if len(data) > 0 {
		message["data"] = data
	}

	payload := map[string]interface{}{"message": message}
	rawPayload, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		// 404 = token no longer registered on FCM; caller should delete it
		if resp.StatusCode == 404 {
			return ErrTokenUnregistered
		}
		return fmt.Errorf("fcm: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// recordNotification writes the history row + per-user inbox rows synchronously,
// regardless of whether FCM is configured or any device is registered. Counts
// start at 0 and are updated by dispatchAsync once delivery finishes.
func recordNotification(title, body, audience string, userIDs []string) uint {
	log := models.NotificationLog{
		Title:    title,
		Body:     body,
		Audience: audience,
	}
	database.DB.Create(&log)
	createUserNotifications(log.ID, userIDs)
	return log.ID
}

// dispatchAsync delivers the push in the background and updates the log's
// sent/failed counts. No-op when FCM is unconfigured or there are no tokens —
// the history row has already been recorded with 0/0.
func dispatchAsync(logID uint, tokens []models.DeviceToken, title, body, imageURL, screen string) {
	if tokenSource == nil {
		slog.Warn("fcm: push skipped — firebase-service-account.json not configured")
		return
	}
	if len(tokens) == 0 {
		slog.Warn("fcm: push skipped — no device tokens registered")
		return
	}
	go func() {
		sent, failed := dispatchTokens(tokens, title, body, imageURL, screen)
		database.DB.Model(&models.NotificationLog{}).Where("id = ?", logID).
			Updates(map[string]interface{}{"sent_count": sent, "failed_count": failed})
	}()
}

// SendToAll broadcasts a push notification to every user with a registered device token.
// The optional screen argument sets the deep-link route the mobile app opens on tap
// (e.g. "gallery", "community", "home"). Omit or pass "" for no routing.
// The notification is always recorded in history; delivery is best-effort and
// skipped if firebase-service-account.json is not configured.
// Stale tokens that FCM reports as unregistered are deleted automatically.
func SendToAll(title, body string, screen ...string) error {
	s := ""
	if len(screen) > 0 {
		s = screen[0]
	}
	return SendToAllRich(title, body, "", s)
}

// SendToAllRich is SendToAll with an optional big-picture image URL and an optional
// deep-link screen name. Pass "" for no image or no screen routing.
func SendToAllRich(title, body, imageURL string, screen ...string) error {
	s := ""
	if len(screen) > 0 {
		s = screen[0]
	}
	setup()

	var tokens []models.DeviceToken
	database.DB.Find(&tokens)

	logID := recordNotification(title, body, "all", uniqueUserIDs(tokens))
	dispatchAsync(logID, tokens, title, body, imageURL, s)
	return nil
}

// SendToUser sends a push notification to all registered devices of a specific user.
// The notification is always recorded in history; delivery is best-effort and
// skipped if firebase-service-account.json is not configured.
func SendToUser(userID, title, body string) error {
	setup()

	var tokens []models.DeviceToken
	database.DB.Where("user_id = ?", userID).Find(&tokens)

	logID := recordNotification(title, body, userID, []string{userID})
	dispatchAsync(logID, tokens, title, body, "", "")
	return nil
}

// SendToSegment sends a push notification to users matching the given plan and/or
// subscription status filter. Pass empty strings to skip a filter.
// The notification is always recorded in history; delivery is best-effort and
// skipped if firebase-service-account.json is not configured.
func SendToSegment(title, body, planID, subStatus string) error {
	setup()

	q := database.DB.Model(&models.DeviceToken{}).
		Joins("JOIN subscriptions ON subscriptions.user_id = device_tokens.user_id")
	if planID != "" {
		q = q.Where("subscriptions.plan_id = ?", planID)
	}
	if subStatus != "" {
		q = q.Where("subscriptions.status = ?", subStatus)
	}

	var tokens []models.DeviceToken
	q.Find(&tokens)
	// Deduplicate tokens by ID — the subscription JOIN can produce one row per
	// subscription per token for users with multiple subscription records.
	{
		seen := make(map[string]struct{}, len(tokens))
		deduped := tokens[:0]
		for _, t := range tokens {
			if _, ok := seen[t.ID]; !ok {
				seen[t.ID] = struct{}{}
				deduped = append(deduped, t)
			}
		}
		tokens = deduped
	}

	// Build a human-readable audience label for the log.
	audience := "segment"
	if planID != "" {
		var plan models.Plan
		database.DB.First(&plan, "id = ?", planID)
		if subStatus != "" {
			audience = fmt.Sprintf("plan:%s+status:%s", plan.Name, subStatus)
		} else {
			audience = "plan:" + plan.Name
		}
	} else if subStatus != "" {
		audience = "status:" + subStatus
	}

	logID := recordNotification(title, body, audience, uniqueUserIDs(tokens))
	dispatchAsync(logID, tokens, title, body, "", "")
	return nil
}

// uniqueUserIDs extracts deduplicated user IDs from a token slice.
func uniqueUserIDs(tokens []models.DeviceToken) []string {
	seen := make(map[string]struct{}, len(tokens))
	ids := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t.UserID]; !ok {
			seen[t.UserID] = struct{}{}
			ids = append(ids, t.UserID)
		}
	}
	return ids
}

// createUserNotifications inserts one UserNotification row per user for the
// given NotificationLog so the in-app inbox and unread badge work correctly.
// After insert, each user's inbox is trimmed to the newest 30 rows so the
// table doesn't grow unbounded.
func createUserNotifications(logID uint, userIDs []string) {
	if len(userIDs) == 0 {
		return
	}
	rows := make([]models.UserNotification, len(userIDs))
	for i, uid := range userIDs {
		rows[i] = models.UserNotification{
			UserID:            uid,
			NotificationLogID: logID,
		}
	}
	database.DB.Create(&rows)

	for _, uid := range userIDs {
		trimUserNotifications(uid, 30)
	}
}

// trimUserNotifications deletes rows beyond the newest `keep` for a user.
func trimUserNotifications(userID string, keep int) {
	// Nested subquery required: Postgres disallows LIMIT directly inside NOT IN.
	database.DB.Exec(`
		DELETE FROM user_notifications
		WHERE user_id = ? AND id NOT IN (
			SELECT id FROM (
				SELECT id FROM user_notifications
				WHERE user_id = ? ORDER BY created_at DESC LIMIT ?
			) keep_rows
		)`, userID, userID, keep)
}

// dispatchTokens sends title/body to each token, deletes stale ones, and returns sent/failed counts.
func dispatchTokens(tokens []models.DeviceToken, title, body, imageURL, screen string) (sent, failed int) {
	ctx := context.Background()
	for _, t := range tokens {
		if err := sendOne(ctx, t.Token, title, body, imageURL, screen); err != nil {
			if errors.Is(err, ErrTokenUnregistered) {
				slog.Info("fcm: removing stale token", "user_id", t.UserID)
				database.DB.Delete(&t)
			} else {
				slog.Error("fcm: send failed", "user_id", t.UserID, "error", err)
				failed++
			}
		} else {
			sent++
		}
	}
	return
}
