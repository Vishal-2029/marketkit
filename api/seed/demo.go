//go:build ignore

// Demo data seeder.
//
//	go run seed/demo.go        # from the api/ directory
//	make seed-demo             # from the repo root
//
// Fills an empty database with a marketplace that looks alive: sellers with
// products and earnings, buyers with wallets and purchase history, platform fee
// revenue, withdrawals, and subscriptions. Use it for local development, for
// the screenshots on your sales page, and for the public demo.
//
// Money is applied through the real ledger functions (wallet.Apply,
// platform_wallet.Apply) rather than inserted directly, so the seeded data
// satisfies the same invariants production data does:
//
//	SUM(wallet_transactions.amount_minor) per user == users.wallet_balance_minor
//	SUM(platform_ledger.amount_minor)             == platform_wallet.balance_minor
//
// Idempotent by email/name: re-running tops the data up rather than duplicating
// it. Pass -reset to delete previously seeded demo rows first.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/wallet"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/money"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// demoPassword is shared by every seeded account so the demo is easy to hand
// out. Nothing here is a real credential.
const demoPassword = "demo1234"

// demoEmailDomain marks every account this seeder creates, so -reset can find
// them without touching anything you made yourself.
const demoEmailDomain = "@demo.marketkit.test"

var rng = rand.New(rand.NewSource(20260728))

func main() {
	reset := flag.Bool("reset", false, "delete previously seeded demo data first")
	flag.Parse()

	if err := config.Load(); err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := database.Connect(config.App.DatabaseURL); err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	// Needed so each demo product gets a real downloadable file behind it.
	if err := storage.Init(); err != nil {
		log.Fatalf("storage: %v", err)
	}

	if *reset {
		resetDemo()
	}

	cur := config.App.PaymentCurrency
	log.Printf("Seeding demo data (currency: %s)…", cur)

	seedPlans()
	cats := seedCategories()
	sellers := seedUsers("seller", sellerNames, models.AppModeMarket)
	buyers := seedUsers("buyer", buyerNames, models.AppModeMarket)
	products := seedProducts(sellers, cats)

	seedTopups(buyers)
	purchases := seedPurchases(buyers, products)
	seedWithdrawals(sellers)
	seedMarketPlanSubs(sellers)
	seedLearningSubs(buyers)

	verifyLedgers()

	log.Println()
	log.Printf("Demo data ready:")
	log.Printf("  %2d sellers, %2d buyers", len(sellers), len(buyers))
	log.Printf("  %2d products across %d categories", len(products), len(cats))
	log.Printf("  %2d purchases", len(purchases))
	log.Printf("  password for every demo account: %s", demoPassword)
	log.Printf("  emails look like: seller1%s", demoEmailDomain)
}

// ── Plans ────────────────────────────────────────────────────────────────────

// seedPlans makes sure there is at least one learning plan and one market plan,
// so the subscription seeding below has something to attach to. `make seed`
// creates the full set; this only fills the gap when the demo seeder is run on
// its own.
func seedPlans() {
	var learning int64
	database.DB.Model(&models.Plan{}).Count(&learning)
	if learning == 0 {
		plans := []models.Plan{
			{Name: "Starter", PriceMinor: 99900, Features: []string{string(models.CategoryA)}, DurationDays: 365, IsActive: true},
			{Name: "All Access", PriceMinor: 249900, Features: []string{
				string(models.CategoryA), string(models.CategoryB), string(models.CategoryC),
			}, DurationDays: 365, IsActive: true},
		}
		for i := range plans {
			if err := database.DB.Create(&plans[i]).Error; err != nil {
				log.Printf("  learning plan %s: %v", plans[i].Name, err)
			}
		}
		log.Printf("  learning plans: created %d", len(plans))
	}

	var market int64
	database.DB.Model(&models.MarketPlan{}).Count(&market)
	if market == 0 {
		mp := []models.MarketPlan{
			{Name: "Seller Basic", PriceMinor: 49900, DurationDays: 30, IsActive: true,
				Description: "Lower platform fee on every sale."},
			{Name: "Seller Pro", PriceMinor: 99900, DurationDays: 30, IsActive: true,
				FeeDiscountPct: 50, FeaturedSeller: true,
				Description: "Half platform fee plus featured placement."},
		}
		for i := range mp {
			if err := database.DB.Create(&mp[i]).Error; err != nil {
				log.Printf("  market plan %s: %v", mp[i].Name, err)
			}
		}
		log.Printf("  market plans: created %d", len(mp))
	}
}

