package main

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	_ "github.com/marketkit/api/docs"
	"github.com/marketkit/api/internal"
	"github.com/marketkit/api/internal/cache"
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/cron"
	"github.com/marketkit/api/internal/database"
	paymentsmod "github.com/marketkit/api/internal/modules/payments"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/payments"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/internal/workers"
	_ "github.com/marketkit/api/pkg/logger"
	fiberswagger "github.com/swaggo/fiber-swagger"
)

// @title           MarketKit API
// @version         1.0
// @description     REST API for the MarketKit marketplace platform. Admin routes require a Bearer JWT token. User routes require a separate User Bearer JWT token.
// @termsOfService  http://swagger.io/terms/

// @contact.name   MarketKit
// @contact.email  support@example.com

// @license.name  MIT

// @host      localhost:3000
// @BasePath  /api/v1

// @securityDefinitions.apikey  AdminAuth
// @in                          header
// @name                        Authorization
// @description                 Admin JWT — prefix with "Bearer "

// @securityDefinitions.apikey  UserAuth
// @in                          header
// @name                        Authorization
// @description                 User JWT — prefix with "Bearer "

func main() {
	// Load and validate configuration
	if err := config.Load(); err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Connect to database
	if err := database.Connect(config.App.DatabaseURL); err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	// Run auto-migrations
	if err := database.Migrate(); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	// Seed default admin account if one doesn't exist yet
	database.SeedAdmin()
	// Seed default playlists and assign existing videos by category
	database.SeedDefaultPlaylists()
	// One-time idempotent backfill of historical platform income into the ledger
	platform_wallet.Backfill()

	// Init file storage backend (local disk or hybrid with Cloudflare R2)
	// Payment gateways: register providers, then wire the modules that can own
	// a gateway order into the capture registry.
	if err := payments.Init(); err != nil {
		log.Fatalf("payments init failed: %v", err)
	}
	paymentsmod.RegisterCaptureHandlers()

	if err := storage.Init(); err != nil {
		log.Fatalf("storage init error: %v", err)
	}
	log.Printf("Storage backend: %s", storage.Mode)

	// Init Redis cache (optional — app works without it)
	cache.Init(config.App.RedisURL)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "MarketKit API v1",
		ErrorHandler: errorHandler,
		BodyLimit:    int(config.App.MaxFileSizeBytes),
		ReadTimeout:  0, // no limit — required for large video uploads
		WriteTimeout: 0, // no limit — required for large video streaming
		IdleTimeout:  120 * time.Second,
	})

	app.Use(recover.New())
	// Blanket per-IP cap as defense-in-depth against a single client hammering
	// read-heavy/expensive endpoints (dashboard aggregates, list/search) that
	// have no endpoint-specific limiter. Generous enough not to interfere with
	// normal usage; /health and /uploads (video byte-range streaming — which
	// generates many small requests per playback/seek) are excluded.
	app.Use(limiter.New(limiter.Config{
		Max:        600,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/health" || strings.HasPrefix(c.Path(), "/uploads")
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "too many requests — please slow down",
			})
		},
	}))
	// Security headers on every response. CSP/frame-ancestors are skipped for
	// /docs since Swagger UI needs inline scripts/styles to render.
	app.Use(helmet.New(helmet.Config{
		Next: func(c *fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/docs")
		},
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000, // 1 year
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self'",
	}))
	app.Use(compress.New())
	app.Use(logger.New(logger.Config{
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/health"
		},
	}))
	// AllowOrigins is sourced from CORS_ORIGIN (comma-separated allowlist) —
	// never a wildcard, since AllowCredentials is true and a wildcard origin
	// combined with credentials would let any site make authenticated requests.
	corsConfig := cors.Config{
		AllowOrigins:     config.App.CORSOrigin,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, Range",
		AllowMethods:     "GET, POST, PATCH, DELETE, OPTIONS",
		ExposeHeaders:    "Content-Range, Accept-Ranges, Content-Length, Content-Type",
		AllowCredentials: true,
	}
	// In dev the web clients run on unpredictable origins (flutter run picks a
	// random port, localhost vs 127.0.0.1 vs LAN IP), so reflect any origin
	// instead of maintaining an allowlist. Never active in production.
	if config.App.AppEnv == "development" {
		corsConfig.AllowOriginsFunc = func(origin string) bool { return true }
	}
	app.Use(cors.New(corsConfig))

	// Legacy files on local disk are always served at /uploads (hybrid keeps old media here).
	app.Static("/uploads", config.App.UploadDir, fiber.Static{
		ByteRange: true,
	})

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Swagger UI — enumerates the full API surface (routes, params, internal
	// field names), so it's dev-only. Never registered in production.
	if config.App.AppEnv == "development" {
		app.Get("/docs/*", fiberswagger.WrapHandler)
	}

	// API v1 group
	v1 := app.Group("/api/v1")

	internal.RegisterRoutes(v1)

	// Start daily subscription expiry checker
	go cron.StartSubscriptionChecker()

	// Start HLS video transcoding worker
	go workers.NewVideoTranscoder(config.App).Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on :%s", config.App.Port)
		if err := app.Listen(":" + config.App.Port); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server...")
	if err := app.ShutdownWithTimeout(30 * 1e9); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("Server stopped")
}

// errorHandler never sends raw Go errors (DB errors, panics, file paths) to
// the client. 5xx responses get a generic message plus a correlation ID; the
// real error is logged server-side against that same ID.
func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "internal server error"
	if e, ok := err.(*fiber.Error); ok && e.Code < fiber.StatusInternalServerError {
		code = e.Code
		message = e.Message
	}

	if code >= fiber.StatusInternalServerError {
		correlationID := uuid.NewString()
		slog.Error("unhandled request error",
			"correlation_id", correlationID,
			"method", c.Method(),
			"path", c.Path(),
			"error", err,
		)
		return c.Status(code).JSON(fiber.Map{
			"success":        false,
			"error":          message,
			"correlation_id": correlationID,
		})
	}

	return c.Status(code).JSON(fiber.Map{"success": false, "error": message})
}
