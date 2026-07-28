package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(dsn string) error {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	DB = db
	log.Println("Database connected")
	return nil
}

func Migrate() error {
	err := DB.AutoMigrate(
		&models.Admin{},
		&models.OtpCode{},
		&models.RefreshToken{},
		&models.User{},
		&models.UserOtpCode{},
		&models.UserRefreshToken{},
		&models.Plan{},
		&models.Subscription{},
		&models.Payment{},
		&models.Video{},
		&models.Photo{},
		&models.PhotoCategory{},
		&models.UserSession{},
		&models.PlaybackLog{},
		&models.AuditLog{},
		&models.VideoProgress{},
		&models.DeviceToken{},
		&models.CommunityPost{},
		&models.CommunityReply{},
		&models.NotificationLog{},
		&models.UserNotification{},
		&models.VideoReaction{},
		&models.VideoComment{},
		&models.RefundRequest{},
		&models.AppVersion{},
		&models.Playlist{},
		&models.PlaylistVideo{},
		&models.Product{},
		&models.ProductCategory{},
		&models.ProductPurchase{},
		&models.ProductPurchaseMessage{},
		&models.MarketPlan{},
		&models.MarketPlanSubscription{},
		&models.WalletTransaction{},
		&models.WalletTopup{},
		&models.PayoutDetail{},
		&models.Withdrawal{},
		&models.PlatformSetting{},
		&models.PendingUserOTP{},
		&models.PlatformWallet{},
		&models.PlatformLedger{},
	)
	if err != nil {
		return err
	}

	// GORM inserts is_active=false for new admins when the field is omitted (Go zero value),
	// but login requires is_active=true. Repair any legacy rows on startup.
	if res := DB.Model(&models.Admin{}).
		Where("is_active = ? AND password_hash <> ''", false).
		Update("is_active", true); res.RowsAffected > 0 {
		log.Printf("Repaired %d admin account(s): set is_active=true", res.RowsAffected)
	}

	// Seed the legacy photo categories so existing photos' categories stay selectable.
	// FirstOrCreate is idempotent — safe to run on every boot.
	for _, name := range []string{"GENERAL", "CATEGORY_A", "CATEGORY_B", "CATEGORY_C"} {
		DB.Where("name = ?", name).FirstOrCreate(&models.PhotoCategory{}, models.PhotoCategory{Name: name})
	}

	// Seed wallet platform settings. FirstOrCreate is idempotent — admins can
	// change the values later without boots resetting them.
	for key, value := range map[string]string{
		models.SettingMarketFeePercent:   "10",
		models.SettingMinWithdrawalMinor: "10000", // 100.00 in the configured currency
	} {
		DB.Where("key = ?", key).FirstOrCreate(&models.PlatformSetting{}, models.PlatformSetting{Key: key, Value: value})
	}

	// Seed the singleton platform wallet row. FirstOrCreate is idempotent —
	// the balance itself is only ever mutated via platform_wallet.Apply().
	DB.Where("id = ?", models.PlatformWalletSingletonID).
		FirstOrCreate(&models.PlatformWallet{}, models.PlatformWallet{ID: models.PlatformWalletSingletonID})

	// Seed Product Market home-page categories: 6 unnamed placeholder sections,
	// each with 3-4 sub-sections, plus one real "Other" catch-all. Names are
	// renamed later directly in the DB — idempotent, only runs once.
	seedProductCategories()

	// Normalize legacy anonymized community author labels ("Learner #1234") to a
	// plain "Learner". Admin-authored rows (user_id = 'admin') keep their "Admin"
	// label. Idempotent — the anon_name guard means later boots update nothing.
	DB.Model(&models.CommunityPost{}).
		Where("user_id <> ? AND anon_name <> ?", "admin", "Learner").
		Update("anon_name", "Learner")
	DB.Model(&models.CommunityReply{}).
		Where("user_id <> ? AND anon_name <> ?", "admin", "Learner").
		Update("anon_name", "Learner")

	// Backfill thread_user_id for legacy video_comments rows created before the
	// private-messaging migration. All pre-existing rows were user-authored (no
	// admin-reply concept existed), so thread_user_id = user_id. Idempotent — the
	// empty-string guard means later boots update nothing once backfilled.
	DB.Model(&models.VideoComment{}).
		Where("thread_user_id = '' OR thread_user_id IS NULL").
		Update("thread_user_id", gorm.Expr("user_id"))

	log.Println("Database migrated")
	return nil
}

type defaultPlaylistEntry struct {
	Name     string
	Category models.VideoCategory
}

// defaultPlaylists maps each video category to its auto-curated default
// playlist. Every published video of a category — free or paid — belongs in
// its category playlist; a video is only ever auto-added to the playlist
// matching its own category.
var defaultPlaylists = []defaultPlaylistEntry{
	{"Category A", models.CategoryA},
	{"Category B", models.CategoryB},
	{"Category C", models.CategoryC},
}

// SeedDefaultPlaylists creates the three category playlists if they don't
// exist, stamps their category (so playlist↔video category matching works),
// and assigns all existing published videos of each category that aren't in
// any playlist yet. Idempotent — safe to run on every boot.
func SeedDefaultPlaylists() {
	for _, e := range defaultPlaylists {
		var pl models.Playlist
		DB.Where("name = ?", e.Name).FirstOrCreate(&pl, models.Playlist{Name: e.Name, Category: e.Category})
		if pl.Category != e.Category {
			DB.Model(&pl).Update("category", e.Category)
		}

		// Assign any published videos of this category that are not yet in
		// any playlist (free and paid alike).
		var videos []models.Video
		DB.Where("category = ? AND status = ?", e.Category, models.VideoStatusPublished).Find(&videos)

		for _, v := range videos {
			assignToDefaultPlaylistIfEligible(v.ID, pl.ID)
		}
	}
	log.Println("Default playlists seeded")
}

