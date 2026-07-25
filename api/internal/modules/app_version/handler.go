package app_version

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// versionNamePattern restricts version_name to a safe display format
// (e.g. "1.2.0") — it's stored in the DB and shown in push notifications,
// so it must not carry arbitrary/control characters.
var versionNamePattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)

// apkMagicBytes is the ZIP local-file-header signature — every valid APK is
// a ZIP archive, so this rejects anything that isn't one regardless of the
// filename extension a client claims.
var apkMagicBytes = []byte{0x50, 0x4b, 0x03, 0x04}

// notifyUpdateAvailable broadcasts a push notification to every registered
// device announcing the newly published version. Delivery is best-effort
// (same as the admin manual-broadcast endpoint) — it never blocks or fails
// the publish request itself.
func notifyUpdateAvailable(v models.AppVersion) {
	fcm.SendToAll(
		"Update Available",
		fmt.Sprintf("Version %s is now available. Go to profile and click the Update Avilable button.", v.VersionName),
		"profile",
	)
}

const maxApkBytes = 200 * 1024 * 1024 // 200 MB

// HandleGetLatest returns the most recently published version. Public — no auth.
//
// Ordered by created_at, not build_number: build_number is only meaningful as a
// per-device "do I already have this" comparison, not as a global ordering — a
// version-number reset (e.g. after switching numbering schemes pre-launch) makes a
// freshly published row have a lower build_number than an older row still in the
// table, and MAX(build_number) would wrongly keep recommending the older one.
func HandleGetLatest(c *fiber.Ctx) error {
	var v models.AppVersion
	err := database.DB.Order("created_at DESC").First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return response.OK(c, nil)
	}
	if err != nil {
		return response.InternalError(c, "failed to fetch version")
	}
	return response.OK(c, v)
}

// HandleUpload uploads an APK file to server storage and publishes a new version record.
// Admin auth required. Use multipart/form-data with fields:
// apk_file, version_name, build_number, release_notes (optional), is_mandatory (optional).
func HandleUpload(c *fiber.Ctx) error {
	versionName := c.FormValue("version_name")
	if versionName == "" || !versionNamePattern.MatchString(versionName) {
		return response.BadRequest(c, "version_name must look like 1.2.0")
	}
	buildNumber, err := strconv.Atoi(c.FormValue("build_number"))
	if err != nil || buildNumber <= 0 {
		return response.BadRequest(c, "build_number must be a positive integer")
	}

	file, err := c.FormFile("apk_file")
	if err != nil {
		return response.BadRequest(c, "apk_file is required")
	}
	if file.Size > maxApkBytes {
		return response.BadRequest(c, "APK must be under 200 MB")
	}
	if strings.ToLower(filepath.Ext(file.Filename)) != ".apk" {
		return response.BadRequest(c, "file must be a .apk")
	}

	src, err := file.Open()
	if err != nil {
		return response.InternalError(c, "failed to read uploaded file")
	}
	defer src.Close()

	header := make([]byte, len(apkMagicBytes))
	if _, err := src.Read(header); err != nil || !bytes.Equal(header, apkMagicBytes) {
		return response.BadRequest(c, "file is not a valid APK")
	}
	if _, err := src.Seek(0, 0); err != nil {
		return response.InternalError(c, "failed to read uploaded file")
	}

	// Random key, independent of admin-supplied version_name/build_number —
	// those are stored only as DB metadata, never used to build a storage
	// path (matches the pattern every other upload handler in this codebase
	// already follows, e.g. videos/photos using uuid.New()).
	fileKey := fmt.Sprintf("apk/%s.apk", uuid.New().String())
	if err := storage.Store.Upload(context.Background(), fileKey, "application/vnd.android.package-archive", src, file.Size); err != nil {
		slog.Error("[upload] failed to store APK", "key", fileKey, "error", err)
		return response.InternalError(c, "failed to save APK")
	}

	isMandatory := c.FormValue("is_mandatory") == "true"
	v := models.AppVersion{
		VersionName:  versionName,
		BuildNumber:  buildNumber,
		DownloadURL:  storage.Store.PublicURL(fileKey),
		ReleaseNotes: c.FormValue("release_notes"),
		IsMandatory:  isMandatory,
	}
	if err := database.DB.Create(&v).Error; err != nil {
		return response.InternalError(c, "failed to save version")
	}
	notifyUpdateAvailable(v)
	return response.Created(c, v)
}

// HandleCreate publishes a new app version. Admin auth required.
func HandleCreate(c *fiber.Ctx) error {
	var body struct {
		VersionName  string `json:"version_name"`
		BuildNumber  int    `json:"build_number"`
		DownloadURL  string `json:"download_url"`
		ReleaseNotes string `json:"release_notes"`
		IsMandatory  bool   `json:"is_mandatory"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}
	if body.VersionName == "" || !versionNamePattern.MatchString(body.VersionName) {
		return response.BadRequest(c, "version_name must look like 1.2.0")
	}
	if body.BuildNumber <= 0 {
		return response.BadRequest(c, "build_number must be a positive integer")
	}
	if body.DownloadURL == "" {
		return response.BadRequest(c, "download_url is required")
	}

	v := models.AppVersion{
		VersionName:  body.VersionName,
		BuildNumber:  body.BuildNumber,
		DownloadURL:  body.DownloadURL,
		ReleaseNotes: body.ReleaseNotes,
		IsMandatory:  body.IsMandatory,
	}
	if err := database.DB.Create(&v).Error; err != nil {
		return response.InternalError(c, "failed to save version")
	}
	notifyUpdateAvailable(v)
	return response.Created(c, v)
}
