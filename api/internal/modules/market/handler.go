package market

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/imageutil"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var imageMIMETypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".png": "image/png", ".webp": "image/webp",
}

// designFileExts whitelists embroidery machine / stitch / related export formats.
var designFileExts = map[string]bool{
	// Core stitch formats
	".dst": true, ".pes": true, ".exp": true, ".jef": true,
	".vp3": true, ".xxx": true, ".hus": true, ".pec": true,
	// Tajima / Barudan / ZSK / Pfaff / Wilcom machine variants
	".dsb": true, ".dsz": true, ".tbf": true, ".t01": true,
	".t03": true, ".t04": true, ".t05": true, ".t09": true,
	".t10": true, ".t15": true, ".u01": true,
	// Melco / Janome / Elna / Husqvarna / Pfaff / Singer / Happy / Inbro
	".cnd": true, ".jpx": true, ".sew": true, ".jan": true,
	".emd": true, ".shv": true, ".pcs": true, ".pcd": true,
	".pcq": true, ".pcm": true, ".csd": true, ".tap": true,
	".inb": true, ".ksm": true, ".yng": true,
	// Wilcom / Hiraoka / Laesser / Saurer / Time & Space / native
	".ess": true, ".esl": true, ".dat": true, ".vep": true,
	".mst": true, ".sas": true, ".mjd": true, ".emb": true, ".emc": true,
	// BERNINA / Dahao / template / vector-cutting (listed in digitizer export dialogs)
	".art": true, ".art50": true, ".art60": true, ".art70": true, ".art80": true,
	".dhp": true, ".dha": true, ".dhe": true,
	".tpl": true, ".plt": true, ".dxf": true,
	// Document formats (e.g. digitized-design reference sheets / instructions)
	".pdf": true,
}

const (
	maxPreviewImageBytes = 5 * 1024 * 1024 // 5 MB
	maxDesignFileBytes   = 5 * 1024 * 1024 // 5 MB
	maxPreviewImages     = 7
	minPriceInPaise      = 1000     // ₹10
	maxPriceInPaise      = 10000000 // ₹1,00,000
)

// populateDesignURLs resolves preview keys → public URLs and computes
// per-viewer flags. FileKey is never exposed; the machine file is only
// reachable via the signed download-url endpoint.
func populateDesignURLs(designs []models.Design, viewerID string, purchased map[string]bool) {
	for i := range designs {
		d := &designs[i]
		d.PreviewURLs = make([]string, 0, len(d.PreviewKeys))
		for _, k := range d.PreviewKeys {
			d.PreviewURLs = append(d.PreviewURLs, storage.Store.PublicURL(k))
		}
		d.IsMine = d.SellerID == viewerID
		if purchased != nil {
			d.IsPurchased = purchased[d.ID]
		}
	}
}

