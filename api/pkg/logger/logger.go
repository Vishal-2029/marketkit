package logger

import (
	"log/slog"
	"os"
)

var L *slog.Logger

func init() {
	// Defaults to the production-safe handler (JSON, Info level) so a missing
	// or misconfigured APP_ENV fails safe instead of leaving verbose Debug
	// logging on. Opt into the dev handler explicitly.
	env := os.Getenv("APP_ENV")
	var handler slog.Handler
	if env == "development" || env == "dev" || env == "local" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	L = slog.New(handler)
	slog.SetDefault(L)
}
