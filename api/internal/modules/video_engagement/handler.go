package video_engagement

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/marketkit/api/internal/database"
	"github.com/marketkit/api/internal/fcm"
	"github.com/marketkit/api/internal/models"
	"github.com/marketkit/api/pkg/response"
)

// HandleGetReactions godoc
// @Summary     Get like/dislike counts and current user's reaction
// @Tags        Video Engagement
// @Produce     json
// @Security    UserAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/reactions [get]
func HandleGetReactions(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")

	var likeCount, dislikeCount int64
	database.DB.Model(&models.VideoReaction{}).Where("video_id = ? AND reaction = ?", videoID, "like").Count(&likeCount)
	database.DB.Model(&models.VideoReaction{}).Where("video_id = ? AND reaction = ?", videoID, "dislike").Count(&dislikeCount)

	userReaction := ""
	var existing models.VideoReaction
	if err := database.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&existing).Error; err == nil {
		userReaction = existing.Reaction
	}

	return response.OK(c, fiber.Map{
		"like_count":    likeCount,
		"dislike_count": dislikeCount,
		"user_reaction": userReaction,
	})
}

// HandleToggleReaction godoc
// @Summary     Toggle like or dislike on a video
// @Tags        Video Engagement
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       id    path  string              true  "Video ID"
// @Param       body  body  map[string]string   true  "reaction: 'like' or 'dislike'"
// @Success     200  {object}  map[string]interface{}
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/reactions [post]
func HandleToggleReaction(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")

	var body struct {
		Reaction string `json:"reaction"`
	}
	if err := c.BodyParser(&body); err != nil || (body.Reaction != "like" && body.Reaction != "dislike") {
		return response.BadRequest(c, "reaction must be 'like' or 'dislike'")
	}

	var existing models.VideoReaction
	err := database.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&existing).Error

	if err == nil {
		if existing.Reaction == body.Reaction {
			// Same reaction — remove it (toggle off)
			database.DB.Delete(&existing)
		} else {
			// Different reaction — switch it
			database.DB.Model(&existing).Update("reaction", body.Reaction)
		}
	} else {
		// No existing reaction — create
		database.DB.Create(&models.VideoReaction{
			UserID:   userID,
			VideoID:  videoID,
			Reaction: body.Reaction,
		})
	}

	// Return updated counts
	return HandleGetReactions(c)
}

// HandleListComments godoc
// @Summary     List the requesting user's private comment thread for a video
// @Tags        Video Engagement
// @Produce     json
// @Security    UserAuth
// @Param       id    path   string  true   "Video ID"
// @Param       page  query  int     false  "Page number (default: 1)"
// @Success     200  {object}  []models.VideoComment
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/comments [get]
func HandleListComments(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	// Scoped to this user's own private thread — their own messages plus any
	// admin replies addressed to them — never another user's comments.
	var comments []models.VideoComment
	database.DB.
		Where("video_id = ? AND thread_user_id = ?", videoID, userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&comments)

	if comments == nil {
		comments = []models.VideoComment{}
	}
	return response.OK(c, comments)
}

// HandlePostComment godoc
// @Summary     Post a comment on a video
// @Tags        Video Engagement
// @Accept      json
// @Produce     json
// @Security    UserAuth
// @Param       id    path  string             true  "Video ID"
// @Param       body  body  map[string]string  true  "content"
// @Success     201  {object}  models.VideoComment
// @Failure     400  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Router      /user/videos/{id}/comments [post]
func HandlePostComment(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")

	var body struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.Content == "" {
		return response.BadRequest(c, "content is required")
	}

	// Look up the user's display name
	var user models.User
	userName := "User"
	if err := database.DB.Select("name").First(&user, "id = ?", userID).Error; err == nil && user.Name != "" {
		userName = user.Name
	}

	comment := models.VideoComment{
		VideoID:      videoID,
		UserID:       userID,
		ThreadUserID: userID,
		IsAdmin:      false,
		UserName:     userName,
		Content:      body.Content,
	}
	if err := database.DB.Create(&comment).Error; err != nil {
		return response.InternalError(c, "failed to post comment")
	}
	return response.Created(c, comment)
}

// HandleDeleteComment godoc
// @Summary     Delete own comment on a video
// @Tags        Video Engagement
// @Produce     json
// @Security    UserAuth
// @Param       id   path  string  true  "Video ID"
// @Param       cid  path  string  true  "Comment ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     403  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /user/videos/{id}/comments/{cid} [delete]
func HandleDeleteComment(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	videoID := c.Params("id")
	commentID := c.Params("cid")

	var comment models.VideoComment
	if err := database.DB.Where("id = ? AND video_id = ?", commentID, videoID).First(&comment).Error; err != nil {
		return response.NotFound(c, "comment not found")
	}
	if comment.UserID != userID {
		return response.Forbidden(c, "you can only delete your own comments")
	}

	database.DB.Delete(&comment)
	return response.OK(c, fiber.Map{"message": "comment deleted"})
}

