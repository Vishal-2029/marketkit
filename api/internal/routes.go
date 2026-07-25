package internal

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/marketkit/api/internal/middleware"
	"github.com/marketkit/api/internal/modules/admins"
	"github.com/marketkit/api/internal/modules/app_version"
	"github.com/marketkit/api/internal/modules/audit_logs"
	"github.com/marketkit/api/internal/modules/auth"
	"github.com/marketkit/api/internal/modules/community"
	"github.com/marketkit/api/internal/modules/dashboard"
	"github.com/marketkit/api/internal/modules/market"
	"github.com/marketkit/api/internal/modules/notifications"
	"github.com/marketkit/api/internal/modules/payments"
	"github.com/marketkit/api/internal/modules/photo_categories"
	"github.com/marketkit/api/internal/modules/photos"
	"github.com/marketkit/api/internal/modules/plans"
	"github.com/marketkit/api/internal/modules/platform_wallet"
	"github.com/marketkit/api/internal/modules/playback"
	"github.com/marketkit/api/internal/modules/playlists"
	"github.com/marketkit/api/internal/modules/refunds"
	"github.com/marketkit/api/internal/modules/revenue"
	"github.com/marketkit/api/internal/modules/sessions"
	"github.com/marketkit/api/internal/modules/user_auth"
	"github.com/marketkit/api/internal/modules/user_notifications"
	"github.com/marketkit/api/internal/modules/user_payments"
	"github.com/marketkit/api/internal/modules/user_playlists"
	"github.com/marketkit/api/internal/modules/user_videos"
	"github.com/marketkit/api/internal/modules/users"
	"github.com/marketkit/api/internal/modules/video_engagement"
	"github.com/marketkit/api/internal/modules/videos"
	"github.com/marketkit/api/internal/modules/wallet"
)

// rate limiters shared across route groups
var otpLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many OTP requests — try again in a minute",
		})
	},
})

var streamLimiter = limiter.New(limiter.Config{
	Max:        10,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "stream_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many stream requests, please slow down",
		})
	},
})

var postLimiter = limiter.New(limiter.Config{
	Max:        3,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "community_post_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many posts — wait 10 minutes",
		})
	},
})

// designUploadLimiter bounds marketplace listings per account — there is no
// admin approval gate, so this is the primary spam control.
var designUploadLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "design_upload_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many design uploads — wait 10 minutes",
		})
	},
})

var replyLimiter = limiter.New(limiter.Config{
	Max:        10,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "community_reply_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many replies — wait 10 minutes",
		})
	},
})

var commentLimiter = limiter.New(limiter.Config{
	Max:        10,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "video_comment_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many comments — wait 10 minutes",
		})
	},
})

var messageLimiter = limiter.New(limiter.Config{
	Max:        10,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "design_message_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many messages — wait 10 minutes",
		})
	},
})

// refreshLimiter guards token-refresh endpoints against brute-forcing a
// stolen/guessed refresh token. Looser than the OTP limiter since legitimate
// clients refresh routinely.
var refreshLimiter = limiter.New(limiter.Config{
	Max:        20,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many requests — try again shortly",
		})
	},
})

// resetPasswordLimiter: 3 attempts per hour per IP on the endpoint that
// actually consumes a reset code and sets a new password.
var resetPasswordLimiter = limiter.New(limiter.Config{
	Max:        3,
	Expiration: 1 * time.Hour,
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many password reset attempts — try again in an hour",
		})
	},
})

// changePasswordLimiter guards the authenticated change-password endpoints
// against using a stolen/leaked session to brute-force the current password.
// Separate instances since admin/user auth middleware store the ID under
// different Locals keys ("adminID" vs "userID").
var adminChangePasswordLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("adminID").(string)
		return "change_password_admin_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many attempts — try again in a minute",
		})
	},
})

var userChangePasswordLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "change_password_user_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many attempts — try again in a minute",
		})
	},
})

