package imageutil

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const (
	MaxPhotoPixels     = 1920
	MaxCommunityPixels = 1280
	JPEGQuality        = 85
)

// Compress decodes any supported image (JPEG, PNG, GIF, WebP), resizes it so
// the longest side is at most maxPx, and returns JPEG-encoded bytes at the
// given quality (1–100). Output is always JPEG regardless of input format.
// Uses the high-quality Lanczos resampler.
func Compress(r io.Reader, maxPx, quality int) ([]byte, error) {
	return compressWithFilter(r, maxPx, quality, imaging.Lanczos)
}

// CompressFast behaves like Compress but uses a much cheaper linear resampler.
// Suitable for smaller, latency-sensitive images (e.g. community thumbnails)
// where the quality difference from Lanczos is visually negligible.
func CompressFast(r io.Reader, maxPx, quality int) ([]byte, error) {
	return compressWithFilter(r, maxPx, quality, imaging.Linear)
}

func compressWithFilter(r io.Reader, maxPx, quality int, filter imaging.ResampleFilter) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	b := src.Bounds()
	if b.Dx() > maxPx || b.Dy() > maxPx {
		src = imaging.Fit(src, maxPx, maxPx, filter)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, src, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
