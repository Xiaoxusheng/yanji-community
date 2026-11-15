package notifications

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 通知 HTTP 层。
type Handler struct {
	svc *Service
	rdb *redis.Client // SSE 推送用（可空）
}

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// SetRedis 注入 Redis 客户端（SSE 用，可空）。
func (h *Handler) SetRedis(r *redis.Client) { h.rdb = r }

// Stream GET /me/notifications/stream —— SSE 实时通知。
func (h *Handler) Stream(c *gin.Context) { StreamHandler(h.rdb)(c) }

// Register 注册路由（全部需登录）。
func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/me/notifications")
	g.Use(httpx.RequireAuth())
	{
		g.GET("", h.List)
		g.GET("/unread-count", h.UnreadCount)
		g.POST("/read-all", h.MarkAllRead)
		g.POST("/:id/read", h.MarkRead)
		g.DELETE("/:id", h.Delete)
	}
	rg.GET("/notifications/preferences", httpx.RequireAuth(), h.Preferences)
	rg.PATCH("/notifications/preferences", httpx.RequireAuth(), h.UpdatePreferences)
}

// List GET /me/notifications
func (h *Handler) List(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	unread := c.Query("unread") == "true"
	items, total, err := h.svc.List(c.Request.Context(), u.ID, pq, unread, c.Query("type"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []Notification{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// UnreadCount GET /me/notifications/unread-count
func (h *Handler) UnreadCount(c *gin.Context) {
	u := httpx.CurrentUser(c)
	n, err := h.svc.UnreadCount(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"unread": n})
}

// MarkRead POST /me/notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, errInvalidID)
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), u.ID, id); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "已标记已读"})
}

// MarkAllRead POST /me/notifications/read-all
func (h *Handler) MarkAllRead(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if err := h.svc.MarkAllRead(c.Request.Context(), u.ID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "全部已读"})
}

// Delete DELETE /me/notifications/:id
func (h *Handler) Delete(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, errInvalidID)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), u.ID, id); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "通知已删除"})
}

// Preferences GET /notifications/preferences
func (h *Handler) Preferences(c *gin.Context) {
	u := httpx.CurrentUser(c)
	p, err := h.svc.Preferences(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"preferences": p})
}

// UpdatePreferences PATCH /notifications/preferences
func (h *Handler) UpdatePreferences(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req struct {
		NotifyEmail *bool `json:"notify_email"`
		NotifyInapp *bool `json:"notify_inapp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, errInvalidBody)
		return
	}
	if err := h.svc.UpdatePreferences(c.Request.Context(), u.ID, req.NotifyEmail, req.NotifyInapp); err != nil {
		respond.Fail(c, err)
		return
	}
	p, err := h.svc.Preferences(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"preferences": p, "message": "通知偏好已保存"})
}
