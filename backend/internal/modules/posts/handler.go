package posts

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 帖子 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// TagPosts GET /tags/:slug/posts
func (h *Handler) TagPosts(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	tag, err := h.svc.deps.Tags.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	items, total, err := h.svc.ListByTag(c.Request.Context(), tag.ID, viewerID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// CategoryPosts GET /categories/:slug/posts
func (h *Handler) CategoryPosts(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	cat, err := h.svc.deps.Cats.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	if cat.Visibility != "public" {
		canView := false
		if viewerID > 0 && h.svc.deps.RBAC != nil {
			canView, _ = h.svc.deps.RBAC.CanModerateCategory(c.Request.Context(), viewerID, cat.ID)
		}
		if !canView {
			respond.Fail(c, apperr.NotFound("版块不存在"))
			return
		}
	}
	items, total, err := h.svc.ListByCategory(c.Request.Context(), cat, viewerID, c.DefaultQuery("sort", "latest"), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// RegisterPublic 公开路由。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/feeds", h.Feeds)
	rg.GET("/posts/:id", h.Detail)
	rg.GET("/posts/:id/revisions", httpx.RequireAuth(), h.Revisions)
	rg.POST("/posts/:id/view", h.RecordView)
	// 用户公开帖子（/users/:username/posts）
	rg.GET("/users/:username/posts", h.UserPosts)
}

// RegisterAuth 发帖/编辑/发布写入路由（受邮箱验证与发帖处罚约束）。
func (h *Handler) RegisterAuth(rg *gin.RouterGroup) {
	rg.POST("/posts", h.Create)
	rg.PATCH("/posts/:id", h.Update)
	rg.POST("/posts/:id/publish", h.Publish)
}

// RegisterOwn 查看自己的帖子与删帖路由（仅需登录，不受禁发帖处罚阻断）。
func (h *Handler) RegisterOwn(rg *gin.RouterGroup) {
	rg.GET("/me/posts", httpx.RequireAuth(), h.MyPosts)
	rg.DELETE("/posts/:id", httpx.RequireAuth(), h.Delete)
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/posts", h.AdminList)
	rg.POST("/posts/:id/moderate", h.Moderate)
}

// Feeds GET /feeds?tab=
func (h *Handler) Feeds(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	u := httpx.CurrentUser(c)
	tab := c.DefaultQuery("tab", "latest")
	switch tab {
	case "latest", "hot", "featured", "recommended", "following":
	default:
		respond.Fail(c, invalid("tab 取值不合法"))
		return
	}
	p := FeedParams{Tab: tab, ViewerID: 0, Limit: pq.PageSize, Offset: pq.Offset}
	if u != nil {
		p.ViewerID = u.ID
		if tab == "following" {
			fu, err := h.svc.deps.Follows.FollowedUserIDs(c.Request.Context(), u.ID)
			if err != nil {
				respond.Fail(c, err)
				return
			}
			fc, err := h.svc.deps.Follows.FollowedCategoryIDs(c.Request.Context(), u.ID)
			if err != nil {
				respond.Fail(c, err)
				return
			}
			ft, err := h.svc.deps.Follows.FollowedTagIDs(c.Request.Context(), u.ID)
			if err != nil {
				respond.Fail(c, err)
				return
			}
			blocked, err := h.svc.deps.Follows.BlockedIDs(c.Request.Context(), u.ID)
			if err != nil {
				respond.Fail(c, err)
				return
			}
			p.FollowUsers, p.FollowCats, p.FollowTags, p.Blocked = fu, fc, ft, blocked
		}
	}
	items, total, err := h.svc.Feed(c.Request.Context(), p)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Detail GET /posts/:id
func (h *Handler) Detail(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := &Viewer{}
	if u != nil {
		viewer.UserID = u.ID
		viewer.ModeratedCategories, _ = h.svc.deps.RBAC.ModeratedCategoryIDs(c.Request.Context(), u.ID)
		global, _ := h.svc.deps.RBAC.HasGlobalPermission(c.Request.Context(), u.ID, "post.moderate")
		viewer.IsStaff = global
	}
	detail, err := h.svc.Detail(c.Request.Context(), c.Param("id"), viewer)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"post": detail})
}

// RecordView POST /posts/:id/view
func (h *Handler) RecordView(c *gin.Context) {
	u := httpx.CurrentUser(c)
	key := "ip:" + c.ClientIP()
	if u != nil {
		key = "u:" + strconv.FormatInt(u.ID, 10)
	}
	h.svc.RecordView(c.Request.Context(), c.Param("id"), key)
	respond.OK(c, gin.H{"recorded": true})
}

// Create POST /posts
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	publicID, status, err := h.svc.Create(c.Request.Context(), u.ID, u.Username, req)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if status == StatusPendingReview {
		respond.Accepted(c, gin.H{"id": publicID, "status": status, "message": "内容已提交，等待审核"})
		return
	}
	respond.Created(c, gin.H{"id": publicID, "status": status})
}

// Update PATCH /posts/:id
func (h *Handler) Update(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := StaffViewer(h.svc.deps.RBAC, u.ID)
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if err := h.svc.Update(c.Request.Context(), viewer, u.ID, c.Param("id"), req, req.EditReason); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "帖子已更新"})
}

