package photos

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/marketkit/api/internal/imageutil"
	"github.com/marketkit/api/internal/storage"
)

// UploadPhotoFile validates a multipart image file (extension, size), compresses
// it via imageutil.Compress, and stores the result under a generated
// "photos/<uuid>.jpg" storage key. Output is always JPEG regardless of input
// format. Returns the storage key, mime type, and stored size.
func UploadPhotoFile(file *multipart.FileHeader) (fileKey, mimeType string, size int64, err error) {
	if file.Size > maxPhotoBytes {
		return "", "", 0, fmt.Errorf("image must be under 10 MB")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := photoMIMETypes[ext]; !ok {
		return "", "", 0, fmt.Errorf("unsupported image format")
	}

	src, err := file.Open()
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read uploaded file")
	}
	defer src.Close()

	compressed, err := imageutil.Compress(src, imageutil.MaxPhotoPixels, imageutil.JPEGQuality)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to process image")
	}

	fileKey = fmt.Sprintf("photos/%s.jpg", uuid.New().String())
	mimeType = "image/jpeg"
	size = int64(len(compressed))

	if err := storage.Store.Upload(context.Background(), fileKey, mimeType, bytes.NewReader(compressed), size); err != nil {
		return "", "", 0, fmt.Errorf("failed to save photo: %w", err)
	}

	return fileKey, mimeType, size, nil
}
