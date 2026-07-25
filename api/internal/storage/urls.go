package storage

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/marketkit/api/internal/models"
)

// KeyFromMediaURL extracts the storage object key from a legacy /uploads path or full URL.
func KeyFromMediaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/uploads/") {
		return strings.TrimPrefix(raw, "/uploads/")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return ""
	}
	// Strip the leading slash and any legacy "uploads/" segment so a full local
	// URL (https://host/uploads/thumb_x.jpg) resolves to the same object key as
	// its relative form (thumb_x.jpg) — otherwise the key keeps "uploads/" and
	// fails to resolve against R2/local.
	key := strings.TrimPrefix(u.Path, "/")
	return strings.TrimPrefix(key, "uploads/")
}

// DeleteByMediaURL removes the object at the key encoded in a /uploads path or full media URL.
func DeleteByMediaURL(ctx context.Context, mediaURL string) {
	if Store == nil {
		return
	}
	key := KeyFromMediaURL(mediaURL)
	if key == "" {
		return
	}
	_ = Store.Delete(ctx, key)
}

// PopulateVideoMedia refreshes thumbnail URLs for the active storage backend and sets
// preview_url (signed video URL) for clients when no thumbnail image exists.
func PopulateVideoMedia(ctx context.Context, videos []models.Video) {
	if Store == nil {
		return
	}
	for i := range videos {
		v := &videos[i]
		if v.ThumbnailURL != nil && *v.ThumbnailURL != "" {
			if key := KeyFromMediaURL(*v.ThumbnailURL); key != "" {
				resolved := Store.PublicURL(key)
				if resolved != "" {
					v.ThumbnailURL = &resolved
				}
			}
		}
		v.PreviewURL = ""
		if v.FileKey != "" {
			if u, err := Store.SignedURL(ctx, v.FileKey, 2*time.Hour); err == nil {
				v.PreviewURL = u
			}
		}
	}
}
