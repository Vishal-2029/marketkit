// Package testutil is the shared test harness for API tests. It runs
// against a real Postgres — not sqlite — because the models lean on
// Postgres-only behavior (gen_random_uuid() defaults, jsonb columns, and
// SELECT ... FOR UPDATE row locking, which the seller and platform wallet
// ledgers depend on for correctness). Use `make test`, which starts a
// throwaway test Postgres (isolated from dev/prod data) and points
// TEST_DATABASE_URL at it before running `go test`.
package testutil

import (
	"os"
	"testing"

	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/storage"
)

// RunMain connects to TEST_DATABASE_URL, migrates the schema once per test
// binary, runs the package's tests, and exits the process with their status.
// Call it as the entire body of each package's TestMain:
//
//	func TestMain(m *testing.M) { testutil.RunMain(m) }
func RunMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		panic("TEST_DATABASE_URL is not set — run tests via `make test`, which starts a throwaway test Postgres and sets it")
	}
	if err := database.Connect(dsn); err != nil {
		panic("testutil: failed to connect to test database: " + err.Error())
	}
	if err := database.Migrate(); err != nil {
		panic("testutil: failed to migrate test database: " + err.Error())
	}

	// Several handlers under test fire fire-and-forget goroutines (email,
	// FCM) that dereference config.App directly — nil here would panic and
	// kill the whole test binary. Point SMTP at an address that refuses
	// connections immediately (127.0.0.1:1) so those goroutines fail fast
	// with a logged error instead of hanging or crashing.
	config.App = &config.Config{
		SMTPHost:      "127.0.0.1",
		SMTPPort:      1,
		SMTPFrom:      "test@example.com",
		AdminEmail:    "",
		UploadDir:     os.TempDir() + "/smh-test-uploads",
		ServerBaseURL: "http://localhost:3000",
	}
	if err := storage.Init(); err != nil {
		panic("testutil: failed to init storage: " + err.Error())
	}

	os.Exit(m.Run())
}

// WithTx points database.DB at a fresh transaction for the duration of fn and
// always rolls it back afterward, so tests never leave rows behind and can
// run repeatedly against the same long-lived test database. This is safe
// even though production code calls database.DB.Transaction(...) internally
// to wrap its own writes — GORM turns a Transaction() call made on an
// already-open transaction into a SAVEPOINT, so nested commits are just
// released and the outer Rollback here still discards everything.
func WithTx(t *testing.T, fn func()) {
	t.Helper()
	real := database.DB
	tx := real.Begin()
	if tx.Error != nil {
		t.Fatalf("testutil: failed to begin test transaction: %v", tx.Error)
	}
	database.DB = tx
	defer func() {
		tx.Rollback()
		database.DB = real
	}()
	fn()
}
