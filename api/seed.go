//go:build ignore

package main

import (
	"log"
	"os"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/mask"
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
	plans := []models.Plan{
		{Name: "Willcom Only", PriceInPaise: 99900, HasWillcom: true, DurationDays: 365},
		{Name: "E4 Only", PriceInPaise: 99900, HasE4: true, DurationDays: 365},
		{Name: "meCAD Only", PriceInPaise: 99900, HasMecad: true, DurationDays: 365},
		{Name: "Willcom + E4", PriceInPaise: 179900, HasWillcom: true, HasE4: true, DurationDays: 365},
		{Name: "Willcom + meCAD", PriceInPaise: 179900, HasWillcom: true, HasMecad: true, DurationDays: 365},
		{Name: "Willcom + E4 + meCAD", PriceInPaise: 249900, HasWillcom: true, HasE4: true, HasMecad: true, DurationDays: 365},
	}
	for _, p := range plans {
		var existing models.Plan
		if database.DB.Where("name = ?", p.Name).First(&existing).Error != nil {
			database.DB.Create(&p)
			log.Printf("Plan created: %s (₹%.0f)", p.Name, float64(p.PriceInPaise)/100)
		} else {
			log.Printf("Plan exists:  %s (₹%.0f)", existing.Name, float64(existing.PriceInPaise)/100)
		}
	}

	log.Println("Seed complete.")
}
