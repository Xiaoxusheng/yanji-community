package categories

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Service 版块业务逻辑。
type Service struct {
	repo   *Repo
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// ListPublic 公开列表。
func (s *Service) ListPublic(ctx context.Context) ([]Category, error) {
	list, err := s.repo.ListPublic(ctx)
	if list == nil {
		list = []Category{}
	}
	return list, err
}

// ListAll 后台列表。
func (s *Service) ListAll(ctx context.Context) ([]Category, error) {
	list, err := s.repo.ListAll(ctx)
	if list == nil {
		list = []Category{}
	}
	return list, err
}

// GetBySlug 详情。
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Category, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// GetByID 内部 ID 查询（供帖子模块校验版块状态）。
func (s *Service) GetByID(ctx context.Context, id int64) (*Category, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateRequest 创建请求。
type CreateRequest struct {
	ParentID      int64  `json:"parent_id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sort_order"`
	Status        string `json:"status"`
	Visibility    string `json:"visibility"`
	PostingPolicy string `json:"posting_policy"`
	ReviewPolicy  string `json:"review_policy"`
}

// Create 创建版块。
func (s *Service) Create(ctx context.Context, req CreateRequest) (int64, error) {
	if len([]rune(req.Name)) < 1 || len([]rune(req.Name)) > 50 {
		return 0, invalid("版块名需为 1-50 字")
	}
	if !slugRe.MatchString(req.Slug) {
		return 0, invalid("slug 需为 2-50 位小写字母/数字/连字符")
	}
	cat := &Category{
		ParentID:      nilIfZero(req.ParentID),
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   req.Description,
		Icon:          req.Icon,
		SortOrder:     req.SortOrder,
		Status:        def(req.Status, "active"),
		Visibility:    def(req.Visibility, "public"),
		PostingPolicy: def(req.PostingPolicy, "all"),
		ReviewPolicy:  def(req.ReviewPolicy, "none"),
	}
	if cat.ParentID != nil {
		if _, err := s.repo.GetByID(ctx, *cat.ParentID); err != nil {
			return 0, err
		}
	}
	return s.repo.Create(ctx, cat)
}

// UpdateRequest 更新请求（指针字段可选）。
type UpdateRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Icon          *string `json:"icon"`
	SortOrder     *int    `json:"sort_order"`
	Status        *string `json:"status"`
	Visibility    *string `json:"visibility"`
	PostingPolicy *string `json:"posting_policy"`
	ReviewPolicy  *string `json:"review_policy"`
}

var enumPolicy = map[string][]string{
	"status":         {"active", "archived"},
	"visibility":     {"public", "private", "hidden"},
	"posting_policy": {"all", "verified", "staff_only"},
	"review_policy":  {"none", "new_users", "all"},
}

// Update 更新版块。
func (s *Service) Update(ctx context.Context, id int64, req UpdateRequest) error {
	fields := map[string]any{}
	apply := func(col string, val any) { fields[col] = val }
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if len([]rune(n)) < 1 || len([]rune(n)) > 50 {
			return invalid("版块名需为 1-50 字")
		}
		apply("name", n)
	}
	if req.Description != nil {
		apply("description", *req.Description)
	}
	if req.Icon != nil {
		apply("icon", *req.Icon)
	}
	if req.SortOrder != nil {
		apply("sort_order", *req.SortOrder)
	}
	for _, f := range []struct {
		col string
		val *string
	}{
		{"status", req.Status}, {"visibility", req.Visibility},
		{"posting_policy", req.PostingPolicy}, {"review_policy", req.ReviewPolicy},
	} {
		if f.val == nil {
			continue
		}
		ok := false
		for _, allowed := range enumPolicy[f.col] {
			if *f.val == allowed {
				ok = true
			}
		}
		if !ok {
			return invalid(f.col + " 取值不合法")
		}
		apply(f.col, *f.val)
	}
	return s.repo.Update(ctx, id, fields)
}

