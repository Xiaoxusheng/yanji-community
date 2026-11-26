// Package tags 标签：公开列表/详情、关注计数、后台 CRUD。
package tags

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}$`)

// Tag 标签。
type Tag struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	PostCount   int64     `json:"post_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// Repo 标签数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ResolveOrCreateIDs 解析标签 slug 列表（仅允许已存在标签；自动创建关闭，防灌水）。
func (r *Repo) ResolveIDs(ctx context.Context, slugs []string) ([]int64, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(slugs))
	for _, s := range slugs {
		var id int64
		err := r.pool.QueryRow(ctx,
			`SELECT id FROM tags WHERE slug = $1 AND status = 'active'`, s).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.BadRequest("VALIDATION_ERROR", "标签不存在："+s)
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AttachTx 在事务内重建帖子的标签关联并维护计数（先减旧、再删、后加新）。
func (r *Repo) AttachTx(ctx context.Context, tx pgx.Tx, postID int64, tagIDs []int64) error {
	// 旧标签计数 -1
	if _, err := tx.Exec(ctx, `
		UPDATE tags t SET post_count = GREATEST(0, t.post_count - 1)
		WHERE t.id IN (SELECT tag_id FROM post_tags WHERE post_id = $1)`, postID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, postID); err != nil {
		return err
	}
	for _, id := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, postID, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE tags SET post_count = post_count + 1 WHERE id = $1`, id); err != nil {
			return err
		}
	}
	return nil
}

// ListPublic 公开标签列表（活跃，按使用量排序）。
func (r *Repo) ListPublic(ctx context.Context, pq pagination.Query) ([]Tag, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tags WHERE status = 'active'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, description, status, post_count, created_at
		FROM tags WHERE status = 'active'
		ORDER BY post_count DESC, id LIMIT $1 OFFSET $2`, pq.PageSize, pq.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, err := scanTags(rows)
	return list, total, err
}

// GetBySlug 详情。
func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Tag, error) {
	var t Tag
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, status, post_count, created_at
		FROM tags WHERE slug = $1 AND status = 'active'`, slug).
		Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.Status, &t.PostCount, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("标签不存在")
	}
	return &t, err
}

// TagsForPost 帖子的标签。
func (r *Repo) TagsForPost(ctx context.Context, postID int64) ([]Tag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.description, t.status, t.post_count, t.created_at
		FROM post_tags pt JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id = $1 ORDER BY t.id`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTags(rows)
}

// TagsForPosts 批量取帖子标签（避免 N+1）。
func (r *Repo) TagsForPosts(ctx context.Context, postIDs []int64) (map[int64][]Tag, error) {
	if len(postIDs) == 0 {
		return map[int64][]Tag{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT pt.post_id, t.id, t.name, t.slug, t.description, t.status, t.post_count, t.created_at
		FROM post_tags pt JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id = ANY($1) ORDER BY pt.post_id, t.id`, postIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Tag{}
	for rows.Next() {
		var postID int64
		var t Tag
		if err := rows.Scan(&postID, &t.ID, &t.Name, &t.Slug, &t.Description, &t.Status, &t.PostCount, &t.CreatedAt); err != nil {
			return nil, err
		}
		out[postID] = append(out[postID], t)
	}
	return out, rows.Err()
}

func scanTags(rows pgx.Rows) ([]Tag, error) {
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.Status, &t.PostCount, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---- 管理与创建 ----

// Create 创建标签。
func (r *Repo) Create(ctx context.Context, name, slug, description string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tags (name, slug, description) VALUES ($1, $2, $3) RETURNING id`,
		name, slug, description).Scan(&id)
	if err != nil && err.Error() == `ERROR: duplicate key value violates unique constraint "tags_slug_key" (SQLSTATE 23505)` {
		return 0, apperr.Conflict("CONFLICT", "标签 slug 已存在")
	}
	return id, err
}

// Update 更新标签。
func (r *Repo) Update(ctx context.Context, id int64, name, description, status *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tags SET
		  name = COALESCE($2, name),
		  description = COALESCE($3, description),
		  status = COALESCE($4, status)
		WHERE id = $1`, id, name, description, status)
	return err
}

// Delete 删除标签（含关联清理与计数清零）。
func (r *Repo) Delete(ctx context.Context, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM post_tags WHERE tag_id = $1`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("标签不存在")
	}
	return tx.Commit(ctx)
}