// ── Users ────────────────────────────────────────────────────────────────────

var sellerNames = []string{
	"Aria Fontaine", "Milo Vasquez", "Nadia Okonkwo", "Theo Lindqvist", "Priya Raman",
}

var buyerNames = []string{
	"Sam Whitfield", "Lena Toporov", "Kofi Mensah", "Iris Nakamura",
	"Diego Salcedo", "Hana Bergström", "Omar Haddad", "Ruth Adeyemi",
}

func seedUsers(role string, names []string, mode string) []models.User {
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	out := make([]models.User, 0, len(names))
	for i, name := range names {
		email := fmt.Sprintf("%s%d%s", role, i+1, demoEmailDomain)

		var u models.User
		if err := database.DB.Where("email = ?", email).First(&u).Error; err == nil {
			out = append(out, u)
			continue
		}
		u = models.User{
			Name:           name,
			Email:          email,
			Phone:          fmt.Sprintf("+1555%07d", 1000000+i),
			PasswordHash:   string(hash),
			Status:         models.UserStatusActive,
			CurrentAppMode: mode,
		}
		if err := database.DB.Create(&u).Error; err != nil {
			log.Fatalf("create %s: %v", email, err)
		}
		out = append(out, u)
	}
	log.Printf("  %s accounts: %d", role, len(out))
	return out
}

// ── Categories ───────────────────────────────────────────────────────────────

type catSpec struct {
	name     string
	children []string
}

var categorySpecs = []catSpec{
	{"Templates", []string{"Landing Pages", "Dashboards"}},
	{"Graphics", []string{"Icon Sets", "Illustrations"}},
	{"Audio", []string{"Loops & Samples"}},
}

func seedCategories() []models.ProductCategory {
	var leaves []models.ProductCategory
	order := 0
	for _, spec := range categorySpecs {
		parent := firstOrCreateCategory(spec.name, nil, order)
		order++
		childOrder := 0
		for _, childName := range spec.children {
			leaves = append(leaves, firstOrCreateCategory(childName, &parent.ID, childOrder))
			childOrder++
		}
	}
	log.Printf("  categories: %d parents, %d sub-sections", len(categorySpecs), len(leaves))
	return leaves
}

func firstOrCreateCategory(name string, parentID *string, order int) models.ProductCategory {
	var c models.ProductCategory
	q := database.DB.Where("name = ?", name)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	if err := q.First(&c).Error; err == nil {
		return c
	}
	c = models.ProductCategory{Name: name, ParentID: parentID, DisplayOrder: order}
	if err := database.DB.Create(&c).Error; err != nil {
		log.Fatalf("category %s: %v", name, err)
	}
	return c
}

// ── Products ─────────────────────────────────────────────────────────────────

var productTitles = []string{
	"Atlas Landing Page Kit", "Nimbus Dashboard UI", "Copper Icon Set (240)",
	"Signal Analytics Template", "Verge Portfolio Theme", "Lumen Illustration Pack",
	"Harbor Admin Starter", "Pixel Weather Icons", "Drift Ambient Loops Vol. 1",
	"Orbit Pricing Sections", "Cinder Email Templates", "Vector Map Collection",
	"Ferro Component Library", "Slate Mobile UI Kit", "Bloom Botanical Prints",
	"Static Noise Sample Pack", "Meridian Chart Widgets", "Paper Texture Bundle",
	"Cobalt Form Elements", "Sunset Gradient Library",
}