// ---------------------------------------------------------------------------
// Admin handlers
// ---------------------------------------------------------------------------

// HandleAdminGetEngagement godoc
// @Summary     Get engagement stats for a video (admin)
// @Tags        Video Engagement
// @Produce     json
// @Security    AdminAuth
// @Param       id  path  string  true  "Video ID"
// @Success     200  {object}  map[string]interface{}
// @Failure     401  {object}  map[string]string
// @Router      /videos/{id}/engagement [get]
func HandleAdminGetEngagement(c *fiber.Ctx) error {
	videoID := c.Params("id")

	var likeCount, dislikeCount, commentCount int64
	database.DB.Model(&models.VideoReaction{}).Where("video_id = ? AND reaction = ?", videoID, "like").Count(&likeCount)
	database.DB.Model(&models.VideoReaction{}).Where("video_id = ? AND reaction = ?", videoID, "dislike").Count(&dislikeCount)
	database.DB.Model(&models.VideoComment{}).Where("video_id = ?", videoID).Count(&commentCount)

	return response.OK(c, fiber.Map{
		"like_count":    likeCount,
		"dislike_count": dislikeCount,
		"comment_count": commentCount,
	})
}

// HandleAdminListComments godoc
// @Summary     List all comments for a video (admin)
// @Tags        Video Engagement
// @Produce     json
// @Security    AdminAuth
// @Param       id    path   string  true   "Video ID"
// @Param       page  query  int     false  "Page number (default: 1)"
// @Success     200  {object}  []models.VideoComment
// @Failure     401  {object}  map[string]string
// @Router      /videos/{id}/comments [get]
func HandleAdminListComments(c *fiber.Ctx) error {
	videoID := c.Params("id")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	var comments []models.VideoComment
	database.DB.
		Where("video_id = ?", videoID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&comments)

	if comments == nil {
		comments = []models.VideoComment{}
	}
	return response.OK(c, comments)
}

// HandleAdminReplyComment godoc
// @Summary     Reply to a user's private comment thread on a video (admin)
// @Tags        Video Engagement
// @Accept      json
// @Produce     json
// @Security    AdminAuth
// @Param       id    path  string             true  "Video ID"
// @Param       body  body  map[string]string  true  "user_id, content"
// @Success     201  {object}  models.VideoComment
// @Failure     400  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /videos/{id}/comments/reply [post]
func HandleAdminReplyComment(c *fiber.Ctx) error {
	videoID := c.Params("id")

	var body struct {
		UserID  string `json:"user_id"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil || body.UserID == "" || body.Content == "" {
		return response.BadRequest(c, "user_id and content are required")
	}

	var targetUser models.User
	if err := database.DB.Select("id").First(&targetUser, "id = ?", body.UserID).Error; err != nil {
		return response.NotFound(c, "user not found")
	}

	adminID, _ := c.Locals("adminID").(string)
	adminName := "Admin"
	var admin models.Admin
	if adminID != "" {
		if err := database.DB.Select("first_name, last_name").First(&admin, "id = ?", adminID).Error; err == nil {
			if full := admin.FullName(); full != "" {
				adminName = full
			}
		}
	}

	comment := models.VideoComment{
		VideoID:      videoID,
		UserID:       "admin",
		ThreadUserID: body.UserID,
		IsAdmin:      true,
		UserName:     adminName,
		Content:      body.Content,
	}
	if err := database.DB.Create(&comment).Error; err != nil {
		return response.InternalError(c, "failed to post reply")
	}

	preview := body.Content
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	go fcm.SendToUser(body.UserID, "Admin replied", preview)

	return response.Created(c, comment)
}

// HandleAdminDeleteComment godoc
// @Summary     Delete any comment (admin moderation)
// @Tags        Video Engagement
// @Produce     json
// @Security    AdminAuth
// @Param       id   path  string  true  "Video ID"
// @Param       cid  path  string  true  "Comment ID"
// @Success     200  {object}  map[string]string
// @Failure     401  {object}  map[string]string
// @Failure     404  {object}  map[string]string
// @Router      /videos/{id}/comments/{cid} [delete]
func HandleAdminDeleteComment(c *fiber.Ctx) error {
	videoID := c.Params("id")
	commentID := c.Params("cid")

	result := database.DB.Where("id = ? AND video_id = ?", commentID, videoID).Delete(&models.VideoComment{})
	if result.RowsAffected == 0 {
		return response.NotFound(c, "comment not found")
	}
	return response.OK(c, fiber.Map{"message": "comment deleted"})
}
