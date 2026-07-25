package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port   string
	AppEnv string // "development" | "production"

	// Database
	DatabaseURL string

	// JWT
	JWTSecret string
	JWTExpiry string

	// SMTP
	SMTPHost   string
	SMTPPort   int
	SMTPSecure bool
	SMTPUser   string
	SMTPPass   string
	SMTPFrom   string

	// Admin notifications
	AdminEmail string

	// Razorpay
	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string

	// Push notifications (FCM V1 API — service account JSON)
	FirebaseCredentialsFile string

	// Google OAuth (admin web redirect + mobile ID-token audiences)
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleRedirectURL     string
	GoogleAndroidClientID string // Android OAuth client — valid ID-token aud
	GoogleWebClientID     string // Web client (serverClientId) — primary mobile aud

	// Security
	SecureCookie bool

	// CORS
	CORSOrigin string

	// Frontend — where OAuth redirects the browser after Google login
	FrontendURL string

	// Upload / Storage
	UploadDir        string
	MaxFileSizeBytes int64
	ServerBaseURL    string // used by local storage to build file URLs

	// Cloudflare R2 (optional — hybrid mode when all required fields are set)
	R2Bucket     string
	R2Region     string
	R2AccessKey  string
	R2SecretKey  string
	R2Endpoint   string
	R2PublicBase string // public CDN URL, e.g. https://pub-xxx.r2.dev

	// When true, ignore R2 creds and use local disk only (debug / rollback).
	StorageLocalOnly bool

	// Cache
	RedisURL string // e.g. redis://localhost:6379; empty = caching disabled

	// Publish secret — used by scripts/publish-apk.sh to upload APK without a JWT
	PublishSecret string
}

var App *Config

func Load() error {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	cfg := &Config{}

	// Server
	cfg.Port = getEnv("PORT", "3000")
	cfg.AppEnv = getEnv("APP_ENV", "production")

	// Database
	cfg.DatabaseURL = mustGetEnv("DATABASE_URL")

	// JWT
	cfg.JWTSecret = mustGetEnv("JWT_SECRET")
	cfg.JWTExpiry = getEnv("JWT_EXPIRY", "8h")

	// SMTP
	cfg.SMTPHost = getEnv("SMTP_HOST", "localhost")
	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "1025"))
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	cfg.SMTPPort = smtpPort
	cfg.SMTPSecure = getEnv("SMTP_SECURE", "false") == "true"
	cfg.SMTPUser = getEnv("SMTP_USER", "")
	cfg.SMTPPass = getEnv("SMTP_PASS", "")
	cfg.SMTPFrom = getEnv("SMTP_FROM", "noreply@example.com")

	// Admin notifications
	cfg.AdminEmail = getEnv("ADMIN_EMAIL", cfg.SMTPFrom)

	// Razorpay
	cfg.RazorpayKeyID = getEnv("RAZORPAY_KEY_ID", "")
	cfg.RazorpayKeySecret = getEnv("RAZORPAY_KEY_SECRET", "")
	cfg.RazorpayWebhookSecret = getEnv("RAZORPAY_WEBHOOK_SECRET", "")

	// FCM V1 (optional — push notifications disabled when file is absent)
	cfg.FirebaseCredentialsFile = getEnv("FIREBASE_CREDENTIALS_FILE", "./firebase-service-account.json")

	// Google OAuth
	cfg.GoogleClientID = getEnv("GOOGLE_CLIENT_ID", "")
	cfg.GoogleClientSecret = getEnv("GOOGLE_CLIENT_SECRET", "")
	cfg.GoogleRedirectURL = getEnv("GOOGLE_REDIRECT_URL", "http://localhost:3000/api/v1/auth/google/callback")
	cfg.GoogleAndroidClientID = getEnv("GOOGLE_ANDROID_CLIENT_ID", "")
	// Web client used as google_sign_in serverClientId; falls back to GOOGLE_CLIENT_ID.
	cfg.GoogleWebClientID = getEnv("GOOGLE_WEB_CLIENT_ID", cfg.GoogleClientID)

	// Security
	// Default to secure everywhere except local dev (plain http, no TLS) —
	// SECURE_COOKIE can still be set explicitly to override either way, but
	// a forgotten env var in production must fail safe (Secure flag ON),
	// not silently ship refresh-token cookies without it.
	secureCookieDefault := "true"
	if cfg.AppEnv == "development" {
		secureCookieDefault = "false"
	}
	cfg.SecureCookie = getEnv("SECURE_COOKIE", secureCookieDefault) == "true"

	// CORS
	cfg.CORSOrigin = getEnv("CORS_ORIGIN", "http://localhost:8080,http://localhost:5173")
	cfg.FrontendURL = getEnv("FRONTEND_URL", "")

	// Upload / Storage
	cfg.UploadDir = getEnv("UPLOAD_DIR", "./uploads")
	maxSize, err := strconv.ParseInt(getEnv("MAX_FILE_SIZE_BYTES", "4294967296"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid MAX_FILE_SIZE_BYTES: %w", err)
	}
	cfg.MaxFileSizeBytes = maxSize
	cfg.ServerBaseURL = getEnv("SERVER_BASE_URL", "http://localhost:3000")

	cfg.R2Bucket = getEnv("R2_BUCKET", "")
	cfg.R2Region = getEnv("R2_REGION", "auto")
	cfg.R2AccessKey = getEnv("R2_ACCESS_KEY", "")
	cfg.R2SecretKey = getEnv("R2_SECRET_KEY", "")
	cfg.R2Endpoint = getEnv("R2_ENDPOINT", "")
	cfg.R2PublicBase = getEnv("R2_PUBLIC_BASE", "")
	cfg.StorageLocalOnly = getEnv("STORAGE_LOCAL_ONLY", "false") == "true"
	cfg.RedisURL = getEnv("REDIS_URL", "")
	cfg.PublishSecret = getEnv("PUBLISH_SECRET", "")

	App = cfg
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

// R2Configured reports whether all required Cloudflare R2 settings are present.
func (c *Config) R2Configured() bool {
	return c.R2Bucket != "" &&
		c.R2AccessKey != "" &&
		c.R2SecretKey != "" &&
		c.R2Endpoint != ""
}

// UseHybridR2 enables R2 for new uploads while keeping legacy local files readable.
func (c *Config) UseHybridR2() bool {
	return c.R2Configured() && !c.StorageLocalOnly
}