func seedProducts(sellers []models.User, cats []models.ProductCategory) []models.Product {
	out := make([]models.Product, 0, len(productTitles))
	for i, title := range productTitles {
		var p models.Product
		if err := database.DB.Where("title = ?", title).First(&p).Error; err == nil {
			out = append(out, p)
			continue
		}

		seller := sellers[i%len(sellers)]
		cat := cats[i%len(cats)]
		// Prices between 4.99 and 79.99 in major units, stored as minor.
		priceMinor := int64(499 + rng.Intn(7500))

		p = models.Product{
			SellerID:      seller.ID,
			Title:         title,
			Description:   fmt.Sprintf("%s — a ready-to-use asset pack. Demo listing seeded for local development.", title),
			PriceMinor:    priceMinor,
			FileKey:       fmt.Sprintf("market/files/demo-%02d.zip", i+1),
			FileName:      fmt.Sprintf("%s.zip", slug(title)),
			FileSizeBytes: int64(200_000 + rng.Intn(8_000_000)),
			FileFormat:    "zip",
			IsActive:      true,
			ViewCount:     rng.Intn(900),
			CategoryID:    &cat.ID,
			CreatedAt:     time.Now().Add(-time.Duration(rng.Intn(90)*24) * time.Hour),
		}
		if err := database.DB.Create(&p).Error; err != nil {
			log.Fatalf("product %s: %v", title, err)
		}
		// Without real bytes the demo has products nobody can download.
		body := []byte("MarketKit demo product: " + title + "\nReplace with your own file.\n")
		if err := storage.Store.Upload(context.Background(), p.FileKey,
			"application/zip", bytes.NewReader(body), int64(len(body))); err != nil {
			log.Printf("  warning: could not write demo file for %s: %v", title, err)
		}
		out = append(out, p)
	}
	log.Printf("  products: %d", len(out))
	return out
}

// ── Wallet top-ups ───────────────────────────────────────────────────────────

func seedTopups(buyers []models.User) {
	n := 0
	for i, b := range buyers {
		// Skip one buyer so the demo shows an empty wallet too.
		if i == len(buyers)-1 {
			continue
		}
		// Idempotent: a buyer who already topped up is left alone, otherwise
		// re-running inflates every wallet.
		var existing int64
		database.DB.Model(&models.WalletTopup{}).Where("user_id = ?", b.ID).Count(&existing)
		if existing > 0 {
			continue
		}
		amount := int64(5000 + rng.Intn(20000))

		err := database.DB.Transaction(func(tx *gorm.DB) error {
			topup := models.WalletTopup{
				UserID:      b.ID,
				AmountMinor: amount,
				Currency:    config.App.PaymentCurrency,
				Status:      models.PaymentSuccess,
				PaidAt:      ptrTime(time.Now().Add(-time.Duration(rng.Intn(60)*24) * time.Hour)),
			}
			if err := tx.Create(&topup).Error; err != nil {
				return err
			}
			_, err := wallet.Apply(tx, b.ID, models.WalletTxTopup, amount, &topup.ID,
				models.JSONMap{"seeded": true})
			return err
		})
		if err != nil {
			log.Fatalf("topup for %s: %v", b.Email, err)
		}
		n++
	}
	log.Printf("  wallet top-ups: %d", n)
}

// ── Purchases ────────────────────────────────────────────────────────────────