// ListAll 后台全量列表。
func (r *Repo) ListAll(ctx context.Context, pq pagination.Query) ([]Tag, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tags`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, description, status, post_count, created_at
		FROM tags ORDER BY id LIMIT $1 OFFSET $2`, pq.PageSize, pq.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, err := scanTags(rows)
	return list, total, err
}

// ---- Service / HTTP ----

// Service 标签业务。
type Service struct {
	repo   *Repo
	audit  *moderation.AuditLogger
	logger *slog.Logger
}

// SetAudit 注入审计日志（标签增删改写入 operation_logs）。
func (s *Service) SetAudit(a *moderation.AuditLogger) { s.audit = a }

// ResolveIDs 暴露给帖子模块。
func (s *Service) ResolveIDs(ctx context.Context, slugs []string) ([]int64, error) {
	return s.repo.ResolveIDs(ctx, slugs)
}

// AttachTx 暴露给帖子模块。
func (s *Service) AttachTx(ctx context.Context, tx pgx.Tx, postID int64, tagIDs []int64) error {
	return s.repo.AttachTx(ctx, tx, postID, tagIDs)
}

// GetBySlug 标签详情。
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Tag, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// TagsForPost 单帖标签。
func (s *Service) TagsForPost(ctx context.Context, postID int64) ([]Tag, error) {
	return s.repo.TagsForPost(ctx, postID)
}

// TagsForPosts 批量标签。
func (s *Service) TagsForPosts(ctx context.Context, postIDs []int64) (map[int64][]Tag, error) {
	return s.repo.TagsForPosts(ctx, postIDs)
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// Create 创建标签。
func (s *Service) Create(ctx context.Context, name, slug, description string) (int64, error) {
	if len([]rune(name)) < 1 || len([]rune(name)) > 30 {
		return 0, invalid("标签名需为 1-30 字")
	}
	if !slugRe.MatchString(slug) {
		return 0, invalid("slug 需为小写字母/数字/连字符")
	}
	return s.repo.Create(ctx, name, slug, description)
}

// Handler 标签 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic 公开路由。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/tags", h.List)
	rg.GET("/tags/:slug", h.Get)
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/tags", h.AdminList)
	rg.POST("/tags", h.Create)
	rg.PATCH("/tags/:id", h.Update)
	rg.DELETE("/tags/:id", h.Delete)
}

// List GET /tags
func (h *Handler) List(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.repo.ListPublic(c.Request.Context(), pq)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []Tag{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Get GET /tags/:slug
func (h *Handler) Get(c *gin.Context) {
	t, err := h.svc.repo.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"tag": t})
}

// AdminList GET /admin/tags
func (h *Handler) AdminList(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.repo.ListAll(c.Request.Context(), pq)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []Tag{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Create POST /admin/tags
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.ErrValidation)
		return
	}
	id, err := h.svc.Create(c.Request.Context(), req.Name, req.Slug, req.Description)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if h.svc.audit != nil && u != nil {
		h.svc.audit.Log(c.Request.Context(), moderation.Detail{
			Action: "tag.create", TargetType: "tag", TargetID: id,
			ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Slug,
		})
	}
	respond.Created(c, gin.H{"id": id})
}

// Update PATCH /admin/tags/:id
func (h *Handler) Update(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, errInvalidID)
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.ErrValidation)
		return
	}
	if req.Status != nil {
		switch *req.Status {
		case "active", "hidden", "disabled":
		default:
			respond.Fail(c, invalid("status 取值不合法"))
			return
		}
	}
	if err := h.svc.repo.Update(c.Request.Context(), id, req.Name, req.Description, req.Status); err != nil {
		respond.Fail(c, err)
		return
	}
	if h.svc.audit != nil && u != nil {
		h.svc.audit.Log(c.Request.Context(), moderation.Detail{
			Action: "tag.update", TargetType: "tag", TargetID: id,
			ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		})
	}
	respond.OK(c, gin.H{"message": "标签已更新"})
}

// Delete DELETE /admin/tags/:id
func (h *Handler) Delete(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, errInvalidID)
		return
	}
	if err := h.svc.repo.Delete(c.Request.Context(), id); err != nil {
		respond.Fail(c, err)
		return
	}
	if h.svc.audit != nil && u != nil {
		h.svc.audit.Log(c.Request.Context(), moderation.Detail{
			Action: "tag.delete", TargetType: "tag", TargetID: id,
			ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		})
	}
	respond.OK(c, gin.H{"message": "标签已删除"})
}

var errInvalidID = apperr.BadRequest("VALIDATION_ERROR", "ID 不合法")