// ReassignVideoDefaultPlaylist moves a video out of whichever category
// default playlist it's currently in (if any) and into the one matching its
// current category. Call this after changing a video's category — otherwise
// AssignVideoToDefaultPlaylist's "already in a playlist" guard means it would
// silently stay listed under its old category forever. Only touches the
// three named default playlists; any custom playlist the video was manually
// added to is left alone.
func ReassignVideoDefaultPlaylist(v *models.Video) {
	names := make([]string, len(defaultPlaylists))
	for i, e := range defaultPlaylists {
		names[i] = e.Name
	}
	var defaultPlaylistIDs []string
	DB.Model(&models.Playlist{}).Where("name IN ?", names).Pluck("id", &defaultPlaylistIDs)
	if len(defaultPlaylistIDs) > 0 {
		DB.Where("playlist_id::text IN ? AND video_id = ?", defaultPlaylistIDs, v.ID).
			Delete(&models.PlaylistVideo{})
	}
	AssignVideoToDefaultPlaylist(v)
}

// AssignVideoToDefaultPlaylist adds a single freshly-published video to its
// category's default playlist (free and paid alike), unless it's already in a
// playlist (e.g. the admin picked one at upload). Called right after a video
// finishes transcoding so it doesn't have to wait for a server restart to
// appear in its category playlist.
func AssignVideoToDefaultPlaylist(v *models.Video) {
	var entry *defaultPlaylistEntry
	for i := range defaultPlaylists {
		if defaultPlaylists[i].Category == v.Category {
			entry = &defaultPlaylists[i]
			break
		}
	}
	if entry == nil {
		return
	}

	var pl models.Playlist
	DB.Where("name = ?", entry.Name).
		FirstOrCreate(&pl, models.Playlist{Name: entry.Name, Category: entry.Category})
	assignToDefaultPlaylistIfEligible(v.ID, pl.ID)
}

func assignToDefaultPlaylistIfEligible(videoID, playlistID string) {
	var count int64
	DB.Model(&models.PlaylistVideo{}).Where("video_id = ?", videoID).Count(&count)
	if count > 0 {
		return
	}
	var maxPos int
	DB.Model(&models.PlaylistVideo{}).
		Where("playlist_id = ?", playlistID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)
	DB.Create(&models.PlaylistVideo{
		PlaylistID: playlistID,
		VideoID:    videoID,
		Position:   maxPos + 1,
	})
}

// SeedAdmin creates the default admin account if none exists.
// Reads ADMIN_EMAIL, ADMIN_NAME, ADMIN_PASSWORD from env.
func SeedAdmin() {
	email := os.Getenv("ADMIN_EMAIL")
	name := os.Getenv("ADMIN_NAME")
	if email == "" {
		email = "admin@example.com"
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if name == "" {
		name = "Admin"
	}

	var admin models.Admin
	if err := DB.Where("LOWER(email) = ?", email).First(&admin).Error; err == nil {
		// Ensure the default account is super-admin and active.
		admin.IsSuper = true
		admin.IsActive = true
		if err := DB.Save(&admin).Error; err != nil {
			log.Printf("SeedAdmin: failed to update existing admin: %v", err)
		}
		log.Printf("Admin account already exists: %s", mask.Email(email))
		return
	}

	// No existing admin — creating one requires an explicit password; there is
	// no hardcoded fallback so a forgotten env var can't seed a known credential.
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Fatal("SeedAdmin: ADMIN_PASSWORD must be set to create the initial admin account")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("SeedAdmin: failed to hash password: %v", err)
		return
	}

	admin = models.Admin{
		FirstName:    name,
		Email:        email,
		PasswordHash: string(hash),
		IsActive:     true,
		IsSuper:      true,
	}
	if err := DB.Create(&admin).Error; err != nil {
		log.Printf("SeedAdmin: failed to create admin: %v", err)
		return
	}
	log.Printf("Admin account created: %s", mask.Email(email))
}

// seedProductCategories creates the initial Product Market category tree once.
// Runs only when no categories exist yet, so admin renames/reorders made
// after the first boot are never overwritten on subsequent boots.
func seedProductCategories() {
	var count int64
	DB.Model(&models.ProductCategory{}).Count(&count)
	if count > 0 {
		return
	}

	for section := 1; section <= 6; section++ {
		parent := models.ProductCategory{
			Name:         fmt.Sprintf("Section %d", section),
			DisplayOrder: section,
		}
		if err := DB.Create(&parent).Error; err != nil {
			log.Printf("seedProductCategories: failed to create parent %d: %v", section, err)
			continue
		}
		for sub := 1; sub <= 3; sub++ {
			child := models.ProductCategory{
				ParentID:     &parent.ID,
				Name:         fmt.Sprintf("Section %d.%d", section, sub),
				DisplayOrder: sub,
			}
			if err := DB.Create(&child).Error; err != nil {
				log.Printf("seedProductCategories: failed to create child %d.%d: %v", section, sub, err)
			}
		}
	}

	other := models.ProductCategory{Name: "Other", DisplayOrder: 999, IsOther: true}
	if err := DB.Create(&other).Error; err != nil {
		log.Printf("seedProductCategories: failed to create Other category: %v", err)
	}
}