// seedPurchases runs the same debit → credit → platform-fee sequence the real
// wallet purchase path uses, so balances and both ledgers stay consistent.
func seedPurchases(buyers []models.User, products []models.Product) []models.ProductPurchase {
	feePct := wallet.FeePercent()
	var out []models.ProductPurchase

	for i, buyer := range buyers {
		// Each buyer takes a few products, skipping their own listings.
		for j := 0; j < 3; j++ {
			p := products[(i*3+j)%len(products)]
			if p.SellerID == buyer.ID {
				continue
			}

			// Idempotent: don't re-sell a product the buyer already owns, or
			// sales counts and platform revenue climb on every run.
			var owned int64
			database.DB.Model(&models.ProductPurchase{}).
				Where("buyer_id = ? AND product_id = ?", buyer.ID, p.ID).Count(&owned)
			if owned > 0 {
				continue
			}

			var fresh models.User
			if err := database.DB.First(&fresh, "id = ?", buyer.ID).Error; err != nil {
				continue
			}
			if fresh.WalletBalanceMinor < p.PriceMinor {
				continue // not enough balance; leave it as a realistic gap
			}

			fee, net := wallet.SplitFee(p.PriceMinor, feePct)
			purchase := models.ProductPurchase{
				ProductID:      p.ID,
				BuyerID:        buyer.ID,
				SellerID:       p.SellerID,
				AmountMinor:    p.PriceMinor,
				Currency:       config.App.PaymentCurrency,
				Status:         models.PaymentSuccess,
				FeeMinor:       fee,
				SellerNetMinor: net,
				PaidVia:        "WALLET",
				PaidAt:         ptrTime(time.Now().Add(-time.Duration(rng.Intn(45)*24) * time.Hour)),
			}

			err := database.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&purchase).Error; err != nil {
					return err
				}
				// Same lock order as production: buyer first, then seller.
				if _, err := wallet.Apply(tx, buyer.ID, models.WalletTxPurchaseDebit, -p.PriceMinor,
					&purchase.ID, models.JSONMap{"product_id": p.ID}); err != nil {
					return err
				}
				if err := tx.Model(&models.Product{}).Where("id = ?", p.ID).
					UpdateColumn("sales_count", gorm.Expr("sales_count + 1")).Error; err != nil {
					return err
				}
				if _, err := wallet.Apply(tx, p.SellerID, models.WalletTxSaleCredit, net,
					&purchase.ID, models.JSONMap{"fee_minor": fee, "product_id": p.ID}); err != nil {
					return err
				}
				if fee > 0 {
					if _, err := platform_wallet.Apply(tx, models.PlatformSourcePlatformFee, fee,
						&purchase.ID, models.JSONMap{"product_id": p.ID}); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				log.Fatalf("purchase: %v", err)
			}
			out = append(out, purchase)
		}
	}
	log.Printf("  purchases: %d (platform fee %d%%)", len(out), feePct)
	return out
}

// ── Withdrawals ──────────────────────────────────────────────────────────────

func seedWithdrawals(sellers []models.User) {
	n := 0
	for i, s := range sellers {
		var fresh models.User
		if err := database.DB.First(&fresh, "id = ?", s.ID).Error; err != nil {
			continue
		}
		// Only the first two sellers cash out, and only part of their balance.
		if i >= 2 || fresh.WalletBalanceMinor < 2000 {
			continue
		}
		var existing int64
		database.DB.Model(&models.Withdrawal{}).Where("user_id = ?", s.ID).Count(&existing)
		if existing > 0 {
			continue
		}
		amount := fresh.WalletBalanceMinor / 2

		database.DB.Where(models.PayoutDetail{UserID: s.ID}).
			Attrs(models.PayoutDetail{
				UpiID:          fmt.Sprintf("demo%d@upi", i+1),
				BankHolderName: s.Name,
			}).FirstOrCreate(&models.PayoutDetail{})

		err := database.DB.Transaction(func(tx *gorm.DB) error {
			w := models.Withdrawal{
				UserID:      s.ID,
				AmountMinor: amount,
				Method:      models.WithdrawalMethodUPI,
				UpiID:       fmt.Sprintf("demo%d@upi", i+1),
				// One still awaiting payout, one already settled — the admin
				// queue is a blank screen otherwise.
				Status: map[bool]string{true: models.WithdrawalStatusApproved,
					false: models.WithdrawalStatusSettled}[i == 0],
			}
			if err := tx.Create(&w).Error; err != nil {
				return err
			}
			_, err := wallet.Apply(tx, s.ID, models.WalletTxWithdrawal, -amount, &w.ID,
				models.JSONMap{"seeded": true})
			return err
		})
		if err != nil {
			log.Fatalf("withdrawal for %s: %v", s.Email, err)
		}
		n++
	}
	log.Printf("  withdrawals: %d", n)
}

// ── Subscriptions ────────────────────────────────────────────────────────────

