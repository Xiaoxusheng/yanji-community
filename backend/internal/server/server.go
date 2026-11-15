// Package server 路由装配：把全部模块、中间件与限流串成 gin.Engine。
package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/modules/activities"
	"github.com/yanji-community/backend/internal/modules/admin"
	"github.com/yanji-community/backend/internal/modules/appeals"
	"github.com/yanji-community/backend/internal/modules/auth"
	"github.com/yanji-community/backend/internal/modules/categories"
	"github.com/yanji-community/backend/internal/modules/comments"
	"github.com/yanji-community/backend/internal/modules/follows"
	"github.com/yanji-community/backend/internal/modules/interactions"
	"github.com/yanji-community/backend/internal/modules/messaging"
	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/modules/points"
	"github.com/yanji-community/backend/internal/modules/polls"
	"github.com/yanji-community/backend/internal/modules/posts"
	"github.com/yanji-community/backend/internal/modules/reports"
	"github.com/yanji-community/backend/internal/modules/search"
	"github.com/yanji-community/backend/internal/modules/tags"
	"github.com/yanji-community/backend/internal/modules/uploads"
	"github.com/yanji-community/backend/internal/modules/users"
	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/storage"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/rbac"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Deps 服务器装配依赖。
type Deps struct {
	Cfg    *config.Config
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Logger *slog.Logger
	RBAC   *rbac.Service
	Audit  *moderation.AuditLogger
	Gate   *moderation.Gate
	Store  storage.Storage
}

