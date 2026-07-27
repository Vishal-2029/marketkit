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

// productFileExts whitelists the file types sellers may upload.
//
// This is a digital-goods marketplace, so the default set covers the common
// deliverables: documents, images, vector/design source files, fonts, audio,
// video, and archives. Add or remove extensions to match what you sell — this
// map is the only place upload types are enforced.
var productFileExts = map[string]bool{
	// Documents
	".pdf": true, ".epub": true, ".txt": true, ".md": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".csv": true,
	// Images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true, ".tif": true, ".tiff": true,
	// Design / vector source
	".ai": true, ".eps": true, ".psd": true, ".sketch": true,
	".fig": true, ".xd": true, ".indd": true, ".dxf": true, ".plt": true,
	// 3D / CAD
	".obj": true, ".fbx": true, ".stl": true, ".blend": true, ".step": true,
	// Fonts
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
	// Audio / video
	".mp3": true, ".wav": true, ".aac": true, ".flac": true,
	".mp4": true, ".mov": true, ".webm": true,
	// Archives — the catch-all for multi-file bundles
	".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
}

const (
	maxPreviewImageBytes = 5 * 1024 * 1024 // 5 MB
	maxProductFileBytes  = 5 * 1024 * 1024 // 5 MB
	maxPreviewImages     = 7
	minPriceInPaise      = 1000     // ₹10
	maxPriceInPaise      = 10000000 // ₹1,00,000
)

