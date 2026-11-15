package follows

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Service 关注业务逻辑。
type Service struct {
	repo   *Repo
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), logger: logger}
}

// FollowUser 关注用户（首次关注发通知）。
func (s *Service) FollowUser(ctx context.Context, followerID int64, targetPublicID string) (bool, error) {
	targetID, err := s.repo.ResolveUserByPublicID(ctx, targetPublicID)
	if err != nil {
		return false, err
	}
	inserted, err := s.repo.FollowUser(ctx, followerID, targetID)
	if err != nil {
		return false, err
	}
	return inserted, nil
}

// UnfollowUser 取消关注。
func (s *Service) UnfollowUser(ctx context.Context, followerID int64, targetPublicID string) error {
	targetID, err := s.repo.ResolveUserByPublicID(ctx, targetPublicID)
	if err != nil {
		return err
	}
	return s.repo.UnfollowUser(ctx, followerID, targetID)
}

// BlockUser 拉黑。
func (s *Service) BlockUser(ctx context.Context, blockerID int64, targetPublicID string) error {
	targetID, err := s.repo.ResolveUserByPublicID(ctx, targetPublicID)
	if err != nil {
		return err
	}
	return s.repo.BlockUser(ctx, blockerID, targetID)
}

// UnblockUser 取消拉黑。
func (s *Service) UnblockUser(ctx context.Context, blockerID int64, targetPublicID string) error {
	targetID, err := s.repo.ResolveUserByPublicID(ctx, targetPublicID)
	if err != nil {
		return err
	}
	return s.repo.UnblockUser(ctx, blockerID, targetID)
}

// ---- FeedProvider 接口实现（posts 模块消费）----

// FollowedUserIDs 关注的作者集合。
func (s *Service) FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.FollowedUserIDs(ctx, userID)
}

// FollowedCategoryIDs 关注的版块集合。
func (s *Service) FollowedCategoryIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.FollowedCategoryIDs(ctx, userID)
}

// FollowedTagIDs 关注的标签集合。
func (s *Service) FollowedTagIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.FollowedTagIDs(ctx, userID)
}

// BlockedIDs 拉黑集合。
func (s *Service) BlockedIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.BlockedIDs(ctx, userID)
}

// FollowCategory / UnfollowCategory / FollowTag / UnfollowTag。
func (s *Service) FollowCategory(ctx context.Context, userID int64, slug string) error {
	id, err := s.repo.ResolveCategoryBySlug(ctx, slug)
	if err != nil {
		return err
	}
	return s.repo.FollowCategory(ctx, userID, id)
}

func (s *Service) UnfollowCategory(ctx context.Context, userID int64, slug string) error {
	id, err := s.repo.ResolveCategoryBySlug(ctx, slug)
	if err != nil {
		return err
	}
	return s.repo.UnfollowCategory(ctx, userID, id)
}

func (s *Service) FollowTag(ctx context.Context, userID int64, slug string) error {
	id, err := s.repo.ResolveTagBySlug(ctx, slug)
	if err != nil {
		return err
	}
	return s.repo.FollowTag(ctx, userID, id)
}

func (s *Service) UnfollowTag(ctx context.Context, userID int64, slug string) error {
	id, err := s.repo.ResolveTagBySlug(ctx, slug)
	if err != nil {
		return err
	}
	return s.repo.UnfollowTag(ctx, userID, id)
}

// ---- HTTP ----

// Handler 关注/拉黑 HTTP 层。`notify` 由 router 注入 notifications 发送函数，避免模块硬依赖。
type Handler struct {
	svc *Service
}

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// OnUserFollowed 首次关注通知钩子（router 注入）。
var OnUserFollowed func(ctx context.Context, followerID, targetID int64)

// Register 注册互动路由（全部需登录 + 已验证）。
func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth(), httpx.RequireVerified())
	{
		g.POST("/users/:username/follow", h.followUser)
		g.DELETE("/users/:username/follow", h.unfollowUser)
		g.POST("/users/:username/block", h.blockUser)
		g.DELETE("/users/:username/block", h.unblockUser)
		g.POST("/categories/:slug/follow", h.followCategory)
		g.DELETE("/categories/:slug/follow", h.unfollowCategory)
		g.POST("/tags/:slug/follow", h.followTag)
		g.DELETE("/tags/:slug/follow", h.unfollowTag)
	}
}

func (h *Handler) followUser(c *gin.Context) {
	u := httpx.CurrentUser(c)
	inserted, err := h.svc.FollowUser(c.Request.Context(), u.ID, c.Param("username"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if inserted && OnUserFollowed != nil {
		targetID, rerr := h.svc.repo.ResolveUserByPublicID(c.Request.Context(), c.Param("username"))
		if rerr == nil {
			OnUserFollowed(c.Request.Context(), u.ID, targetID)
		}
	}
	respond.OK(c, gin.H{"following": true, "already": !inserted})
}

func (h *Handler) unfollowUser(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.UnfollowUser(c.Request.Context(), u.ID, c.Param("username")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"following": false})
}

func (h *Handler) blockUser(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.BlockUser(c.Request.Context(), u.ID, c.Param("username")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"blocked": true, "message": "已拉黑该用户"})
}

func (h *Handler) unblockUser(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.UnblockUser(c.Request.Context(), u.ID, c.Param("username")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"blocked": false})
}

func (h *Handler) followCategory(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.FollowCategory(c.Request.Context(), u.ID, c.Param("slug")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"following": true})
}

func (h *Handler) unfollowCategory(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.UnfollowCategory(c.Request.Context(), u.ID, c.Param("slug")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"following": false})
}

func (h *Handler) followTag(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.FollowTag(c.Request.Context(), u.ID, c.Param("slug")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"following": true})
}

func (h *Handler) unfollowTag(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.UnfollowTag(c.Request.Context(), u.ID, c.Param("slug")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"following": false})
}

var _ = apperr.ErrNotFound
var _ = notifications.EmitTx