// Build 装配路由。
func Build(d Deps) *gin.Engine {
	if d.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.MaxMultipartMemory = 8 << 20
	r.Use(httpx.RequestID(d.Cfg.RequestIDHeader))
	r.Use(httpx.Recovery(d.Logger))
	r.Use(httpx.CORS(d.Cfg.CORSAllowOrigins))
	r.Use(httpx.SecureHeaders())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) { respond.OK(c, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
		if d.Redis != nil {
			if err := d.Redis.Ping(ctx).Err(); err != nil {
				respond.Fail(c, apperr.Internal(err))
				return
			}
		}
		respond.OK(c, gin.H{"status": "ready"})
	})

	// 本地存储静态服务
	if ls, ok := d.Store.(*storage.LocalStorage); ok {
		r.GET("/files/*key", uploads.ServeLocal(ls))
	}

	// ---- 模块装配 ----
	ptsSvc := points.New(d.Pool, d.Logger)
	pollSvc := polls.New(d.Pool, d.Logger)
	actSvc := activities.New(d.Pool, ptsSvc, d.Audit, d.Logger)
	appealSvc := appeals.New(d.Pool, moderation.NewSanctionService(d.Pool, d.Logger), d.Audit, d.Logger)
	sanctionSvc := moderation.NewSanctionService(d.Pool, d.Logger)

	authSvc := auth.New(d.Pool, d.Logger, d.Cfg.SessionTTL, d.Cfg.PublicURL)
	authSvc.SetPoints(ptsSvc)
	authHandler := auth.NewHandler(authSvc, d.Cfg.CookieDomain, d.Cfg.CookieSecure)

	userSvc := users.New(d.Pool, d.Logger)
	userHandler := users.NewHandler(userSvc)

	followSvc := follows.New(d.Pool, d.Logger)
	followHandler := follows.NewHandler(followSvc)

	catSvc := categories.New(d.Pool, d.Logger)
	catHandler := categories.NewHandler(catSvc)

	tagSvc := tags.New(d.Pool, d.Logger)
	tagHandler := tags.NewHandler(tagSvc)

	postSvc := posts.New(posts.Deps{
		Pool: d.Pool, Redis: d.Redis, RBAC: d.RBAC, Cats: catSvc, Tags: tagSvc,
		Gate: d.Gate, Audit: d.Audit, Logger: d.Logger, MaxTags: 5,
		Follows: followSvc, Users: userSvc, Points: ptsSvc, Polls: pollSvc,
	})
	postHandler := posts.NewHandler(postSvc)

	// 最佳回答：发布奖励后授予「首答」勋章
	answerSvc := comments.NewAnswerService(comments.Deps2{
		Pool: d.Pool, Points: ptsSvc, Logger: d.Logger,
	})
	answerHandler := comments.NewAnswerHandler(answerSvc)

	commentSvc := comments.New(comments.Deps{
		Pool: d.Pool, RBAC: d.RBAC, Gate: d.Gate, Audit: d.Audit, Logger: d.Logger, Post: postSvc,
	})
	commentHandler := comments.NewHandler(commentSvc)

	interHandler := interactions.NewHandler(d.Pool, ptsSvc)

	notifSvc := notifications.New(d.Pool, d.Logger)
	notifHandler := notifications.NewHandler(notifSvc)
	notifHandler.SetRedis(d.Redis)

	reportSvc := reports.New(d.Pool, d.RBAC, d.Audit, d.Logger)
	reportHandler := reports.NewHandler(reportSvc)

	searchSvc := search.New(d.Pool, d.Logger)
	searchHandler := search.NewHandler(searchSvc)

	ptsHandler := points.NewHandler(ptsSvc)
	dmSvc := messaging.New(d.Pool, d.Logger)
	dmSvc.PushEvent = func(ctx context.Context, recipientID int64, ev notifications.SSEEvent) {
		notifications.PublishNotification(ctx, d.Redis, recipientID, ev)
	}
	dmHandler := messaging.NewHandler(dmSvc)
	pollHandler := polls.NewHandler(pollSvc)
	actHandler := activities.NewHandler(actSvc)
	appealHandler := appeals.NewHandler(appealSvc)
	_ = sanctionSvc

	adminSvc := admin.New(d.Pool, d.RBAC, moderation.NewSanctionService(d.Pool, d.Logger), d.Gate, d.Audit, d.Logger)
	adminHandler := admin.NewHandler(adminSvc)

	uploadSvc := uploads.New(d.Store, d.Pool, d.Logger, d.Cfg.UploadMaxBytes)
	uploadHandler := uploads.NewHandler(uploadSvc)

	// 跨模块装配
	categories.UserResolver = userSvc.InternalIDByUsername

	// 会话校验适配器：auth.SessionUser → httpx.AuthUser
	sessionValidator := sessionValidatorFunc{svc: authSvc}

	api := r.Group("/api/v1")
	api.Use(httpx.SessionAuth(sessionValidator, auth.CookieName))

	// ---- 认证（限流：auth）----
	authGroup := api.Group("/auth")
	authGroup.Use(httpx.RateLimit(d.Redis, "auth", d.Cfg.RateLimitAuth))
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.POST("/logout-all", authHandler.LogoutAll)
		authGroup.GET("/me", authHandler.Me)
		authGroup.POST("/email/verify", authHandler.VerifyEmail)
		authGroup.POST("/email/resend", httpx.RequireAuth(), authHandler.ResendVerification)
		authGroup.POST("/password/forgot", authHandler.ForgotPassword)
		authGroup.POST("/password/reset", authHandler.ResetPassword)
		authGroup.GET("/sessions", httpx.RequireAuth(), authHandler.Sessions)
		authGroup.DELETE("/sessions/:id", httpx.RequireAuth(), authHandler.RevokeSession)
	}

	// ---- 公开读（限流：read）----
	pub := api.Group("")
	pub.Use(httpx.RateLimit(d.Redis, "read", d.Cfg.RateLimitRead))
	{
		postHandler.RegisterPublic(pub)
		commentHandler.RegisterPublic(pub)
		userHandler.RegisterPublic(pub)
		catHandler.RegisterPublic(pub)
		tagHandler.RegisterPublic(pub)
		pub.GET("/categories/:slug/posts", postHandler.CategoryPosts)
		pub.GET("/tags/:slug/posts", postHandler.TagPosts)
		searchHandler.Register(pub)
		pub.GET("/announcements", adminHandler.PublicAnnouncements)
		pub.GET("/activities", actHandler.List)
		pub.GET("/activities/:id", actHandler.Get)
		pub.GET("/badges", ptsHandler.Badges)
		pub.GET("/users/:username/badges", ptsHandler.UserBadges)
		pub.GET("/settings/public", adminHandler.PublicSettings)
		pub.POST("/posts/:id/view", postHandler.RecordView) // 浏览计数（游客可上报）
	}

	// ---- 用户中心（需登录）----
	userHandler.RegisterMe(api)
	notifHandler.Register(api)
	ptsHandler.RegisterUser(api)

	// SSE 实时通知
	api.GET("/me/notifications/stream", notifHandler.Stream)

	// 私信（专属限流桶：30 条/分钟）
	dmHandler.Register(api, httpx.RateLimit(d.Redis, "dm", 30))

	// ---- 写入互动（需登录 + 验证邮箱 + 限流 write）----
	write := api.Group("")
	write.Use(httpx.RateLimit(d.Redis, "write", d.Cfg.RateLimitWrite))
	{
		followHandler.Register(write)
		interHandler.Register(write)
		reportHandler.RegisterUser(write)
		actHandler.RegisterPublic(write)
		pollHandler.Register(write)
		answerHandler.Register(write)
		appealHandler.RegisterUser(write)

		postsWrite := write.Group("")
		postsWrite.Use(httpx.RequireVerified(), httpx.RequireCanPost())
		postHandler.RegisterAuth(postsWrite)

		commentsWrite := write.Group("")
		commentsWrite.Use(httpx.RequireCanComment())
		commentHandler.RegisterAuth(commentsWrite)

		uploadHandler.Register(write)
	}

	// ---- 管理后台（RBAC）----
	adminGroup := api.Group("/admin")
	adminGroup.Use(httpx.RequireAuth())
	perm := func(code string) gin.HandlerFunc { return httpx.RequirePermission(d.RBAC, code) }
	{
		adminGroup.GET("/dashboard", perm("dashboard.read"), adminHandler.Dashboard)

		adminGroup.GET("/users", perm("user.read"), adminHandler.Users)
		adminGroup.GET("/users/:id", perm("user.read"), adminHandler.UserDetail)
		adminGroup.GET("/users/:id/roles", perm("user.read"), adminHandler.UserRoles)
		adminGroup.POST("/users/:id/roles", perm("role.assign"), adminHandler.AssignRole)
		adminGroup.DELETE("/users/:id/roles", perm("role.assign"), adminHandler.RemoveRole)
		adminGroup.POST("/users/:id/sanctions", perm("user.sanction"), adminHandler.CreateSanction)
		adminGroup.DELETE("/users/:id/sanctions/:sanctionId", perm("user.sanction"), adminHandler.LiftSanction)
		adminGroup.POST("/users/:id/unban", perm("user.sanction"), adminHandler.Unban)
		adminGroup.GET("/sanctions", perm("user.read"), adminHandler.Sanctions)

		adminGroup.GET("/posts", perm("post.moderate"), postHandler.AdminList)
		adminGroup.POST("/posts/:id/moderate", perm("post.moderate"), postHandler.Moderate)
		adminGroup.GET("/comments", perm("comment.moderate"), adminHandler.Comments)
		adminGroup.POST("/comments/:id/moderate", perm("comment.moderate"), adminHandler.ModerateComment)

		catAdmin := adminGroup.Group("/categories", perm("category.manage"))
		catHandler.RegisterAdmin(catAdmin)
		tagAdmin := adminGroup.Group("/tags", perm("tag.manage"))
		tagHandler.RegisterAdmin(tagAdmin)

		reportAdmin := adminGroup.Group("/reports", perm("report.handle"))
		reportHandler.RegisterAdmin(reportAdmin)

		swordAdmin := adminGroup.Group("/sensitive-words", perm("moderation.manage"))
		swordAdmin.GET("", adminHandler.ListSensitiveWords)
		swordAdmin.POST("", adminHandler.CreateSensitiveWord)
		swordAdmin.PATCH("/:id", adminHandler.UpdateSensitiveWord)
		swordAdmin.DELETE("/:id", adminHandler.DeleteSensitiveWord)

		annAdmin := adminGroup.Group("/announcements", perm("announcement.manage"))
		annAdmin.GET("", adminHandler.ListAnnouncements)
		annAdmin.POST("", adminHandler.CreateAnnouncement)
		annAdmin.PATCH("/:id", adminHandler.UpdateAnnouncement)
		annAdmin.DELETE("/:id", adminHandler.DeleteAnnouncement)

		// 阶段 2：积分/勋章/活动/申诉
		ptsAdmin := adminGroup.Group("")
		ptsAdmin.Use(perm("user.sanction"))
		ptsHandler.RegisterAdmin(ptsAdmin)
		actAdmin := adminGroup.Group("/activities", perm("announcement.manage"))
		actHandler.RegisterAdmin(actAdmin)
		appealAdmin := adminGroup.Group("/appeals", perm("report.handle"))
		appealHandler.RegisterAdmin(appealAdmin)

		// 批量审核（上限 50，逐条校验范围并写审计）
		adminGroup.POST("/posts/batch-moderate", perm("post.moderate"), postHandler.BatchModerate)

		adminGroup.GET("/roles", perm("role.read"), adminHandler.Roles)
		adminGroup.GET("/permissions", perm("role.read"), adminHandler.Permissions)
		adminGroup.GET("/settings", perm("settings.manage"), adminHandler.GetSettings)
		adminGroup.PATCH("/settings", perm("settings.manage"), adminHandler.UpdateSettings)
		adminGroup.GET("/operation-logs", perm("audit.read"), adminHandler.OperationLogs)
		adminGroup.GET("/login-logs", perm("audit.read"), adminHandler.LoginLogs)
	}

	return r
}

// sessionValidatorFunc 将 auth.Service 的会话校验适配为 httpx.SessionValidator。
type sessionValidatorFunc struct{ svc *auth.Service }

func (a sessionValidatorFunc) ValidateSession(ctx context.Context, token string) (*httpx.AuthUser, error) {
	su, err := a.svc.ValidateSession(ctx, token)
	if err != nil || su == nil {
		return nil, err
	}
	return &httpx.AuthUser{
		ID: su.ID, PublicID: su.PublicID, Username: su.Username, Email: su.Email,
		EmailVerified: su.EmailVerified, Status: su.Status, SessionID: su.SessionID,
		SanctionType: su.SanctionType, SanctionEnds: su.SanctionEnds,
	}, nil
}
