package storage

import (
	"fmt"

	"github.com/marketkit/api/internal/config"
)

// Mode describes the active storage backend ("local" or "hybrid").
var Mode string

// Init creates the storage backend from the current config and sets Store.
func Init() error {
	local := newLocal(config.App.UploadDir, config.App.ServerBaseURL)

	if !config.App.UseHybridR2() {
		Store = local
		Mode = "local"
		return nil
	}

	if config.App.R2PublicBase == "" {
		return fmt.Errorf("R2_PUBLIC_BASE is required when R2 credentials are configured")
	}

	r2, err := newR2(
		config.App.R2Bucket,
		config.App.R2Region,
		config.App.R2AccessKey,
		config.App.R2SecretKey,
		config.App.R2Endpoint,
		config.App.R2PublicBase,
	)
	if err != nil {
		return fmt.Errorf("r2 init: %w", err)
	}

	Store = newHybrid(local, r2)
	Mode = "hybrid"
	return nil
}
