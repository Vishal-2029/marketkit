package community

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/internal/storage"
	"github.com/marketkit/api/pkg/response"
	"gorm.io/gorm"
)

// HandleAdminListPosts godoc
// @Summary     List all community posts (admin)
// @Tags        Admin Community
// @Produce     json
// @Security    AdminAuth
// @Param       page  query  int  false  "Page number (default: 1)"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /community/posts [get]
func HandleAdminListPosts(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	var total int64
	if err := database.DB.Model(&models.CommunityPost{}).Count(&total).Error; err != nil {
		return response.InternalError(c, "failed to fetch posts")
	}

	var posts []models.CommunityPost
	if err := database.DB.Order("created_at DESC").Limit(limit).Offset(offset).Find(&posts).Error; err != nil {
		return response.InternalError(c, "failed to fetch posts")
	}
	if posts == nil {
		posts = []models.CommunityPost{}
	}
	populatePostURLs(posts)

	// Bulk-fetch real user identities for admin display.
	userIDs := make([]string, 0, len(posts))
	for _, p := range posts {
		if p.UserID != "" && p.UserID != "admin" {
			userIDs = append(userIDs, p.UserID)
		}
	}
	userMap := make(map[string]models.User)
	if len(userIDs) > 0 {
		var users []models.User
		database.DB.Select("id, name, email").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	for i := range posts {
		if posts[i].UserID == "admin" {
			posts[i].UserName = "Admin"
		} else if u, ok := userMap[posts[i].UserID]; ok {
			posts[i].UserName = u.Name
			posts[i].UserEmail = u.Email
		}
	}

	return response.OK(c, fiber.Map{"posts": posts, "total": total})
}

// HandleAdminGetPost godoc
// @Summary     Get a community post with all its replies (admin)
// @Tags        Admin Community
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Post ID"
// @Success     200  {object}  models.CommunityPost
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/posts/{id} [get]
func HandleAdminGetPost(c *fiber.Ctx) error {
	id := c.Params("id")

	var post models.CommunityPost
	err := database.DB.
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Where("id = ?", id).
		First(&post).Error
	if err != nil {
		return response.NotFound(c, "post not found")
	}
	if post.Replies == nil {
		post.Replies = []models.CommunityReply{}
	}
	populatePostURL(&post)

	// Resolve the post author's real identity for admin display.
	if post.UserID == "admin" {
		post.UserName = "Admin"
	} else if post.UserID != "" {
		var u models.User
		if database.DB.Select("id, name, email").Where("id = ?", post.UserID).First(&u).Error == nil {
			post.UserName = u.Name
			post.UserEmail = u.Email
		}
	}

	return response.OK(c, post)
}

// HandleAdminDeletePost godoc
// @Summary     Delete a community post (admin)
// @Tags        Admin Community
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Post ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/posts/{id} [delete]
func HandleAdminDeletePost(c *fiber.Ctx) error {
	id := c.Params("id")
	var post models.CommunityPost
	if err := database.DB.Where("id = ?", id).First(&post).Error; err != nil {
		return response.NotFound(c, "post not found")
	}
	for _, k := range post.ImageKeys {
		if k != "" {
			storage.Store.Delete(context.Background(), k)
		}
	}
	database.DB.Where("post_id = ?", id).Delete(&models.CommunityReply{})
	database.DB.Delete(&post)
	return response.OK(c, fiber.Map{"message": "post deleted"})
}

// HandleAdminDeleteReply godoc
// @Summary     Delete a community reply (admin)
// @Tags        Admin Community
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Reply ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/replies/{id} [delete]
func HandleAdminDeleteReply(c *fiber.Ctx) error {
	id := c.Params("id")
	var reply models.CommunityReply
	if err := database.DB.Where("id = ?", id).First(&reply).Error; err != nil {
		return response.NotFound(c, "reply not found")
	}
	if reply.ImageKey != nil && *reply.ImageKey != "" {
		storage.Store.Delete(context.Background(), *reply.ImageKey)
	}
	database.DB.Delete(&reply)
	database.DB.Model(&models.CommunityPost{}).
		Where("id = ?", reply.PostID).
		UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)"))
	return response.OK(c, fiber.Map{"message": "reply deleted"})
}

