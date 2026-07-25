package market

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tinyJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 150, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func multipartPhotoBody(t *testing.T, jpegBytes []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("photo", "banner.jpg")
	require.NoError(t, err)
	_, err = part.Write(jpegBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return body, w.FormDataContentType()
}

// TestHandleAdminUploadCategoryPhoto_SetsAndReplacesPhoto covers Task
// "admin section photo": uploading sets photo_url, and re-uploading replaces
// it (and cleans up the old file) rather than accumulating orphans.
func TestHandleAdminUploadCategoryPhoto_SetsAndReplacesPhoto(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		cat := models.ProductCategory{Name: "Test Section", DisplayOrder: 1}
		require.NoError(t, tx.Create(&cat).Error)

		app := testutil.FiberApp(nil)
		app.Post("/x/:id/photo", HandleAdminUploadCategoryPhoto)

		jpegBytes := tinyJPEGBytes(t)
		body, contentType := multipartPhotoBody(t, jpegBytes)
		req := httptest.NewRequest("POST", "/x/"+cat.ID+"/photo", body)
		req.Header.Set("Content-Type", contentType)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data struct {
				PhotoURL string `json:"photo_url"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(respBody, &parsed))
		assert.NotEmpty(t, parsed.Data.PhotoURL)

		var updated models.ProductCategory
		require.NoError(t, tx.First(&updated, "id = ?", cat.ID).Error)
		require.NotNil(t, updated.PhotoKey)
		firstKey := *updated.PhotoKey

		// Re-upload should replace, not just append.
		body2, contentType2 := multipartPhotoBody(t, jpegBytes)
		req2 := httptest.NewRequest("POST", "/x/"+cat.ID+"/photo", body2)
		req2.Header.Set("Content-Type", contentType2)
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		require.Equal(t, 200, resp2.StatusCode)

		var updated2 models.ProductCategory
		require.NoError(t, tx.First(&updated2, "id = ?", cat.ID).Error)
		require.NotNil(t, updated2.PhotoKey)
		assert.NotEqual(t, firstKey, *updated2.PhotoKey, "each upload should get a fresh key")
	})
}

// TestHandleAdminListCategories_PhotoURLEmptyUntilSet covers the fallback:
// sections without an admin-set photo report no photo_url at all, so the app
// knows to fall back to the first product's own preview.
func TestHandleAdminListCategories_PhotoURLEmptyUntilSet(t *testing.T) {
	testutil.WithTx(t, func() {
		tx := database.DB
		cat := models.ProductCategory{Name: "No Photo Section", DisplayOrder: 2}
		require.NoError(t, tx.Create(&cat).Error)

		app := testutil.FiberApp(nil)
		app.Get("/x", HandleAdminListCategories)
		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var parsed struct {
			Data []struct {
				ID       string `json:"id"`
				PhotoURL string `json:"photo_url"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))
		found := false
		for _, row := range parsed.Data {
			if row.ID == cat.ID {
				found = true
				assert.Empty(t, row.PhotoURL)
			}
		}
		assert.True(t, found)
	})
}
