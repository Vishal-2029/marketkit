package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

// HybridStorage writes new files to Cloudflare R2 while serving legacy files from local disk.
type HybridStorage struct {
	local *LocalStorage
	r2    *R2Storage
}

func newHybrid(local *LocalStorage, r2 *R2Storage) *HybridStorage {
	return &HybridStorage{local: local, r2: r2}
}

func (h *HybridStorage) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	return h.r2.Upload(ctx, key, contentType, body, size)
}

func (h *HybridStorage) Delete(ctx context.Context, key string) error {
	if ok, err := h.r2.Exists(ctx, key); err != nil {
		return err
	} else if ok {
		if err := h.r2.Delete(ctx, key); err != nil {
			return err
		}
	}
	if h.local.Exists(key) {
		return h.local.Delete(ctx, key)
	}
	return nil
}

func (h *HybridStorage) PublicURL(key string) string {
	// Legacy files remain on disk; new uploads go to R2 only.
	if h.local.Exists(key) {
		return h.local.PublicURL(key)
	}
	if url := h.r2.PublicURL(key); url != "" {
		return url
	}
	return h.local.PublicURL(key)
}

func (h *HybridStorage) SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if h.local.Exists(key) {
		return h.local.SignedURL(ctx, key, expiry)
	}
	return h.r2.SignedURL(ctx, key, expiry)
}

func (h *HybridStorage) Size(ctx context.Context, key string) (int64, bool, error) {
	if h.local.Exists(key) {
		return h.local.Size(ctx, key)
	}
	return h.r2.Size(ctx, key)
}

func (l *LocalStorage) Exists(key string) bool {
	_, err := os.Stat(filepath.Join(l.uploadDir, key))
	return err == nil
}