// Delete DELETE /posts/:id
func (h *Handler) Delete(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := StaffViewer(h.svc.deps.RBAC, u.ID)
	if err := h.svc.Delete(c.Request.Context(), viewer, u.ID, c.Param("id")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "帖子已删除"})
}

// Publish POST /posts/:id/publish
func (h *Handler) Publish(c *gin.Context) {
	u := httpx.CurrentUser(c)
	status, err := h.svc.Publish(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if status == StatusPendingReview {
		respond.Accepted(c, gin.H{"status": status, "message": "内容已提交，等待审核"})
		return
	}
	respond.OK(c, gin.H{"status": status, "message": "发布成功"})
}

// Revisions GET /posts/:id/revisions
func (h *Handler) Revisions(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	viewer := StaffViewer(h.svc.deps.RBAC, u.ID)
	list, err := h.svc.Revisions(c.Request.Context(), viewer, u.ID, c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

// MyPosts GET /me/posts
func (h *Handler) MyPosts(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.MyPosts(c.Request.Context(), u.ID, c.Query("status"), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// UserPosts GET /users/:username/posts
func (h *Handler) UserPosts(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	uid, err := h.svc.deps.Users.InternalIDByUsername(c.Request.Context(), c.Param("username"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	items, total, err := h.svc.ListByAuthor(c.Request.Context(), uid, viewerID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// AdminList GET /admin/posts
func (h *Handler) AdminList(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var allowedCats []int64
	global, err := h.svc.deps.RBAC.HasGlobalPermission(c.Request.Context(), u.ID, "post.moderate")
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if !global {
		ids, err := h.svc.deps.RBAC.ModeratedCategoryIDs(c.Request.Context(), u.ID)
		if err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
		if len(ids) == 0 {
			respond.Page(c, []PostItem{}, pq.Page, pq.PageSize, 0)
			return
		}
		allowedCats = ids
	}
	items, total, err := h.svc.AdminList(c.Request.Context(),
		c.Query("status"), c.Query("category"), c.Query("q"), allowedCats, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// BatchModerate POST /admin/posts/batch-moderate
func (h *Handler) BatchModerate(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := StaffViewer(h.svc.deps.RBAC, u.ID)
	var req BatchModerateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	results, err := h.svc.BatchModerate(c.Request.Context(), viewer, u.ID, c.GetString("request_id"), req)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"results": results})
}

// Moderate POST /admin/posts/:id/moderate
func (h *Handler) Moderate(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := StaffViewer(h.svc.deps.RBAC, u.ID)
	var req ModerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if err := h.svc.Moderate(c.Request.Context(), viewer, u.ID, c.GetString("request_id"), c.Param("id"), req); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "操作成功"})
}