// avatarLimiter bounds avatar uploads per account. Without this, an
// authenticated user could fire unbounded concurrent/rapid uploads to fill
// storage — each upload writes a new object before the old one is cleaned up.
var adminAvatarLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("adminID").(string)
		return "avatar_admin_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many avatar uploads — wait 10 minutes",
		})
	},
})

var userAvatarLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 10 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		id, _ := c.Locals("userID").(string)
		return "avatar_user_" + id
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "too many avatar uploads — wait 10 minutes",
		})
	},
})

// RegisterRoutes mounts all API routes onto the given v1 router group.
func RegisterRoutes(v1 fiber.Router) {
	// ── Admin Auth (/auth) ────────────────────────────────────────────────────
	a := v1.Group("/auth")
	a.Post("/register", otpLimiter, auth.HandleRegister)
	a.Post("/send-otp", otpLimiter, auth.HandleSendOTP)
	a.Post("/verify-otp", otpLimiter, auth.HandleVerifyOTP)
	a.Post("/refresh", refreshLimiter, auth.HandleRefresh)
	a.Post("/logout", auth.HandleLogout)
	a.Get("/me", middleware.Authenticate, auth.HandleMe)
	a.Patch("/me", middleware.Authenticate, auth.HandleUpdateProfile)
	a.Post("/avatar", middleware.Authenticate, adminAvatarLimiter, auth.HandleUploadAvatar)
	a.Delete("/avatar", middleware.Authenticate, auth.HandleDeleteAvatar)
	a.Post("/change-password", middleware.Authenticate, adminChangePasswordLimiter, auth.HandleChangePassword)
	a.Get("/google", auth.HandleGoogleLogin)
	a.Get("/google/callback", auth.HandleGoogleCallback)

	// ── User Auth (/user/auth) ────────────────────────────────────────────────
	ua := v1.Group("/user/auth")
	ua.Post("/register", otpLimiter, user_auth.HandleRegister)
	ua.Post("/forgot-password", otpLimiter, user_auth.HandleForgotPassword)
	ua.Post("/reset-password", resetPasswordLimiter, user_auth.HandleResetPassword)
	ua.Post("/send-otp", otpLimiter, user_auth.HandleSendOTP)
	ua.Post("/verify-otp", otpLimiter, user_auth.HandleVerifyOTP)
	ua.Post("/google", otpLimiter, user_auth.HandleGoogleLogin)
	ua.Post("/refresh", refreshLimiter, user_auth.HandleRefresh)
	ua.Post("/logout", user_auth.HandleLogout)
	ua.Get("/me", middleware.UserAuthenticate, user_auth.HandleMe)
	ua.Patch("/me", middleware.UserAuthenticate, user_auth.HandleUpdateProfile)
	ua.Delete("/me", middleware.UserAuthenticate, user_auth.HandleDeleteAccount)
	ua.Post("/avatar", middleware.UserAuthenticate, userAvatarLimiter, user_auth.HandleUploadAvatar)
	ua.Delete("/avatar", middleware.UserAuthenticate, user_auth.HandleDeleteAvatar)
	ua.Post("/change-password", middleware.UserAuthenticate, userChangePasswordLimiter, user_auth.HandleChangePassword)
	ua.Get("/device-token", middleware.UserAuthenticate, user_auth.HandleListDeviceTokens)
	ua.Post("/device-token", middleware.UserAuthenticate, user_auth.HandleRegisterDeviceToken)
	ua.Patch("/me/app-mode", middleware.UserAuthenticate, user_auth.HandleSetAppMode)

	// ── Admin: Users (/users) ─────────────────────────────────────────────────
	u := v1.Group("/users", middleware.Authenticate)
	u.Get("/", users.HandleList)
	u.Post("/send-otp", otpLimiter, users.HandleSendUserOTP)
	u.Post("/verify-otp", otpLimiter, users.HandleVerifyUserOTP)
	u.Post("/", users.HandleCreate)
	u.Get("/:id", users.HandleGet)
	u.Patch("/:id", users.HandleUpdate)
	u.Delete("/:id", users.HandleDelete)
	u.Post("/:id/force-logout", users.HandleForceLogout)
	u.Patch("/:id/suspend", users.HandleSuspend)
	u.Post("/:id/change-plan", users.HandleChangePlan)
	u.Post("/:id/assign-free-plan", users.HandleAssignFreePlan)
	u.Post("/:id/cancel-free-plan", users.HandleCancelFreePlan)

	// ── Admin: Admin accounts (/admins) ─────────────────────────────────────────────────
	ad := v1.Group("/admins", middleware.Authenticate)
	ad.Get("/", admins.HandleList)
	ad.Post("/", admins.HandleCreate)
	ad.Delete("/:id", admins.HandleDelete)
	ad.Patch("/:id/super", admins.HandleSetSuper)

	// ── Admin: Videos (/videos) ───────────────────────────────────────────────
	v := v1.Group("/videos", middleware.Authenticate)
	v.Get("/", videos.HandleList)
	v.Get("/presign-upload", videos.HandlePresignUpload)
	v.Post("/finalize", videos.HandleFinalize)
	v.Post("/", videos.HandleCreate)
	v.Get("/:id", videos.HandleGet)
	v.Patch("/:id", videos.HandleUpdate)
	v.Delete("/:id", videos.HandleDelete)
	v.Post("/:id/retry", videos.HandleRetry)
	v.Post("/:id/photos", videos.HandleUploadPhotos)
	v.Get("/:id/stream", streamLimiter, videos.HandleStream)
	v.Get("/:id/stream-url", streamLimiter, videos.HandleStreamURL)
	// Admin engagement (moderation)
	v.Get("/:id/engagement", video_engagement.HandleAdminGetEngagement)
	v.Get("/:id/comments", video_engagement.HandleAdminListComments)
	v.Post("/:id/comments/reply", video_engagement.HandleAdminReplyComment)
	v.Delete("/:id/comments/:cid", video_engagement.HandleAdminDeleteComment)

	// ── Admin: Plans (/plans) ─────────────────────────────────────────────────
	v1.Get("/plans/public", plans.HandlePublicList)
	pl := v1.Group("/plans", middleware.Authenticate)
	pl.Get("/", plans.HandleList)
	pl.Post("/", plans.HandleCreate)
	pl.Patch("/:id", plans.HandleUpdate)
	pl.Delete("/:id", plans.HandleDelete)

	// ── Admin: Payments (/payments) ───────────────────────────────────────────
	pay := v1.Group("/payments")
	pay.Post("/razorpay-webhook", payments.HandleRazorpayWebhook)
	payAuth := pay.Use(middleware.Authenticate)
	payAuth.Get("/", payments.HandleList)
	payAuth.Get("/:id", payments.HandleGet)
	payAuth.Post("/manual", payments.HandleManual)
	payAuth.Post("/:id/activate", payments.HandleActivate)
	payAuth.Post("/:id/request-refund", refunds.HandleRequest)

	// ── Admin: Refund Requests (/refunds) ─────────────────────────────────────
	rf := v1.Group("/refunds", middleware.Authenticate)
	rf.Get("/", refunds.HandleList)
	rf.Post("/:id/approve", refunds.HandleApprove)
	rf.Post("/:id/reject", refunds.HandleReject)

	// ── Admin: Sessions (/sessions) ───────────────────────────────────────────
	s := v1.Group("/sessions", middleware.Authenticate)
	s.Get("/", sessions.HandleList)
	s.Post("/backfill-locations", sessions.HandleBackfillLocations)
	s.Delete("/", sessions.HandleRevokeAll)
	s.Delete("/:id", sessions.HandleRevoke)

	// ── Admin: Audit Logs (/audit-logs) ───────────────────────────────────────
	al := v1.Group("/audit-logs", middleware.Authenticate)
	al.Get("/", audit_logs.HandleList)

	// ── Admin: Dashboard (/dashboard) ─────────────────────────────────────────
	d := v1.Group("/dashboard", middleware.Authenticate)
	d.Get("/stats", dashboard.HandleStats)
	d.Get("/recent-payments", dashboard.HandleRecentPayments)
	d.Get("/recent-activity", dashboard.HandleRecentActivity)
	d.Get("/user-growth", dashboard.HandleUserGrowth)
	d.Get("/plan-distribution", dashboard.HandlePlanDistribution)

	// ── Admin: Revenue (/revenue) ─────────────────────────────────────────────
	r := v1.Group("/revenue", middleware.Authenticate)
	r.Get("/summary", revenue.HandleSummary)
	r.Get("/monthly", revenue.HandleMonthly)
	r.Get("/by-plan", revenue.HandleByPlan)
	r.Get("/forecast", revenue.HandleForecast)
	r.Get("/renewal-stats", revenue.HandleRenewalStats)

	// ── Admin: Playback Analytics (/playback) ─────────────────────────────────
	pb := v1.Group("/playback", middleware.Authenticate)
	pb.Get("/top-videos", playback.HandleTopVideos)
	pb.Get("/by-user", playback.HandleByUser)
	pb.Get("/summary", playback.HandleSummary)
	pb.Get("/daily-trend", playback.HandleDailyTrend)

	// ── Admin: Photos (/photos) ───────────────────────────────────────────────
	v1.Get("/photos/public", photos.HandlePublic)
	ph := v1.Group("/photos", middleware.Authenticate)
	ph.Get("/", photos.HandleList)
	ph.Post("/", photos.HandleCreate)
	ph.Patch("/:id", photos.HandleUpdate)
	ph.Delete("/:id", photos.HandleDelete)

	pc := v1.Group("/photo-categories", middleware.Authenticate)
	pc.Get("/", photo_categories.HandleList)
	pc.Post("/", photo_categories.HandleCreate)
	pc.Delete("/:id", photo_categories.HandleDelete)

	// ── Admin: Notifications (/admin/notifications) ───────────────────────────
	n := v1.Group("/admin/notifications", middleware.Authenticate)
	n.Get("/", notifications.HandleListNotifications)
	n.Post("/broadcast", notifications.HandleBroadcast)
	n.Delete("/", notifications.HandleClearNotifications)
	n.Delete("/:id", notifications.HandleDeleteNotification)

	// ── Admin: Community (/community) ─────────────────────────────────────────
	ac := v1.Group("/community", middleware.Authenticate)
	ac.Get("/posts", community.HandleAdminListPosts)
	ac.Get("/posts/:id", community.HandleAdminGetPost)
	ac.Post("/posts", community.HandleAdminCreatePost)
	ac.Post("/posts/:id/replies", community.HandleAdminCreateReply)
	ac.Patch("/posts/:id", community.HandleAdminUpdatePost)
	ac.Delete("/posts/:id", community.HandleAdminDeletePost)
	ac.Delete("/replies/:id", community.HandleAdminDeleteReply)
	ac.Patch("/posts/:id/flag", community.HandleAdminFlagPost)

	// ── Admin: Playlists (/admin/playlists) ──────────────────────────────────
	ap := v1.Group("/admin/playlists", middleware.Authenticate)
	ap.Get("/", playlists.HandleList)
	ap.Post("/", playlists.HandleCreate)
	ap.Get("/:id", playlists.HandleGetDetail)
	ap.Patch("/:id", playlists.HandleUpdate)
	ap.Delete("/:id", playlists.HandleDelete)
	ap.Put("/:id/videos", playlists.HandleSetVideos)

	// ── User: Videos (/user/videos) ───────────────────────────────────────────
	uv := v1.Group("/user/videos", middleware.UserAuthenticate)
	// Static sub-paths must be registered before /:id to avoid param capture.
	uv.Get("/intro", user_videos.HandleGetIntro)
	uv.Get("/latest", user_videos.HandleGetLatest)
	uv.Get("/", user_videos.HandleList)
	uv.Get("/:id/stream", user_videos.HandleStream)
	uv.Get("/:id/stream-url", user_videos.HandleStreamURL)
	uv.Get("/:id/download-url", user_videos.HandleDownloadURL)
	uv.Get("/:id/qualities", user_videos.HandleGetQualities)
	uv.Get("/:id/photos", user_videos.HandleGetPhotos)
	uv.Post("/:id/playback-log", user_videos.HandleLogPlayback)
	uv.Post("/:id/progress", user_videos.HandleSaveProgress)
	uv.Delete("/:id/progress", user_videos.HandleDeleteProgress)
	// User engagement
	uv.Get("/:id/reactions", video_engagement.HandleGetReactions)
	uv.Post("/:id/reactions", video_engagement.HandleToggleReaction)
	uv.Get("/:id/comments", video_engagement.HandleListComments)
	uv.Post("/:id/comments", commentLimiter, video_engagement.HandlePostComment)
	uv.Delete("/:id/comments/:cid", video_engagement.HandleDeleteComment)

	// ── User: Payments (/user/payments) ───────────────────────────────────────
	up := v1.Group("/user/payments", middleware.UserAuthenticate)
	up.Post("/order", user_payments.HandleCreateOrder)
	up.Post("/verify", user_payments.HandleVerifyPayment)

	// ── User: Playlists (/user/playlists) ────────────────────────────────────
	uplists := v1.Group("/user/playlists", middleware.UserAuthenticate)
	uplists.Get("/", user_playlists.HandleList)
	uplists.Get("/:id", user_playlists.HandleGetDetail)

	// ── User: Notifications (/user/notifications) ─────────────────────────────
	un := v1.Group("/user/notifications", middleware.UserAuthenticate)
	un.Get("/", user_notifications.HandleList)
	un.Post("/read", user_notifications.HandleMarkAllRead)
	un.Delete("/", user_notifications.HandleClearAll)
	un.Delete("/:id", user_notifications.HandleDeleteOne)

	// ── App Version (public check + admin publish) ────────────────────────────
	v1.Get("/app/version", app_version.HandleGetLatest)
	av := v1.Group("/admin/app-version", middleware.Authenticate)
	av.Post("/", app_version.HandleCreate)
	av.Post("/upload", app_version.HandleUpload)
	// Script-friendly upload: uses X-Publish-Secret header instead of JWT (no expiry).
	// Must NOT be under the /admin/app-version group prefix — that group injects
	// JWT middleware for all sub-paths, which would block PublishSecretAuth.
	v1.Post("/admin/apk/publish", middleware.PublishSecretAuth, app_version.HandleUpload)

	// ── User: Community (/user/community) ─────────────────────────────────────
	uc := v1.Group("/user/community", middleware.UserAuthenticate)
	uc.Get("/posts", community.HandleListPosts)
	uc.Post("/posts", postLimiter, community.HandleCreatePost)
	uc.Get("/posts/:id", community.HandleGetPost)
	uc.Get("/posts/:id/replies", community.HandleListReplies)
	uc.Post("/posts/:id/replies", replyLimiter, community.HandleCreateReply)

	// ── User: Design Market (/user/market) ────────────────────────────────────
	um := v1.Group("/user/market", middleware.UserAuthenticate)
	um.Get("/designs", market.HandleListDesigns)
	um.Post("/designs", designUploadLimiter, market.HandleCreateDesign)
	// Static sub-paths must be registered before /designs/:id to avoid param capture.
	um.Get("/fee", market.HandleGetSellerFee)
	um.Get("/categories", market.HandleListCategories)
	um.Get("/my/designs", market.HandleMyDesigns)
	um.Get("/my/designs/:id/stats", market.HandleMyDesignStats)
	um.Get("/my/purchases", market.HandleMyPurchases)
	um.Get("/my/earnings", market.HandleEarnings)
	um.Post("/purchases/order", market.HandleCreatePurchaseOrder)
	um.Post("/purchases/verify", market.HandleVerifyPurchase)
	um.Post("/purchases/wallet", market.HandlePurchaseWithWallet)
	// Design Market Plans (separate from learning /plans)
	um.Get("/plans", market.HandleListMarketPlans)
	um.Get("/plans/my", market.HandleMyMarketPlan)
	um.Delete("/plans/my", market.HandleCancelMyMarketPlan)
	um.Post("/plans/verify", market.HandleVerifyMarketPlan)
	um.Post("/plans/:id/order", market.HandleCreateMarketPlanOrder)
	um.Post("/plans/:id/wallet", market.HandleSubscribeMarketPlanWithWallet)
	um.Get("/designs/:id", market.HandleGetDesign)
	um.Delete("/designs/:id", market.HandleDeleteDesign)
	um.Get("/designs/:id/download-url", market.HandleDownloadURL)
	um.Get("/purchases/:id/invoice", market.HandleDownloadInvoice)
	um.Get("/designs/:id/messages", market.HandleListDesignMessages)
	um.Post("/designs/:id/messages", messageLimiter, market.HandlePostDesignMessage)

	// ── User: Wallet (/user/wallet) ───────────────────────────────────────────
	uw := v1.Group("/user/wallet", middleware.UserAuthenticate)
	uw.Get("/", wallet.HandleSummary)
	uw.Get("/transactions", wallet.HandleTransactions)
	uw.Post("/topup/order", wallet.HandleTopupOrder)
	uw.Post("/topup/verify", wallet.HandleTopupVerify)
	uw.Get("/payout-details", wallet.HandleGetPayoutDetails)
	uw.Put("/payout-details", wallet.HandleUpsertPayoutDetails)
	uw.Post("/withdrawals", wallet.HandleCreateWithdrawal)
	uw.Get("/withdrawals", wallet.HandleMyWithdrawals)

	// ── Admin: Design Market (/market) ────────────────────────────────────────
	am := v1.Group("/market", middleware.Authenticate)
	am.Get("/designs", market.HandleAdminListDesigns)
	am.Delete("/designs/:id", market.HandleAdminDeleteDesign)
	am.Get("/categories", market.HandleAdminListCategories)
	am.Post("/categories", market.HandleAdminCreateCategory)
	am.Put("/categories/:id", market.HandleAdminUpdateCategory)
	am.Post("/categories/:id/photo", market.HandleAdminUploadCategoryPhoto)
	am.Delete("/categories/:id", market.HandleAdminDeleteCategory)
	am.Get("/purchases", market.HandleAdminListPurchases)
	am.Get("/purchases/:id/messages", market.HandleAdminListPurchaseMessages)
	am.Post("/purchases/:id/messages", market.HandleAdminReplyPurchaseMessage)
	am.Get("/users", market.HandleAdminListMarketUsers)
	am.Get("/users/:id/designs", market.HandleAdminMarketUserDesigns)
	am.Get("/plans", market.HandleAdminListMarketPlans)
	am.Post("/plans", market.HandleAdminCreateMarketPlan)
	am.Put("/plans/:id", market.HandleAdminUpdateMarketPlan)
	am.Delete("/plans/:id", market.HandleAdminDeleteMarketPlan)
	am.Get("/plan-payments", market.HandleAdminListMarketPlanPayments)
	am.Get("/revenue/summary", market.HandleMarketRevenueSummary)
	am.Get("/revenue/monthly", market.HandleMarketRevenueMonthly)

	// ── Admin: Wallet (/wallet) ───────────────────────────────────────────────
	aw := v1.Group("/wallet", middleware.Authenticate)
	aw.Get("/withdrawals", wallet.HandleAdminListWithdrawals)
	aw.Post("/withdrawals/:id/settle", wallet.HandleAdminSettle)
	aw.Get("/users/:id/transactions", wallet.HandleAdminUserTransactions)
	aw.Get("/settings", wallet.HandleGetSettings)
	aw.Put("/settings", wallet.HandleUpdateSettings)

	// ── Admin: Platform Wallet (/platform-wallet) — super-admin only ─────────
	pw := v1.Group("/platform-wallet", middleware.Authenticate)
	pw.Get("/", platform_wallet.HandleGet)
	pw.Get("/transactions", platform_wallet.HandleTransactions)
	pw.Post("/withdrawals", platform_wallet.HandleCreateWithdrawal)
}