// purchasedDesignIDs returns the set of design IDs the user has successfully bought.
func purchasedDesignIDs(userID string, designIDs []string) map[string]bool {
	out := map[string]bool{}
	if len(designIDs) == 0 {
		return out
	}
	var ids []string
	database.DB.Model(&models.DesignPurchase{}).
		Where("buyer_id = ? AND status = ? AND design_id IN ?", userID, models.PaymentSuccess, designIDs).
		Pluck("design_id", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// attachSellerNames fills SellerName for a slice of designs in one query.
// Also stamps FeaturedSeller from active market plan subscriptions (placeholder perk).
func attachSellerNames(designs []models.Design) {
	if len(designs) == 0 {
		return
	}
	sellerIDs := make([]string, 0, len(designs))
	seen := map[string]bool{}
	for _, d := range designs {
		if !seen[d.SellerID] {
			seen[d.SellerID] = true
			sellerIDs = append(sellerIDs, d.SellerID)
		}
	}
	type row struct{ ID, Name string }
	var rows []row
	database.DB.Model(&models.User{}).Where("id IN ?", sellerIDs).Select("id", "name").Scan(&rows)
	names := map[string]string{}
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	featured := activeFeaturedSellerIDs(sellerIDs)
	for i := range designs {
		designs[i].SellerName = names[designs[i].SellerID]
		designs[i].FeaturedSeller = featured[designs[i].SellerID]
	}
}

// uploadPreviewImage validates, compresses, and stores one preview image.
func uploadPreviewImage(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > maxPreviewImageBytes {
		return "", fmt.Errorf("image must be under 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if _, ok := imageMIMETypes[ext]; !ok {
		return "", fmt.Errorf("unsupported image format %s", ext)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	compressed, err := imageutil.CompressFast(src, imageutil.MaxCommunityPixels, imageutil.JPEGQuality)
	if err != nil {
		return "", fmt.Errorf("failed to process image: %w", err)
	}

	fileKey := fmt.Sprintf("market/previews/%s.jpg", uuid.New().String())
	if err := storage.Store.Upload(context.Background(), fileKey, "image/jpeg", bytes.NewReader(compressed), int64(len(compressed))); err != nil {
		slog.Error("market: failed to store preview image", "key", fileKey, "error", err)
		return "", err
	}
	return fileKey, nil
}

// uploadDesignFile validates and stores the embroidery machine file privately.
func uploadDesignFile(fileHeader *multipart.FileHeader) (key, format string, err error) {
	if fileHeader.Size > maxDesignFileBytes {
		return "", "", fmt.Errorf("design file must be under 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !designFileExts[ext] {
		return "", "", fmt.Errorf("unsupported design format %s", ext)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", err
	}
	defer src.Close()

	key = fmt.Sprintf("market/files/%s%s", uuid.New().String(), ext)
	if err := storage.Store.Upload(context.Background(), key, "application/octet-stream", src, fileHeader.Size); err != nil {
		slog.Error("market: failed to store design file", "key", key, "error", err)
		return "", "", err
	}
	return key, strings.TrimPrefix(ext, "."), nil
}

func cleanupKeys(keys []string) {
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := storage.Store.Delete(context.Background(), k); err != nil {
			slog.Error("market: orphan file cleanup failed", "key", k, "error", err)
		}
	}
}

// HandleListDesigns godoc
// @Summary     Browse marketplace designs
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       search  query  string  false  "Search by title"
// @Param       page    query  int     false  "Page number (default: 1)"
// @Success     200  {object}  []models.Design
// @Failure     401  {object}  map[string]string
// @Router      /user/market/designs [get]
func HandleListDesigns(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Design{}).
		Where("is_active = true").
		Order("created_at DESC").
		Limit(limit).Offset(offset)
	if s := c.Query("search"); s != "" {
		query = query.Where("title ILIKE ?", "%"+s+"%")
	}
	if catID := c.Query("category_id"); catID != "" {
		// catID may be a leaf category (exact match is correct) or a
		// top-level section — designs are filed against the specific leaf
		// category chosen at upload time, never against the parent section
		// itself, so a section's preview strip must also pick up any design
		// filed under one of its children.
		var childIDs []string
		database.DB.Model(&models.DesignCategory{}).
			Where("parent_id = ?", catID).
			Pluck("id", &childIDs)
		if len(childIDs) > 0 {
			query = query.Where("category_id IN ?", append(childIDs, catID))
		} else {
			query = query.Where("category_id = ?", catID)
		}
	}

	var designs []models.Design
	if err := query.Find(&designs).Error; err != nil {
		return response.InternalError(c, "failed to fetch designs")
	}
	if designs == nil {
		designs = []models.Design{}
	}

	ids := make([]string, len(designs))
	for i, d := range designs {
		ids[i] = d.ID
	}
	attachSellerNames(designs)
	populateDesignURLs(designs, userID, purchasedDesignIDs(userID, ids))
	return response.OK(c, designs)
}

// HandleGetDesign godoc
// @Summary     Get a marketplace design
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  models.Design
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/designs/{id} [get]
func HandleGetDesign(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Design
	if err := database.DB.Where("id = ? AND is_active = true", c.Params("id")).First(&d).Error; err != nil {
		return response.NotFound(c, "design not found")
	}

	// Don't let sellers inflate their own view count by previewing.
	if d.SellerID != userID {
		_ = database.DB.Model(&models.Design{}).
			Where("id = ?", d.ID).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
		d.ViewCount++
	}

	designs := []models.Design{d}
	attachSellerNames(designs)
	populateDesignURLs(designs, userID, purchasedDesignIDs(userID, []string{d.ID}))
	return response.OK(c, designs[0])
}

// HandleCreateDesign godoc
// @Summary     List a design for sale
// @Tags        User Market
// @Accept      multipart/form-data
// @Produce     json
// @Security    UserAuth
// @Param       title           formData  string  true   "Design title"
// @Param       description     formData  string  false  "Description"
// @Param       price_in_paise  formData  int     true   "Price in paise (min 1000 = ₹10)"
// @Param       file            formData  file    true   "Embroidery machine file (.dst/.pes/...)"
// @Param       previews        formData  file    true   "1-7 preview images"
// @Success     201  {object}  models.Design
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/market/designs [post]
func HandleCreateDesign(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	title := strings.TrimSpace(c.FormValue("title"))
	description := strings.TrimSpace(c.FormValue("description"))
	price, err := strconv.ParseInt(c.FormValue("price_in_paise"), 10, 64)
	if title == "" || err != nil {
		return response.BadRequest(c, "title and price_in_paise are required")
	}
	if price < minPriceInPaise || price > maxPriceInPaise {
		return response.BadRequest(c, "price must be between ₹10 and ₹1,00,000")
	}

	categoryID := strings.TrimSpace(c.FormValue("category_id"))
	categoryOther := strings.TrimSpace(c.FormValue("category_other"))
	if categoryID == "" {
		return response.BadRequest(c, "category_id is required")
	}
	var category models.DesignCategory
	if err := database.DB.First(&category, "id = ?", categoryID).Error; err != nil {
		return response.BadRequest(c, "invalid category")
	}
	if category.IsOther && categoryOther == "" {
		return response.BadRequest(c, "category_other is required when category is Other")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.BadRequest(c, "design file is required")
	}

	form, err := c.MultipartForm()
	if err != nil || form == nil || len(form.File["previews"]) == 0 {
		return response.BadRequest(c, "at least one preview image is required")
	}
	previewHeaders := form.File["previews"]
	if len(previewHeaders) > maxPreviewImages {
		previewHeaders = previewHeaders[:maxPreviewImages]
	}

	fileKey, fileFormat, err := uploadDesignFile(fileHeader)
	if err != nil {
		return response.BadRequest(c, "invalid design file: "+err.Error())
	}

	// Upload previews concurrently, mirroring the community image pipeline.
	previewKeys := make([]string, len(previewHeaders))
	var g errgroup.Group
	for i, fh := range previewHeaders {
		i, fh := i, fh
		g.Go(func() error {
			key, err := uploadPreviewImage(fh)
			if err != nil {
				return err
			}
			previewKeys[i] = key
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		cleanupKeys(append(previewKeys, fileKey))
		return response.BadRequest(c, "invalid preview image: "+err.Error())
	}

	design := models.Design{
		SellerID:      userID,
		Title:         title,
		Description:   description,
		PriceInPaise:  price,
		FileKey:       fileKey,
		FileName:      filepath.Base(fileHeader.Filename),
		FileSizeBytes: fileHeader.Size,
		FileFormat:    fileFormat,
		PreviewKeys:   previewKeys,
		IsActive:      true,
		CategoryID:    &category.ID,
	}
	if category.IsOther {
		design.CategoryOther = &categoryOther
	}
	if err := database.DB.Create(&design).Error; err != nil {
		cleanupKeys(append(previewKeys, fileKey))
		return response.InternalError(c, "failed to create design")
	}

	designs := []models.Design{design}
	attachSellerNames(designs)
	populateDesignURLs(designs, userID, nil)
	return response.Created(c, designs[0])
}

// deleteDesignRow applies the shared takedown rule: designs with sales are
// soft-deactivated (buyers keep download rights); unsold designs are removed
// along with their stored files.
func deleteDesignRow(d *models.Design) error {
	if d.SalesCount > 0 {
		return database.DB.Model(&models.Design{}).
			Where("id = ?", d.ID).
			Update("is_active", false).Error
	}
	if err := database.DB.Delete(&models.Design{}, "id = ?", d.ID).Error; err != nil {
		return err
	}
	cleanupKeys(append(append([]string{}, d.PreviewKeys...), d.FileKey))
	return nil
}

// HandleDeleteDesign godoc
// @Summary     Remove own design listing
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/designs/{id} [delete]
func HandleDeleteDesign(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Design
	if err := database.DB.Where("id = ? AND seller_id = ?", c.Params("id"), userID).First(&d).Error; err != nil {
		return response.NotFound(c, "design not found")
	}
	if err := deleteDesignRow(&d); err != nil {
		return response.InternalError(c, "failed to delete design")
	}
	return response.OK(c, fiber.Map{"message": "design removed"})
}

// HandleMyDesigns godoc
// @Summary     List own design listings (including unlisted)
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.Design
// @Failure     401  {object}  map[string]string
// @Router      /user/market/my/designs [get]
func HandleMyDesigns(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var designs []models.Design
	if err := database.DB.
		Where("seller_id = ?", userID).
		Order("created_at DESC").
		Find(&designs).Error; err != nil {
		return response.InternalError(c, "failed to fetch designs")
	}
	if designs == nil {
		designs = []models.Design{}
	}
	attachSellerNames(designs)
	populateDesignURLs(designs, userID, nil)
	return response.OK(c, designs)
}

// HandleMyDesignStats godoc
// @Summary     Seller analytics for one of their designs
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Design ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/my/designs/{id}/stats [get]
func HandleMyDesignStats(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	designID := c.Params("id")

	var d models.Design
	if err := database.DB.Where("id = ? AND seller_id = ?", designID, userID).First(&d).Error; err != nil {
		return response.NotFound(c, "design not found")
	}

	var revenue int64
	database.DB.Model(&models.DesignPurchase{}).
		Where("design_id = ? AND seller_id = ? AND status = ?", designID, userID, models.PaymentSuccess).
		Select("COALESCE(SUM(seller_net_in_paise), 0)").
		Scan(&revenue)

	return response.OK(c, fiber.Map{
		"view_count":       d.ViewCount,
		"sales_count":      d.SalesCount,
		"revenue_in_paise": revenue,
	})
}

// HandleListCategories godoc
// @Summary     List Design Market categories
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.DesignCategory
// @Failure     401  {object}  map[string]string
// @Router      /user/market/categories [get]
func HandleListCategories(c *fiber.Ctx) error {
	var categories []models.DesignCategory
	if err := database.DB.
		Order("display_order ASC").
		Find(&categories).Error; err != nil {
		return response.InternalError(c, "failed to fetch categories")
	}
	if categories == nil {
		categories = []models.DesignCategory{}
	}
	populateCategoryPhotoURLs(categories)
	return response.OK(c, categories)
}