// populateProductURLs resolves preview keys → public URLs and computes
// per-viewer flags. FileKey is never exposed; the machine file is only
// reachable via the signed download-url endpoint.
func populateProductURLs(products []models.Product, viewerID string, purchased map[string]bool) {
	for i := range products {
		d := &products[i]
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

// purchasedProductIDs returns the set of product IDs the user has successfully bought.
func purchasedProductIDs(userID string, productIDs []string) map[string]bool {
	out := map[string]bool{}
	if len(productIDs) == 0 {
		return out
	}
	var ids []string
	database.DB.Model(&models.ProductPurchase{}).
		Where("buyer_id = ? AND status = ? AND product_id IN ?", userID, models.PaymentSuccess, productIDs).
		Pluck("product_id", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// attachSellerNames fills SellerName for a slice of products in one query.
// Also stamps FeaturedSeller from active market plan subscriptions (placeholder perk).
func attachSellerNames(products []models.Product) {
	if len(products) == 0 {
		return
	}
	sellerIDs := make([]string, 0, len(products))
	seen := map[string]bool{}
	for _, d := range products {
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
	for i := range products {
		products[i].SellerName = names[products[i].SellerID]
		products[i].FeaturedSeller = featured[products[i].SellerID]
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

// uploadProductFile validates and stores the seller's product file privately.
func uploadProductFile(fileHeader *multipart.FileHeader) (key, format string, err error) {
	if fileHeader.Size > maxProductFileBytes {
		return "", "", fmt.Errorf("product file must be under 5 MB")
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !productFileExts[ext] {
		return "", "", fmt.Errorf("unsupported product format %s", ext)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", err
	}
	defer src.Close()

	key = fmt.Sprintf("market/files/%s%s", uuid.New().String(), ext)
	if err := storage.Store.Upload(context.Background(), key, "application/octet-stream", src, fileHeader.Size); err != nil {
		slog.Error("market: failed to store product file", "key", key, "error", err)
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

// HandleListProducts godoc
// @Summary     Browse marketplace products
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       search  query  string  false  "Search by title"
// @Param       page    query  int     false  "Page number (default: 1)"
// @Success     200  {object}  []models.Product
// @Failure     401  {object}  map[string]string
// @Router      /user/market/products [get]
func HandleListProducts(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Product{}).
		Where("is_active = true").
		Order("created_at DESC").
		Limit(limit).Offset(offset)
	if s := c.Query("search"); s != "" {
		query = query.Where("title ILIKE ?", "%"+s+"%")
	}
	if catID := c.Query("category_id"); catID != "" {
		// catID may be a leaf category (exact match is correct) or a
		// top-level section — products are filed against the specific leaf
		// category chosen at upload time, never against the parent section
		// itself, so a section's preview strip must also pick up any product
		// filed under one of its children.
		var childIDs []string
		database.DB.Model(&models.ProductCategory{}).
			Where("parent_id = ?", catID).
			Pluck("id", &childIDs)
		if len(childIDs) > 0 {
			query = query.Where("category_id IN ?", append(childIDs, catID))
		} else {
			query = query.Where("category_id = ?", catID)
		}
	}

	var products []models.Product
	if err := query.Find(&products).Error; err != nil {
		return response.InternalError(c, "failed to fetch products")
	}
	if products == nil {
		products = []models.Product{}
	}

	ids := make([]string, len(products))
	for i, d := range products {
		ids[i] = d.ID
	}
	attachSellerNames(products)
	populateProductURLs(products, userID, purchasedProductIDs(userID, ids))
	return response.OK(c, products)
}

// HandleGetProduct godoc
// @Summary     Get a marketplace product
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Product ID"
// @Success     200  {object}  models.Product
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/products/{id} [get]
func HandleGetProduct(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Product
	if err := database.DB.Where("id = ? AND is_active = true", c.Params("id")).First(&d).Error; err != nil {
		return response.NotFound(c, "product not found")
	}

	// Don't let sellers inflate their own view count by previewing.
	if d.SellerID != userID {
		_ = database.DB.Model(&models.Product{}).
			Where("id = ?", d.ID).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
		d.ViewCount++
	}

	products := []models.Product{d}
	attachSellerNames(products)
	populateProductURLs(products, userID, purchasedProductIDs(userID, []string{d.ID}))
	return response.OK(c, products[0])
}

// HandleCreateProduct godoc
// @Summary     List a product for sale
// @Tags        User Market
// @Accept      multipart/form-data
// @Produce     json
// @Security    UserAuth
// @Param       title           formData  string  true   "Product title"
// @Param       description     formData  string  false  "Description"
// @Param       price_in_paise  formData  int     true   "Price in paise (min 1000 = ₹10)"
// @Param       file            formData  file    true   "Product file (.pdf/.zip/.png/...)"
// @Param       previews        formData  file    true   "1-7 preview images"
// @Success     201  {object}  models.Product
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/market/products [post]
func HandleCreateProduct(c *fiber.Ctx) error {
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
	var category models.ProductCategory
	if err := database.DB.First(&category, "id = ?", categoryID).Error; err != nil {
		return response.BadRequest(c, "invalid category")
	}
	if category.IsOther && categoryOther == "" {
		return response.BadRequest(c, "category_other is required when category is Other")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.BadRequest(c, "product file is required")
	}

	form, err := c.MultipartForm()
	if err != nil || form == nil || len(form.File["previews"]) == 0 {
		return response.BadRequest(c, "at least one preview image is required")
	}
	previewHeaders := form.File["previews"]
	if len(previewHeaders) > maxPreviewImages {
		previewHeaders = previewHeaders[:maxPreviewImages]
	}

	fileKey, fileFormat, err := uploadProductFile(fileHeader)
	if err != nil {
		return response.BadRequest(c, "invalid product file: "+err.Error())
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

	product := models.Product{
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
		product.CategoryOther = &categoryOther
	}
	if err := database.DB.Create(&product).Error; err != nil {
		cleanupKeys(append(previewKeys, fileKey))
		return response.InternalError(c, "failed to create product")
	}

	products := []models.Product{product}
	attachSellerNames(products)
	populateProductURLs(products, userID, nil)
	return response.Created(c, products[0])
}

// deleteProductRow applies the shared takedown rule: products with sales are
// soft-deactivated (buyers keep download rights); unsold products are removed
// along with their stored files.
func deleteProductRow(d *models.Product) error {
	if d.SalesCount > 0 {
		return database.DB.Model(&models.Product{}).
			Where("id = ?", d.ID).
			Update("is_active", false).Error
	}
	if err := database.DB.Delete(&models.Product{}, "id = ?", d.ID).Error; err != nil {
		return err
	}
	cleanupKeys(append(append([]string{}, d.PreviewKeys...), d.FileKey))
	return nil
}

// HandleDeleteProduct godoc
// @Summary     Remove own product listing
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Product ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/products/{id} [delete]
func HandleDeleteProduct(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var d models.Product
	if err := database.DB.Where("id = ? AND seller_id = ?", c.Params("id"), userID).First(&d).Error; err != nil {
		return response.NotFound(c, "product not found")
	}
	if err := deleteProductRow(&d); err != nil {
		return response.InternalError(c, "failed to delete product")
	}
	return response.OK(c, fiber.Map{"message": "product removed"})
}

// HandleMyProducts godoc
// @Summary     List own product listings (including unlisted)
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.Product
// @Failure     401  {object}  map[string]string
// @Router      /user/market/my/products [get]
func HandleMyProducts(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var products []models.Product
	if err := database.DB.
		Where("seller_id = ?", userID).
		Order("created_at DESC").
		Find(&products).Error; err != nil {
		return response.InternalError(c, "failed to fetch products")
	}
	if products == nil {
		products = []models.Product{}
	}
	attachSellerNames(products)
	populateProductURLs(products, userID, nil)
	return response.OK(c, products)
}

// HandleMyProductStats godoc
// @Summary     Seller analytics for one of their products
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Product ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/market/my/products/{id}/stats [get]
func HandleMyProductStats(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	productID := c.Params("id")

	var d models.Product
	if err := database.DB.Where("id = ? AND seller_id = ?", productID, userID).First(&d).Error; err != nil {
		return response.NotFound(c, "product not found")
	}

	var revenue int64
	database.DB.Model(&models.ProductPurchase{}).
		Where("product_id = ? AND seller_id = ? AND status = ?", productID, userID, models.PaymentSuccess).
		Select("COALESCE(SUM(seller_net_in_paise), 0)").
		Scan(&revenue)

	return response.OK(c, fiber.Map{
		"view_count":       d.ViewCount,
		"sales_count":      d.SalesCount,
		"revenue_in_paise": revenue,
	})
}

// HandleListCategories godoc
// @Summary     List Product Market categories
// @Tags        User Market
// @Produce     json
// @Security    UserAuth
// @Success     200  {object}  []models.ProductCategory
// @Failure     401  {object}  map[string]string
// @Router      /user/market/categories [get]
func HandleListCategories(c *fiber.Ctx) error {
	var categories []models.ProductCategory
	if err := database.DB.
		Order("display_order ASC").
		Find(&categories).Error; err != nil {
		return response.InternalError(c, "failed to fetch categories")
	}
	if categories == nil {
		categories = []models.ProductCategory{}
	}
	populateCategoryPhotoURLs(categories)
	return response.OK(c, categories)
}