func seedMarketPlanSubs(sellers []models.User) {
	var plan models.MarketPlan
	if err := database.DB.Where("is_active = true").Order("price_minor ASC").First(&plan).Error; err != nil {
		log.Printf("  market plans: none configured, skipping seller subscriptions")
		return
	}

	n := 0
	for i, s := range sellers {
		if i >= 2 {
			break // only some sellers subscribe
		}
		var existing models.MarketPlanSubscription
		if database.DB.Where("user_id = ? AND plan_id = ?", s.ID, plan.ID).
			First(&existing).Error == nil {
			continue
		}
		sub := models.MarketPlanSubscription{
			UserID:      s.ID,
			PlanID:      plan.ID,
			Status:      models.SubscriptionActive,
			StartDate:   time.Now().Add(-10 * 24 * time.Hour),
			ExpiryDate:  time.Now().AddDate(0, 0, plan.DurationDays),
			AmountMinor: plan.PriceMinor,
			PaidAt:      ptrTime(time.Now().Add(-10 * 24 * time.Hour)),
		}
		if err := database.DB.Create(&sub).Error; err != nil {
			log.Printf("  market plan sub skipped for %s: %v", s.Email, err)
			continue
		}
		_ = database.DB.Transaction(func(tx *gorm.DB) error {
			_, err := platform_wallet.Apply(tx, models.PlatformSourceMarketPlan, plan.PriceMinor,
				&sub.ID, models.JSONMap{"seeded": true})
			return err
		})
		n++
	}
	log.Printf("  market plan subscriptions: %d", n)
}

func seedLearningSubs(buyers []models.User) {
	var plan models.Plan
	if err := database.DB.Where("is_active = true").Order("price_minor ASC").First(&plan).Error; err != nil {
		log.Printf("  learning plans: none configured, skipping subscriptions")
		return
	}

	n := 0
	for i, b := range buyers {
		if i >= 3 {
			break
		}
		var existing models.Subscription
		if database.DB.Where("user_id = ? AND plan_id = ?", b.ID, plan.ID).First(&existing).Error == nil {
			continue
		}

		payment := models.Payment{
			UserID:      b.ID,
			PlanID:      plan.ID,
			AmountMinor: plan.PriceMinor,
			Currency:    config.App.PaymentCurrency,
			Status:      models.PaymentSuccess,
			Provider:    models.ProviderManual,
			PaidAt:      ptrTime(time.Now().Add(-20 * 24 * time.Hour)),
		}
		if err := database.DB.Create(&payment).Error; err != nil {
			log.Printf("  payment skipped for %s: %v", b.Email, err)
			continue
		}
		sub := models.Subscription{
			UserID:      b.ID,
			PlanID:      plan.ID,
			Status:      models.SubscriptionActive,
			ExpiryDate:  time.Now().AddDate(0, 0, plan.DurationDays),
			ActivatedBy: "demo-seed",
		}
		if err := database.DB.Create(&sub).Error; err != nil {
			continue
		}
		_ = database.DB.Transaction(func(tx *gorm.DB) error {
			_, err := platform_wallet.Apply(tx, models.PlatformSourceLearningPlan, plan.PriceMinor,
				&payment.ID, models.JSONMap{"seeded": true})
			return err
		})
		n++
	}
	log.Printf("  learning subscriptions: %d", n)
}

// ── Verification ─────────────────────────────────────────────────────────────

