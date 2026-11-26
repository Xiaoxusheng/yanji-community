package users

import (
	"github.com/gin-gonic/gin"

	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 用户 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic 公开路由（/users/*）；`/users/:username/posts` 由路由装配层注入 posts 模块实现。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/users/:username", h.PublicProfile)
	rg.GET("/users/:username/followers", h.Followers)
	rg.GET("/users/:username/following", h.Following)
}

// RegisterMe 认证路由（/me/*，强制 RequireAuth 防止游客空指针访问）。
func (h *Handler) RegisterMe(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth())
	{
		g.GET("/me/profile", h.MyProfile)
		g.PATCH("/me/profile", h.UpdateProfile)
		g.PATCH("/me/settings", h.UpdateSettings)
		g.GET("/me/following", h.MyFollowing)
		g.GET("/me/followers", h.MyFollowers)
	}
}

// PublicProfile GET /users/:username
func (h *Handler) PublicProfile(c *gin.Context) {
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	p, err := h.svc.PublicProfile(c.Request.Context(), c.Param("username"), viewerID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"profile": p})
}

// MyProfile GET /me/profile
func (h *Handler) MyProfile(c *gin.Context) {
	u := httpx.CurrentUser(c)
	p, err := h.svc.MyProfile(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"profile": p})
}

// UpdateProfile PATCH /me/profile
func (h *Handler) UpdateProfile(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, errInvalidBody)
		return
	}
	if err := h.svc.UpdateProfile(c.Request.Context(), u.ID, req); err != nil {
		respond.Fail(c, err)
		return
	}
	p, err := h.svc.MyProfile(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"profile": p, "message": "资料已更新"})
}

// UpdateSettings PATCH /me/settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, errInvalidBody)
		return
	}
	if err := h.svc.UpdateSettings(c.Request.Context(), u.ID, req); err != nil {
		respond.Fail(c, err)
		return
	}
	p, err := h.svc.MyProfile(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"profile": p, "message": "设置已保存"})
}

// Followers GET /users/:username/followers
func (h *Handler) Followers(c *gin.Context) { h.listFollow(c, true) }

// Following GET /users/:username/following
func (h *Handler) Following(c *gin.Context) { h.listFollow(c, false) }

func (h *Handler) listFollow(c *gin.Context, followerList bool) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	profile, err := h.svc.PublicProfile(c.Request.Context(), c.Param("username"), viewerID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	isSelf := httpx.CurrentUser(c) != nil && httpx.CurrentUser(c).Username == c.Param("username")
	if !isSelf && (profile.Restricted || !profile.ShowFollowers) {
		respond.Fail(c, errFollowersPrivate)
		return
	}
	uid, err := h.svc.InternalIDByUsername(c.Request.Context(), c.Param("username"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var items []FollowerRow
	var total int64
	if followerList {
		items, total, err = h.svc.Followers(c.Request.Context(), uid, pq.PageSize, pq.Offset)
	} else {
		items, total, err = h.svc.Following(c.Request.Context(), uid, pq.PageSize, pq.Offset)
	}
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// MyFollowing GET /me/following?type=user|category|tag
func (h *Handler) MyFollowing(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	switch c.DefaultQuery("type", "user") {
	case "category":
		items, total, err := h.svc.FollowingCategories(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
		if err != nil {
			respond.Fail(c, err)
			return
		}
		if items == nil {
			items = []FollowedCategoryRow{}
		}
		respond.Page(c, items, pq.Page, pq.PageSize, total)
	case "tag":
		items, total, err := h.svc.FollowingTags(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
		if err != nil {
			respond.Fail(c, err)
			return
		}
		if items == nil {
			items = []FollowedTagRow{}
		}
		respond.Page(c, items, pq.Page, pq.PageSize, total)
	default:
		items, total, err := h.svc.Following(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
		if err != nil {
			respond.Fail(c, err)
			return
		}
		if items == nil {
			items = []FollowerRow{}
		}
		respond.Page(c, items, pq.Page, pq.PageSize, total)
	}
}

// MyFollowers GET /me/followers
func (h *Handler) MyFollowers(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.Followers(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}
