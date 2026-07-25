package storage

import (
	"context"
	"io"
	"time"
)

// Storage is the file backend interface. Call Init() once at startup.
type Storage interface {
	// Upload streams body to the given key. contentType is the MIME type.
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	// Delete removes the object at key. Errors are silently ignored by callers.
	Delete(ctx context.Context, key string) error
	// PublicURL returns the publicly accessible URL for photos and avatars.
	PublicURL(key string) string
	// SignedURL returns a time-limited URL for private objects (videos).
	SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// Size returns the object's byte size and whether it exists.
	Size(ctx context.Context, key string) (size int64, exists bool, err error)
}

// Store is the active storage backend, set by Init().
var Store Storage