// verifyLedgers re-checks the invariants the ledgers promise. Seeding through
// wallet.Apply should make these hold by construction — this catches it loudly
// if a future change to the seeder starts writing balances directly.
func verifyLedgers() {
	type row struct {
		UserID  string
		Balance int64
		Summed  int64
	}
	var bad []row
	// users.id is uuid while wallet_transactions.user_id is text (GORM writes
	// the FK as a plain string), so the join needs an explicit cast — without
	// it Postgres raises "operator does not exist: text = uuid".
	res := database.DB.Raw(`
		SELECT u.id::text AS user_id,
		       u.wallet_balance_minor AS balance,
		       COALESCE(SUM(wt.amount_minor), 0) AS summed
		FROM users u
		LEFT JOIN wallet_transactions wt ON wt.user_id = u.id::text
		GROUP BY u.id, u.wallet_balance_minor
		HAVING u.wallet_balance_minor <> COALESCE(SUM(wt.amount_minor), 0)
	`).Scan(&bad)
	// A failed query must not read as a pass. Without this the check reports
	// "OK" whenever the SQL itself is broken, which is exactly backwards.
	if res.Error != nil {
		log.Fatalf("ledger check could not run: %v", res.Error)
	}
	if len(bad) > 0 {
		for _, b := range bad {
			log.Printf("  user %s: balance=%d ledger sum=%d", b.UserID, b.Balance, b.Summed)
		}
		log.Fatalf("ledger invariant broken for %d user(s): balance != SUM(transactions)", len(bad))
	}

	var checked int64
	if err := database.DB.Model(&models.WalletTransaction{}).Count(&checked).Error; err != nil {
		log.Fatalf("ledger check could not count transactions: %v", err)
	}

	var pw models.PlatformWallet
	if err := database.DB.First(&pw, "id = ?", models.PlatformWalletSingletonID).Error; err == nil {
		var summed int64
		if err := database.DB.Model(&models.PlatformLedger{}).
			Select("COALESCE(SUM(amount_minor), 0)").Scan(&summed).Error; err != nil {
			log.Fatalf("platform ledger check could not run: %v", err)
		}
		if summed != pw.BalanceMinor {
			log.Fatalf("platform ledger broken: wallet=%d ledger sum=%d", pw.BalanceMinor, summed)
		}
		log.Printf("  platform wallet: %s", money.Format(pw.BalanceMinor, config.App.PaymentCurrency))
	}
	log.Printf("  ledger invariants: OK (%d wallet transactions checked)", checked)
}

// ── Reset ────────────────────────────────────────────────────────────────────

// resetDemo removes rows this seeder created. It keys off the demo email
// domain, so anything you added yourself is left alone.
func resetDemo() {
	log.Println("Resetting demo data…")

	// Unscoped throughout: a previous soft-delete would otherwise hide the very
	// rows this needs to purge, leaving the unique email index occupied.
	var ids []string
	database.DB.Unscoped().Model(&models.User{}).
		Where("email LIKE ?", "%"+demoEmailDomain).Pluck("id", &ids)
	if len(ids) == 0 {
		log.Println("  nothing to reset")
		return
	}

	// Children first. Several FKs to users are NO ACTION rather than CASCADE
	// (payments, playback_logs, subscriptions, user_otp_codes, user_sessions),
	// so anything left behind blocks the user delete below. Logging in even
	// once creates otp_code and session rows, which is enough to wedge a reset.
	database.DB.Where("buyer_id IN ? OR seller_id IN ?", ids, ids).Delete(&models.ProductPurchase{})
	database.DB.Where("seller_id IN ?", ids).Delete(&models.Product{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.WalletTransaction{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.WalletTopup{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.Withdrawal{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.PayoutDetail{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.MarketPlanSubscription{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.Subscription{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.Payment{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.PlaybackLog{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.UserOtpCode{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.UserSession{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.UserRefreshToken{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.DeviceToken{})
	database.DB.Where("user_id IN ?", ids).Delete(&models.UserNotification{})
	// User has gorm.DeletedAt, so a plain Delete only soft-deletes: the row and
	// its unique email index survive, and the next seed fails with a duplicate
	// key. Unscoped makes -reset an actual reset.
	if err := database.DB.Unscoped().Where("id IN ?", ids).Delete(&models.User{}).Error; err != nil {
		log.Fatalf("reset: could not delete demo users (a child row still references them): %v", err)
	}

	log.Printf("  removed %d demo accounts and their data", len(ids))
	log.Println("  note: platform ledger entries are append-only and are left in place")
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func ptrTime(t time.Time) *time.Time { return &t }

func slug(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '-' || r == '_':
			out = append(out, '-')
		}
	}
	return string(out)
}
