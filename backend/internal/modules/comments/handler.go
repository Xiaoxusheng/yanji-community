package comments

import (
	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/modules/posts"
	"github.com/yanji-community/backend/internal/shared/apperr"

	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 评论 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic 公开路由。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/posts/:id/comments", h.List)
}

// RegisterAuth 认证路由。
func (h *Handler) RegisterAuth(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireVerified())
	{
		g.POST("/posts/:id/comments", h.Create)
		g.PATCH("/comments/:id", h.Update)
		g.DELETE("/comments/:id", h.Delete)
	}
}

// List GET /posts/:id/comments
func (h *Handler) List(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	items, total, err := h.svc.List(c.Request.Context(), c.Param("id"), viewerID, c.DefaultQuery("sort", "latest"), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

type createCommentReq struct {
	Body     string `json:"body_markdown"`
	ParentID string `json:"parent_id"`
	QuoteID  string `json:"quote_comment_id"`
}

// Create POST /posts/:id/comments
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req createCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确"))
		return
	}
	publicID, status, err := h.svc.Create(c.Request.Context(), u.ID, c.Param("id"), req.Body, req.ParentID, req.QuoteID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if status == "pending_review" {
		respond.Accepted(c, gin.H{"id": publicID, "status": status, "message": "评论已提交，等待审核"})
		return
	}
	respond.Created(c, gin.H{"id": publicID, "status": status})
}

type updateCommentReq struct {
	Body string `json:"body_markdown"`
}

// Update PATCH /comments/:id
func (h *Handler) Update(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req updateCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确"))
		return
	}
	if err := h.svc.Update(c.Request.Context(), u.ID, c.Param("id"), req.Body); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "评论已更新"})
}

// Delete DELETE /comments/:id
func (h *Handler) Delete(c *gin.Context) {
	u := httpx.CurrentUser(c)
	viewer := posts.StaffViewer(h.svc.deps.RBAC, u.ID)
	if err := h.svc.Delete(c.Request.Context(), viewer, u.ID, c.GetString("request_id"), c.Param("id")); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "评论已删除"})
}
