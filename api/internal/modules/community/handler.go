package community

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
	"github.com/marketkit/api/internal/config"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/email"
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

// populatePostURLs resolves storage keys → public URLs for posts and their replies.
func populatePostURLs(posts []models.CommunityPost) {
	for i := range posts {
		posts[i].ImageURLs = make([]string, 0, len(posts[i].ImageKeys))
		for _, k := range posts[i].ImageKeys {
			posts[i].ImageURLs = append(posts[i].ImageURLs, storage.Store.PublicURL(k))
		}
		populateReplyURLs(posts[i].Replies)
	}
}

func populatePostURL(post *models.CommunityPost) {
	post.ImageURLs = make([]string, 0, len(post.ImageKeys))
	for _, k := range post.ImageKeys {
		post.ImageURLs = append(post.ImageURLs, storage.Store.PublicURL(k))
	}
	populateReplyURLs(post.Replies)
}

func populateReplyURLs(replies []models.CommunityReply) {
	for i := range replies {
		if replies[i].ImageKey != nil && *replies[i].ImageKey != "" {
			u := storage.Store.PublicURL(*replies[i].ImageKey)
			replies[i].ImageURL = &u
		}
	}
}

const maxCommunityImageBytes = 5 * 1024 * 1024 // 5 MB

// uploadCommunityImage validates, compresses, and stores a single image, returning its storage key.
func uploadCommunityImage(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > maxCommunityImageBytes {
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

	fileKey := fmt.Sprintf("community/%s.jpg", uuid.New().String())
	if err := storage.Store.Upload(context.Background(), fileKey, "image/jpeg", bytes.NewReader(compressed), int64(len(compressed))); err != nil {
		slog.Error("[upload] failed to store community image", "key", fileKey, "error", err)
		return "", err
	}
	return fileKey, nil
}

// HandleListPosts godoc
// @Summary     List community posts
// @Tags        User Community
// @Produce     json
// @Security    UserAuth
// @Param       category  query  string  false  "Category filter"
// @Param       page      query  int     false  "Page number (default: 1)"
// @Success     200  {object}  []models.CommunityPost
// @Failure     401  {object}  map[string]string
// @Router      /user/community/posts [get]
func HandleListPosts(c *fiber.Ctx) error {
	category := c.Query("category", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	query := database.DB.Model(&models.CommunityPost{}).
		Where("is_flagged = false").
		Order("created_at DESC").
		Limit(limit).Offset(offset)
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var posts []models.CommunityPost
	if err := query.Find(&posts).Error; err != nil {
		return response.InternalError(c, "failed to fetch posts")
	}
	if posts == nil {
		posts = []models.CommunityPost{}
	}
	populatePostURLs(posts)
	return response.OK(c, posts)
}

// HandleCreatePost godoc
// @Summary     Create a community post
// @Tags        User Community
// @Accept      multipart/form-data
// @Produce     json
// @Security    UserAuth
// @Param       title     formData  string  true   "Post title"
// @Param       content   formData  string  true   "Post content"
// @Param       category  formData  string  false  "Category (default: GENERAL)"
// @Param       images    formData  file    false  "Up to 3 images"
// @Success     201  {object}  models.CommunityPost
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/community/posts [post]
func HandleCreatePost(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	category := c.FormValue("category")
	title := c.FormValue("title")
	content := c.FormValue("content")
	if title == "" || content == "" {
		return response.BadRequest(c, "title and content are required")
	}
	if category == "" {
		category = "GENERAL"
	}

	// Upload up to 3 images, processing them concurrently to keep latency low.
	var imageKeys []string
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		fhs := form.File["images"]
		if len(fhs) > 3 {
			fhs = fhs[:3]
		}
		if len(fhs) > 0 {
			keys := make([]string, len(fhs))
			var g errgroup.Group
			for i, fh := range fhs {
				i, fh := i, fh
				g.Go(func() error {
					key, err := uploadCommunityImage(fh)
					if err != nil {
						return err
					}
					keys[i] = key
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				// Clean up any images that did upload before the failure.
				for _, k := range keys {
					if k == "" {
						continue
					}
					if delErr := storage.Store.Delete(context.Background(), k); delErr != nil {
						slog.Error("community: orphan image cleanup failed", "key", k, "error", delErr)
					}
				}
				return response.BadRequest(c, "invalid image: "+err.Error())
			}
			for _, k := range keys {
				if k != "" {
					imageKeys = append(imageKeys, k)
				}
			}
		}
	}

	post := models.CommunityPost{
		UserID:    userID,
		AnonName:  anonName(userID),
		Category:  category,
		Title:     title,
		Content:   content,
		ImageKeys: imageKeys,
	}
	if err := database.DB.Create(&post).Error; err != nil {
		for _, k := range imageKeys {
			if delErr := storage.Store.Delete(context.Background(), k); delErr != nil {
				slog.Error("community: orphan image cleanup failed", "key", k, "error", delErr)
			}
		}
		return response.InternalError(c, "failed to create post")
	}
	populatePostURL(&post)

	// Alert the admin by email that a learner published a new post.
	if adminEmail := config.App.AdminEmail; adminEmail != "" {
		go email.SendAdminCommunityPostAlert(adminEmail, email.AdminPostAlertData{
			Author:   post.AnonName,
			Category: post.Category,
			Title:    post.Title,
			Content:  snippet(post.Content, 200),
			PostedAt: email.FormatDate(post.CreatedAt),
		})
	}

	return response.Created(c, post)
}

// snippet trims s to at most max runes, appending an ellipsis when truncated.
func snippet(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// HandleGetPost godoc
// @Summary     Get a community post with replies
// @Tags        User Community
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Post ID"
// @Success     200  {object}  models.CommunityPost
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/community/posts/{id} [get]
func HandleGetPost(c *fiber.Ctx) error {
	id := c.Params("id")
	var post models.CommunityPost
	err := database.DB.
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_flagged = false").Order("created_at ASC")
		}).
		Where("id = ? AND is_flagged = false", id).
		First(&post).Error
	if err != nil {
		return response.NotFound(c, "post not found")
	}
	if post.Replies == nil {
		post.Replies = []models.CommunityReply{}
	}
	populatePostURL(&post)
	return response.OK(c, post)
}

// HandleListReplies godoc
// @Summary     List replies for a community post
// @Tags        User Community
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Post ID"
// @Success     200  {object}  []models.CommunityReply
// @Failure     401  {object}  map[string]string
// @Router      /user/community/posts/{id}/replies [get]
func HandleListReplies(c *fiber.Ctx) error {
	postID := c.Params("id")
	var replies []models.CommunityReply
	if err := database.DB.
		Where("post_id = ? AND is_flagged = false", postID).
		Order("created_at ASC").
		Find(&replies).Error; err != nil {
		return response.InternalError(c, "failed to fetch replies")
	}
	if replies == nil {
		replies = []models.CommunityReply{}
	}
	populateReplyURLs(replies)
	return response.OK(c, replies)
}

// HandleCreateReply godoc
// @Summary     Reply to a community post
// @Tags        User Community
// @Accept      multipart/form-data
// @Produce     json
// @Security    UserAuth
// @Param       id       path      string  true   "Post ID"
// @Param       content  formData  string  true   "Reply content"
// @Param       image    formData  file    false  "Optional image"
// @Success     201  {object}  models.CommunityReply
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/community/posts/{id}/replies [post]
func HandleCreateReply(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	postID := c.Params("id")

	var postExists models.CommunityPost
	if err := database.DB.Where("id = ? AND is_flagged = false", postID).First(&postExists).Error; err != nil {
		return response.NotFound(c, "post not found")
	}

	content := c.FormValue("content")
	if content == "" {
		return response.BadRequest(c, "content is required")
	}

	// Optional single image.
	var imageKey *string
	if fh, err := c.FormFile("image"); err == nil && fh != nil {
		key, err := uploadCommunityImage(fh)
		if err != nil {
			return response.BadRequest(c, "invalid image: "+err.Error())
		}
		imageKey = &key
	}

	reply := models.CommunityReply{
		PostID:   postID,
		UserID:   userID,
		AnonName: anonName(userID),
		Content:  content,
		ImageKey: imageKey,
	}
	if err := database.DB.Create(&reply).Error; err != nil {
		if imageKey != nil {
			if delErr := storage.Store.Delete(context.Background(), *imageKey); delErr != nil {
				slog.Error("community: orphan reply image cleanup failed", "key", *imageKey, "error", delErr)
			}
		}
		return response.InternalError(c, "failed to create reply")
	}
	database.DB.Model(&models.CommunityPost{}).
		Where("id = ?", postID).
		UpdateColumn("reply_count", gorm.Expr("reply_count + 1"))

	populateReplyURLs([]models.CommunityReply{reply})
	return response.Created(c, reply)
}