// HandleAdminFlagPost godoc
// @Summary     Toggle flag on a community post (admin)
// @Tags        Admin Community
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Post ID"
// @Success     200  {object}  map[string]bool
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/posts/{id}/flag [patch]
func HandleAdminFlagPost(c *fiber.Ctx) error {
	id := c.Params("id")
	var post models.CommunityPost
	if err := database.DB.Where("id = ?", id).First(&post).Error; err != nil {
		return response.NotFound(c, "post not found")
	}
	post.IsFlagged = !post.IsFlagged
	if err := database.DB.Save(&post).Error; err != nil {
		return response.InternalError(c, "failed to update post")
	}
	return response.OK(c, fiber.Map{"is_flagged": post.IsFlagged})
}

// HandleAdminUpdatePost godoc
// @Summary     Update a community post (admin)
// @Tags        Admin Community
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id    path  string             true  "Post ID"
// @Param       body  body  map[string]string  true  "title and content"
// @Success     200  {object}  models.CommunityPost
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/posts/{id} [patch]
func HandleAdminUpdatePost(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Title == "" || body.Content == "" {
		return response.BadRequest(c, "title and content are required")
	}

	var post models.CommunityPost
	if err := database.DB.Where("id = ?", id).First(&post).Error; err != nil {
		return response.NotFound(c, "post not found")
	}
	post.Title = body.Title
	post.Content = body.Content
	if err := database.DB.Save(&post).Error; err != nil {
		return response.InternalError(c, "failed to update post")
	}

	go fcm.SendToAll("Community Update", body.Title, "community")
	return response.OK(c, post)
}

// HandleAdminCreateReply godoc
// @Summary     Post an admin reply to a community post
// @Tags        Admin Community
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id    path  string             true  "Post ID"
// @Param       body  body  map[string]string  true  "content"
// @Success     201  {object}  models.CommunityReply
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /community/posts/{id}/replies [post]
func HandleAdminCreateReply(c *fiber.Ctx) error {
	postID := c.Params("id")
	var post models.CommunityPost
	if err := database.DB.Where("id = ?", postID).First(&post).Error; err != nil {
		return response.NotFound(c, "post not found")
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return response.BadRequest(c, "content is required")
	}

	reply := models.CommunityReply{
		PostID:   postID,
		UserID:   "admin",
		AnonName: "Admin",
		Content:  body.Content,
	}
	if err := database.DB.Create(&reply).Error; err != nil {
		return response.InternalError(c, "failed to create reply")
	}
	database.DB.Model(&models.CommunityPost{}).
		Where("id = ?", postID).
		UpdateColumn("reply_count", gorm.Expr("reply_count + 1"))

	return response.Created(c, reply)
}

// HandleAdminCreatePost godoc
// @Summary     Create a community post as admin
// @Tags        Admin Community
// @Accept      multipart/form-data
// @Produce     json
// @Security    AdminAuth
// @Param       title     formData  string  true   "Post title"
// @Param       content   formData  string  true   "Post content"
// @Param       category  formData  string  false  "Category (default: GENERAL)"
// @Param       images    formData  file    false  "Up to 3 images"
// @Success     201  {object}  models.CommunityPost
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /community/posts [post]
func HandleAdminCreatePost(c *fiber.Ctx) error {
	category := c.FormValue("category")
	title := c.FormValue("title")
	content := c.FormValue("content")
	if title == "" || content == "" {
		return response.BadRequest(c, "title and content are required")
	}
	if category == "" {
		category = "GENERAL"
	}

	var imageKeys []string
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		for _, fh := range form.File["images"] {
			if len(imageKeys) >= 3 {
				break
			}
			key, err := uploadCommunityImage(fh)
			if err != nil {
				for _, k := range imageKeys {
					storage.Store.Delete(context.Background(), k)
				}
				return response.BadRequest(c, "invalid image: "+err.Error())
			}
			imageKeys = append(imageKeys, key)
		}
	}

	post := models.CommunityPost{
		UserID:    "admin",
		AnonName:  "Admin",
		Category:  category,
		Title:     title,
		Content:   content,
		ImageKeys: imageKeys,
	}
	if err := database.DB.Create(&post).Error; err != nil {
		for _, k := range imageKeys {
			if delErr := storage.Store.Delete(context.Background(), k); delErr != nil {
				slog.Error("community: orphan admin image cleanup failed", "key", k, "error", delErr)
			}
		}
		return response.InternalError(c, "failed to create post")
	}
	populatePostURL(&post)
	go fcm.SendToAll("New Community Post", title, "community")
	return response.Created(c, post)
}