// Delete 软删除。
func (s *Service) Delete(ctx context.Context, id int64) error {
	cat, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if cat.PostCount > 0 {
		return apperr.Conflict("CONFLICT", "版块仍有帖子，请先迁移内容")
	}
	return s.repo.SoftDelete(ctx, id)
}

// Moderators / AddModerator / RemoveModerator。
func (s *Service) Moderators(ctx context.Context, categoryID int64) ([]Moderator, error) {
	list, err := s.repo.ListModerators(ctx, categoryID)
	if list == nil {
		list = []Moderator{}
	}
	return list, err
}

// ---- HTTP ----

// Handler 版块 HTTP 层。
type Handler struct {
	svc *Service
}

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic 公开路由。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/categories", h.List)
	rg.GET("/categories/:slug", h.Get)
}

// RegisterAdmin 后台路由（权限中间件由装配层包裹）。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/categories", h.AdminList)
	rg.POST("/categories", h.Create)
	rg.PATCH("/categories/:id", h.Update)
	rg.DELETE("/categories/:id", h.Delete)
	rg.GET("/categories/:id/moderators", h.Moderators)
	rg.POST("/categories/:id/moderators", h.AddModerator)
	rg.DELETE("/categories/:id/moderators/:userId", h.RemoveModerator)
}

// List GET /categories
func (h *Handler) List(c *gin.Context) {
	list, err := h.svc.ListPublic(c.Request.Context())
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

// Get GET /categories/:slug
func (h *Handler) Get(c *gin.Context) {
	cat, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	mods, _ := h.svc.Moderators(c.Request.Context(), cat.ID)
	respond.OK(c, gin.H{"category": cat, "moderators": mods})
}

// AdminList GET /admin/categories
func (h *Handler) AdminList(c *gin.Context) {
	list, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

// Create POST /admin/categories
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.ErrValidation)
		return
	}
	id, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"id": id})
}

// Update PATCH /admin/categories/:id
func (h *Handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.ErrValidation)
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "版块已更新"})
}

// Delete DELETE /admin/categories/:id
func (h *Handler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "版块已删除"})
}

// Moderators GET /admin/categories/:id/moderators
func (h *Handler) Moderators(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	list, err := h.svc.Moderators(c.Request.Context(), id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

// AddModerator POST /admin/categories/:id/moderators {username}
func (h *Handler) AddModerator(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		respond.Fail(c, invalid("请输入用户名"))
		return
	}
	if err := h.svc.AddModerator(c.Request.Context(), id, req.Username); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "版主已添加"})
}

// RemoveModerator DELETE /admin/categories/:id/moderators/:userId
func (h *Handler) RemoveModerator(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	userID, err := parseIDParam(c, "userId")
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.RemoveModerator(c.Request.Context(), id, userID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "版主已移除"})
}

// ---- helpers ----

func nilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func def(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func parseID(c *gin.Context) (int64, error) {
	return parseIDParam(c, "id")
}

func parseIDParam(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("VALIDATION_ERROR", "ID 不合法")
	}
	return id, nil
}

// AddModerator 添加版主（写审计由装配层负责）。
func (s *Service) AddModerator(ctx context.Context, categoryID int64, username string) error {
	if _, err := s.repo.GetByID(ctx, categoryID); err != nil {
		return err
	}
	userID, err := s.userResolver(ctx, username)
	if err != nil {
		return err
	}
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.AddModerator(ctx, tx, categoryID, userID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// RemoveModerator 移除版主。
func (s *Service) RemoveModerator(ctx context.Context, categoryID, userID int64) error {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.RemoveModerator(ctx, tx, categoryID, userID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// UserResolver 由路由装配注入（依赖 users 模块）。
var UserResolver func(ctx context.Context, username string) (int64, error)

func (s *Service) userResolver(ctx context.Context, username string) (int64, error) {
	if UserResolver != nil {
		return UserResolver(ctx, username)
	}
	return 0, apperr.Internal(errResolverMissing)
}

var errResolverMissing = apperr.Internal(nil)
