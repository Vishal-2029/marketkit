//go:build ignore

package main

import (
	"log"
	"os"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
	"github.com/marketkit/api/pkg/money"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := database.Connect(config.App.DatabaseURL); err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// ── Admin ─────────────────────────────────────────────────────────────────
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminFirst := os.Getenv("ADMIN_FIRST_NAME")
	adminLast := os.Getenv("ADMIN_LAST_NAME")
	adminPhone := os.Getenv("ADMIN_PHONE")
	adminPass := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}
	if adminFirst == "" {
		adminFirst = "Admin"
	}
	if adminLast == "" {
		adminLast = ""
	}

	var admin models.Admin
	result := database.DB.Where("email = ?", adminEmail).First(&admin)
	if result.Error != nil {
		// Admin doesn't exist — creating one requires an explicit password; there
		// is no hardcoded fallback so a forgotten env var can't seed a known credential.
		if adminPass == "" {
			log.Fatal("ADMIN_PASSWORD must be set to create the initial admin account")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("hash password: %v", err)
		}
		admin = models.Admin{
			FirstName:    adminFirst,
			LastName:     adminLast,
			Email:        adminEmail,
			Phone:        adminPhone,
			PasswordHash: string(hash),
			IsActive:     true,
			IsSuper:      true,
		}
		if err := database.DB.Create(&admin).Error; err != nil {
			log.Fatalf("create admin: %v", err)
		}
		log.Printf("Admin created: %s %s <%s>", adminFirst, adminLast, mask.Email(adminEmail))
	} else {
		log.Printf("Admin already exists: %s %s <%s>", admin.FirstName, admin.LastName, mask.Email(adminEmail))
	}

	// ── Plans ──────────────────────────────────────────────────────────────────
	// Feature keys match the video categories in models/video.go, so these
	// plans gate content out of the box. The mix below shows both shapes the
	// model supports: à-la-carte single-category plans and bundles.
	catA := string(models.CategoryA)
	catB := string(models.CategoryB)
	catC := string(models.CategoryC)

	plans := []models.Plan{
		{Name: "Category A", PriceMinor: 99900, Features: []string{catA}, DurationDays: 365},
		{Name: "Category B", PriceMinor: 99900, Features: []string{catB}, DurationDays: 365},
		{Name: "Category C", PriceMinor: 99900, Features: []string{catC}, DurationDays: 365},
		{Name: "A + B", PriceMinor: 179900, Features: []string{catA, catB}, DurationDays: 365},
		{Name: "A + C", PriceMinor: 179900, Features: []string{catA, catC}, DurationDays: 365},
		{Name: "All Access", PriceMinor: 249900, Features: []string{catA, catB, catC}, DurationDays: 365},
	}
	for _, p := range plans {
		var existing models.Plan
		if database.DB.Where("name = ?", p.Name).First(&existing).Error != nil {
			database.DB.Create(&p)
			log.Printf("Plan created: %s (%s)", p.Name, money.Format(p.PriceMinor, config.App.PaymentCurrency))
		} else {
			log.Printf("Plan exists:  %s (%s)", existing.Name, money.Format(existing.PriceMinor, config.App.PaymentCurrency))
		}
	}

	log.Println("Seed complete.")
}
